package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReportContextResolverResolvePullRequest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"number":17}`), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}

	ctx, err := ReportContextResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "pull_request",
			"GITHUB_EVENT_PATH": eventPath,
			"GITHUB_REPOSITORY": "shotomorisk/kgh",
			"GITHUB_SERVER_URL": "https://github.example.com",
			"GITHUB_RUN_ID":     "42",
		}),
		ReadFile: os.ReadFile,
	}.Resolve()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ctx.PullRequestNumber != 17 {
		t.Fatalf("unexpected pull request number %d", ctx.PullRequestNumber)
	}
	if ctx.RepositoryOwner != "shotomorisk" || ctx.RepositoryName != "kgh" {
		t.Fatalf("unexpected repository split %+v", ctx)
	}
	if ctx.RunURL != "https://github.example.com/shotomorisk/kgh/actions/runs/42" {
		t.Fatalf("unexpected run url %q", ctx.RunURL)
	}
}

func TestReportContextResolverResolveNonPullRequest(t *testing.T) {
	t.Parallel()

	ctx, err := ReportContextResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "push",
			"GITHUB_REPOSITORY": "shotomorisk/kgh",
		}),
		ReadFile: os.ReadFile,
	}.Resolve()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ctx.HasPullRequest() {
		t.Fatalf("expected no pull request context, got %+v", ctx)
	}
}

func TestReportContextResolverResolveWorkflowDispatchOverride(t *testing.T) {
	t.Parallel()

	ctx, err := ReportContextResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME":       "workflow_dispatch",
			"GITHUB_REPOSITORY":       "shotomorisk/kgh",
			"KGH_PULL_REQUEST_NUMBER": "17",
			"GITHUB_SERVER_URL":       "https://github.example.com",
			"GITHUB_RUN_ID":           "42",
		}),
		ReadFile: os.ReadFile,
	}.Resolve()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ctx.PullRequestNumber != 17 {
		t.Fatalf("unexpected pull request number %d", ctx.PullRequestNumber)
	}
	if ctx.RunURL != "https://github.example.com/shotomorisk/kgh/actions/runs/42" {
		t.Fatalf("unexpected run url %q", ctx.RunURL)
	}
}

func TestReportContextResolverRejectsInvalidPullRequestOverride(t *testing.T) {
	t.Parallel()

	_, err := ReportContextResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME":       "workflow_dispatch",
			"KGH_PULL_REQUEST_NUMBER": "abc",
		}),
		ReadFile: os.ReadFile,
	}.Resolve()
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != "KGH_PULL_REQUEST_NUMBER must be a positive integer" {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestReportContextResolverRejectsMissingPullRequestNumber(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"number":0}`), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}

	_, err := ReportContextResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "pull_request",
			"GITHUB_EVENT_PATH": eventPath,
		}),
		ReadFile: os.ReadFile,
	}.Resolve()
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != "pull request number is required for pull_request events" {
		t.Fatalf("unexpected error %q", got)
	}
}
