//go:build smoke && unix

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

const smokeCommentUpsertEnv = "KGH_GITHUB_COMMENT_SMOKE"

func TestSmokePRCommentUpsertLive(t *testing.T) {
	if os.Getenv(smokeCommentUpsertEnv) != "1" {
		t.Fatalf("set %s=1 to run the GitHub comment smoke test", smokeCommentUpsertEnv)
	}

	resolver := NewReportContextResolver()
	reportCtx, err := resolver.Resolve()
	if err != nil {
		t.Fatalf("resolve GitHub reporting context: %v", err)
	}
	if !reportCtx.HasPullRequest() {
		t.Fatal("GitHub comment smoke test requires a pull request context")
	}
	if reportCtx.RepositoryOwner == "" || reportCtx.RepositoryName == "" {
		t.Fatal("GitHub comment smoke test requires GITHUB_REPOSITORY=owner/repo")
	}

	writer := NewPRCommentWriter()
	bodyOne := smokeCommentBody("first pass")
	bodyTwo := smokeCommentBody("second pass")

	if err := writer.Write(context.Background(), reportCtx, bodyOne); err != nil {
		t.Fatalf("first comment write: %v", err)
	}
	if err := writer.Write(context.Background(), reportCtx, bodyTwo); err != nil {
		t.Fatalf("second comment write: %v", err)
	}

	comments, err := listPRComments(context.Background(), writer, reportCtx)
	if err != nil {
		t.Fatalf("list PR comments: %v", err)
	}

	markerBodies := make([]string, 0, 1)
	for _, comment := range comments {
		if strings.Contains(comment.Body, commentMarker) {
			markerBodies = append(markerBodies, comment.Body)
		}
	}

	if len(markerBodies) != 1 {
		t.Fatalf("expected exactly 1 marker comment, got %d", len(markerBodies))
	}
	if !strings.Contains(markerBodies[0], bodyTwo) {
		t.Fatalf("expected comment body to contain second write payload, got %q", markerBodies[0])
	}
	if strings.Contains(markerBodies[0], bodyOne) {
		t.Fatalf("expected existing comment to be updated in place, got stale first payload in %q", markerBodies[0])
	}
}

func smokeCommentBody(label string) string {
	return commentMarker + "\n" +
		"## kgh comment smoke\n" +
		"- label: " + label + "\n"
}

func listPRComments(ctx context.Context, writer PRCommentWriter, reportCtx ReportContext) ([]struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}, error) {
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}

	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=%d&page=%d", writer.apiURL(), reportCtx.RepositoryOwner, reportCtx.RepositoryName, reportCtx.PullRequestNumber, commentsPageSize, page), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(writer.getenv("GITHUB_TOKEN")))
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		var pageComments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := doJSONResponse(writer.client(), req, &pageComments); err != nil {
			return nil, err
		}
		comments = append(comments, pageComments...)
		if len(pageComments) < commentsPageSize {
			return comments, nil
		}
	}
}

func doJSONResponse(client httpDoer, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
