package github

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/execution"
)

func TestBaselineResolverResolvesLatestSuccessfulBaseBranchRun(t *testing.T) {
	client := &baselineHTTPClient{
		t: t,
		handlers: map[string]func(*http.Request) (*http.Response, error){
			"GET /repos/shotomorisk/kgh/actions/runs": func(r *http.Request) (*http.Response, error) {
				if got := r.URL.Query().Get("branch"); got != "main" {
					t.Fatalf("unexpected branch filter %q", got)
				}
				if got := r.URL.Query().Get("status"); got != "completed" {
					t.Fatalf("unexpected status filter %q", got)
				}
				if got := r.URL.Query().Get("page"); got != "1" {
					t.Fatalf("unexpected page %q", got)
				}
				if got := r.URL.Query().Get("per_page"); got != "100" {
					t.Fatalf("unexpected per_page %q", got)
				}
				return baselineJSONResponse(`{"workflow_runs":[
					{"id":3,"path":".github/workflows/commit-trigger.yml@refs/heads/main","conclusion":"failure"},
					{"id":2,"path":".github/workflows/other.yml@refs/heads/main","conclusion":"success"},
					{"id":1,"path":".github/workflows/commit-trigger.yml@refs/heads/main","conclusion":"success"}
				]}`), nil
			},
			"GET /repos/shotomorisk/kgh/actions/runs/1/logs": func(*http.Request) (*http.Response, error) {
				return baselineZipResponse(logArchive(t, `workflow step output
{
  "mode": "live",
  "dry_run": false,
  "config_path": ".kgh/config.yaml",
  "execution": {
    "target_name": "exp142"
  },
  "score": {
    "public_score": "0.12345"
  }
}
`)), nil
			},
		},
	}

	t.Setenv("GITHUB_API_URL", "https://api.github.test")
	t.Setenv("GITHUB_TOKEN", "token")

	resolver := BaselineResolver{
		Getenv:     os.Getenv,
		HTTPClient: client,
	}

	comparison, err := resolver.Resolve(context.Background(), ReportContext{
		RepositoryOwner: "shotomorisk",
		RepositoryName:  "kgh",
		BaseBranch:      "main",
		WorkflowPath:    ".github/workflows/commit-trigger.yml",
	}, execution.Result{
		Score: &execution.ScoreResult{
			PublicScore: "0.23456",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if comparison == nil {
		t.Fatal("expected a comparison")
	}
	if comparison.Label != "main" {
		t.Fatalf("unexpected baseline label %q", comparison.Label)
	}
	if comparison.BaselineScore != "0.12345" {
		t.Fatalf("unexpected baseline score %q", comparison.BaselineScore)
	}
	if comparison.CurrentScore != "0.23456" {
		t.Fatalf("unexpected current score %q", comparison.CurrentScore)
	}
	if comparison.Delta != "+0.11111" {
		t.Fatalf("unexpected delta %q", comparison.Delta)
	}
}

func TestBaselineResolverReturnsUnavailableWhenNoSuccessfulRunExists(t *testing.T) {
	client := &baselineHTTPClient{
		t: t,
		handlers: map[string]func(*http.Request) (*http.Response, error){
			"GET /repos/shotomorisk/kgh/actions/runs": func(*http.Request) (*http.Response, error) {
				return baselineJSONResponse(`{"workflow_runs":[
					{"id":3,"path":".github/workflows/commit-trigger.yml@refs/heads/main","conclusion":"failure"},
					{"id":2,"path":".github/workflows/commit-trigger.yml@refs/heads/main","conclusion":"cancelled"}
				]}`), nil
			},
		},
	}

	t.Setenv("GITHUB_API_URL", "https://api.github.test")
	t.Setenv("GITHUB_TOKEN", "token")

	resolver := BaselineResolver{
		Getenv:     os.Getenv,
		HTTPClient: client,
	}

	comparison, err := resolver.Resolve(context.Background(), ReportContext{
		RepositoryOwner: "shotomorisk",
		RepositoryName:  "kgh",
		BaseBranch:      "main",
		WorkflowPath:    ".github/workflows/commit-trigger.yml",
	}, execution.Result{
		Score: &execution.ScoreResult{
			PublicScore: "0.23456",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if comparison != nil {
		t.Fatalf("expected unavailable comparison, got %+v", comparison)
	}
}

func TestBaselineResolverReturnsUnavailableForMalformedPayload(t *testing.T) {
	client := &baselineHTTPClient{
		t: t,
		handlers: map[string]func(*http.Request) (*http.Response, error){
			"GET /repos/shotomorisk/kgh/actions/runs": func(*http.Request) (*http.Response, error) {
				return baselineJSONResponse(`{"workflow_runs":[
					{"id":1,"path":".github/workflows/commit-trigger.yml@refs/heads/main","conclusion":"success"}
				]}`), nil
			},
			"GET /repos/shotomorisk/kgh/actions/runs/1/logs": func(*http.Request) (*http.Response, error) {
				return baselineZipResponse(logArchive(t, "workflow output without a JSON payload")), nil
			},
		},
	}

	t.Setenv("GITHUB_API_URL", "https://api.github.test")
	t.Setenv("GITHUB_TOKEN", "token")

	resolver := BaselineResolver{
		Getenv:     os.Getenv,
		HTTPClient: client,
	}

	comparison, err := resolver.Resolve(context.Background(), ReportContext{
		RepositoryOwner: "shotomorisk",
		RepositoryName:  "kgh",
		BaseBranch:      "main",
		WorkflowPath:    ".github/workflows/commit-trigger.yml",
	}, execution.Result{
		Score: &execution.ScoreResult{
			PublicScore: "0.23456",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if comparison != nil {
		t.Fatalf("expected unavailable comparison, got %+v", comparison)
	}
}

func logArchive(t *testing.T, content string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	file, err := zw.Create("job.log")
	if err != nil {
		t.Fatalf("create log file: %v", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatalf("write log content: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func baselineJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func baselineZipResponse(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

type baselineHTTPClient struct {
	t        *testing.T
	handlers map[string]func(*http.Request) (*http.Response, error)
}

func (c *baselineHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.t.Helper()
	key := req.Method + " " + req.URL.Path
	handler, ok := c.handlers[key]
	if !ok {
		c.t.Fatalf("unexpected request %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
	}
	return handler(req)
}
