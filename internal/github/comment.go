package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const commentMarker = "<!-- kgh:run-report -->"

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// PRCommentWriter upserts the kgh report comment onto a pull request.
type PRCommentWriter struct {
	Getenv     func(string) string
	HTTPClient httpDoer
}

func NewPRCommentWriter() PRCommentWriter {
	return PRCommentWriter{
		Getenv:     os.Getenv,
		HTTPClient: http.DefaultClient,
	}
}

func (w PRCommentWriter) Write(ctx context.Context, reportCtx ReportContext, body string) error {
	if !reportCtx.HasPullRequest() {
		return nil
	}

	token := strings.TrimSpace(w.getenv("GITHUB_TOKEN"))
	if token == "" {
		return fmt.Errorf("upsert GitHub PR comment: GITHUB_TOKEN is required")
	}
	if reportCtx.RepositoryOwner == "" || reportCtx.RepositoryName == "" {
		return fmt.Errorf("upsert GitHub PR comment: GITHUB_REPOSITORY must be owner/repo")
	}

	commentID, err := w.findExistingCommentID(ctx, reportCtx, token)
	if err != nil {
		return fmt.Errorf("upsert GitHub PR comment: %w", err)
	}
	if commentID == 0 {
		if err := w.createComment(ctx, reportCtx, token, body); err != nil {
			return fmt.Errorf("upsert GitHub PR comment: %w", err)
		}
		return nil
	}
	if err := w.updateComment(ctx, reportCtx, token, commentID, body); err != nil {
		return fmt.Errorf("upsert GitHub PR comment: %w", err)
	}
	return nil
}

func (w PRCommentWriter) findExistingCommentID(ctx context.Context, reportCtx ReportContext, token string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.apiURL()+"/repos/"+reportCtx.RepositoryOwner+"/"+reportCtx.RepositoryName+"/issues/"+fmt.Sprintf("%d", reportCtx.PullRequestNumber)+"/comments", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := w.doJSON(req, &comments); err != nil {
		return 0, err
	}
	for _, comment := range comments {
		if strings.Contains(comment.Body, commentMarker) {
			return comment.ID, nil
		}
	}
	return 0, nil
}

func (w PRCommentWriter) createComment(ctx context.Context, reportCtx ReportContext, token, body string) error {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.apiURL()+"/repos/"+reportCtx.RepositoryOwner+"/"+reportCtx.RepositoryName+"/issues/"+fmt.Sprintf("%d", reportCtx.PullRequestNumber)+"/comments", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return w.doNoContent(req)
}

func (w PRCommentWriter) updateComment(ctx context.Context, reportCtx ReportContext, token string, commentID int64, body string) error {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, w.apiURL()+"/repos/"+reportCtx.RepositoryOwner+"/"+reportCtx.RepositoryName+"/issues/comments/"+fmt.Sprintf("%d", commentID), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return w.doNoContent(req)
}

func (w PRCommentWriter) doJSON(req *http.Request, out any) error {
	resp, err := w.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (w PRCommentWriter) doNoContent(req *http.Request) error {
	resp, err := w.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusError(resp)
	}
	return nil
}

func (w PRCommentWriter) client() httpDoer {
	if w.HTTPClient != nil {
		return w.HTTPClient
	}
	return http.DefaultClient
}

func (w PRCommentWriter) getenv(key string) string {
	if w.Getenv != nil {
		return w.Getenv(key)
	}
	return os.Getenv(key)
}

func (w PRCommentWriter) apiURL() string {
	base := strings.TrimSpace(w.getenv("GITHUB_API_URL"))
	if base == "" {
		base = defaultGitHubAPIURL
	}
	return strings.TrimRight(base, "/")
}

func httpStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	return fmt.Errorf("GitHub API returned %s: %s", resp.Status, text)
}
