package github

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultGitHubAPIURL = "https://api.github.com"

// ReportContext contains the GitHub metadata needed for result publication.
type ReportContext struct {
	EventName         string
	Repository        string
	RepositoryOwner   string
	RepositoryName    string
	PullRequestNumber int
	RunURL            string
}

func (c ReportContext) HasPullRequest() bool {
	return c.PullRequestNumber > 0
}

// ReportContextResolver resolves GitHub reporting metadata from environment and event payload.
type ReportContextResolver struct {
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
}

func NewReportContextResolver() ReportContextResolver {
	return ReportContextResolver{
		Getenv:   os.Getenv,
		ReadFile: os.ReadFile,
	}
}

func (r ReportContextResolver) Resolve() (ReportContext, error) {
	if r.Getenv == nil {
		r.Getenv = os.Getenv
	}
	if r.ReadFile == nil {
		r.ReadFile = os.ReadFile
	}

	ctx := ReportContext{
		EventName:  strings.TrimSpace(r.Getenv("GITHUB_EVENT_NAME")),
		Repository: strings.TrimSpace(r.Getenv("GITHUB_REPOSITORY")),
	}
	ctx.RepositoryOwner, ctx.RepositoryName = splitRepository(ctx.Repository)
	ctx.RunURL = buildRunURL(
		r.Getenv("GITHUB_SERVER_URL"),
		ctx.Repository,
		r.Getenv("GITHUB_RUN_ID"),
	)

	switch ctx.EventName {
	case "pull_request", "pull_request_target":
		number, err := r.resolvePullRequestNumber()
		if err != nil {
			return ReportContext{}, err
		}
		ctx.PullRequestNumber = number
	case "":
		return ReportContext{}, fmt.Errorf("GITHUB_EVENT_NAME is required")
	}

	return ctx, nil
}

func (r ReportContextResolver) resolvePullRequestNumber() (int, error) {
	eventPath := strings.TrimSpace(r.Getenv("GITHUB_EVENT_PATH"))
	if eventPath == "" {
		return 0, fmt.Errorf("GITHUB_EVENT_PATH is required for %s events", strings.TrimSpace(r.Getenv("GITHUB_EVENT_NAME")))
	}

	body, err := r.ReadFile(eventPath)
	if err != nil {
		return 0, fmt.Errorf("read GitHub event payload %q: %w", eventPath, err)
	}

	var payload struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("parse GitHub event payload %q: %w", eventPath, err)
	}
	if payload.Number <= 0 {
		return 0, fmt.Errorf("pull request number is required for %s events", strings.TrimSpace(r.Getenv("GITHUB_EVENT_NAME")))
	}
	return payload.Number, nil
}

func splitRepository(repository string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(repository), "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func buildRunURL(serverURL, repository, runID string) string {
	serverURL = strings.TrimSpace(serverURL)
	repository = strings.TrimSpace(repository)
	runID = strings.TrimSpace(runID)
	if serverURL == "" {
		serverURL = "https://github.com"
	}
	if repository == "" || runID == "" {
		return ""
	}
	if _, err := strconv.ParseInt(runID, 10, 64); err != nil {
		return ""
	}
	return strings.TrimRight(serverURL, "/") + "/" + repository + "/actions/runs/" + runID
}
