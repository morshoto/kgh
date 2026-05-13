package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/execution"
	ghctx "github.com/shotomorisk/kgh/internal/github"
	"github.com/shotomorisk/kgh/internal/spec"
)

type fakeExecutionRunner struct {
	result execution.Result
	err    error
}

type fakeGitHubReporter struct {
	calls int
	err   error
}

func (f fakeExecutionRunner) Execute(context.Context, execution.Request) (execution.Result, error) {
	return f.result, f.err
}

func (f *fakeGitHubReporter) WriteExecutionReport(context.Context, execution.Result, *execution.FailureSummary) error {
	f.calls++
	return f.err
}

func TestExecuteRequestWritesJSONAndGitHubReport(t *testing.T) {
	originalNewRunner := newRunner
	originalGitHubReporter := newGitHubReporter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubReporter = originalGitHubReporter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode: execution.ModeDryRun,
				Execution: spec.ExecutionSpec{
					TargetName: "exp142",
					Notebook:   "notebooks/exp142.ipynb",
					KernelID:   "yourname/exp142",
					KernelRef:  "yourname/exp142",
				},
			},
		}
	}
	reporter := &fakeGitHubReporter{}
	newGitHubReporter = func() githubExecutionReporter {
		return reporter
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{Target: "exp142", DryRun: true}, &stdout, &stderr, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"target_name": "exp142"`) {
		t.Fatalf("expected stdout JSON target, got %s", stdout.String())
	}
	if reporter.calls != 1 {
		t.Fatalf("expected 1 GitHub report write, got %d", reporter.calls)
	}
}

func TestExecuteRequestGitHubReportFailureIsFatalAfterJSON(t *testing.T) {
	originalNewRunner := newRunner
	originalGitHubReporter := newGitHubReporter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubReporter = originalGitHubReporter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode:      execution.ModeDryRun,
				Execution: spec.ExecutionSpec{TargetName: "exp142"},
			},
		}
	}
	newGitHubReporter = func() githubExecutionReporter {
		return &fakeGitHubReporter{err: errors.New("disk full")}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{Target: "exp142", DryRun: true}, &stdout, &stderr, true)
	if err == nil {
		t.Fatal("expected an error")
	}
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(err.Error(), "write GitHub report: disk full") {
		t.Fatalf("expected wrapped report error, got %q", err.Error())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected stdout JSON to still be written")
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected executeRequest not to write stderr directly, got %q", stderr.String())
	}
}

func TestExecuteRequestWritesGitHubSummaryOnExecutionFailure(t *testing.T) {
	originalNewRunner := newRunner
	originalGitHubReporter := newGitHubReporter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubReporter = originalGitHubReporter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode: execution.ModeLive,
				Execution: spec.ExecutionSpec{
					TargetName: "exp142",
					Notebook:   "notebooks/exp142.ipynb",
					KernelID:   "yourname/exp142",
					KernelRef:  "yourname/exp142",
				},
				Push: &execution.PushResult{
					KernelRef: "yourname/exp142",
				},
			},
			err: &execution.ErrorWithResult{
				Result: execution.Result{
					Mode: execution.ModeLive,
					Execution: spec.ExecutionSpec{
						TargetName: "exp142",
						Notebook:   "notebooks/exp142.ipynb",
						KernelID:   "yourname/exp142",
						KernelRef:  "yourname/exp142",
					},
					Push: &execution.PushResult{
						KernelRef: "yourname/exp142",
					},
				},
				Stage: execution.FailureStageSubmit,
				Err:   errors.New("submit failed"),
			},
		}
	}

	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)
	t.Setenv("GITHUB_EVENT_NAME", "workflow_dispatch")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{Target: "exp142", DryRun: false}, &stdout, &stderr, true)
	if err == nil {
		t.Fatal("expected an error")
	}
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(err.Error(), "submit failed") {
		t.Fatalf("expected wrapped execution error, got %q", err.Error())
	}
	if !strings.Contains(stdout.String(), `"target_name": "exp142"`) {
		t.Fatalf("expected stdout JSON target, got %s", stdout.String())
	}

	body, readErr := os.ReadFile(summaryPath)
	if readErr != nil {
		t.Fatalf("read summary file: %v", readErr)
	}
	if !strings.Contains(string(body), "### Failure") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Stage: `submit`") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Error: submit failed") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Target: `exp142`") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Kernel ID: `yourname/exp142`") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected executeRequest not to write stderr directly, got %q", stderr.String())
	}
}

func TestExecuteRequestWritesGitHubSummaryOnExecutionFailureWithoutPartialResult(t *testing.T) {
	originalNewRunner := newRunner
	originalGitHubReporter := newGitHubReporter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubReporter = originalGitHubReporter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			err: errors.New("bootstrap failed"),
		}
	}
	newGitHubReporter = func() githubExecutionReporter {
		return ghctx.NewRunReporter()
	}

	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)
	t.Setenv("GITHUB_EVENT_NAME", "workflow_dispatch")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{
		Target:     "exp142",
		DryRun:     false,
		ConfigPath: ".kgh/config.yaml",
	}, &stdout, &stderr, true)
	if err == nil {
		t.Fatal("expected an error")
	}
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(err.Error(), "bootstrap failed") {
		t.Fatalf("expected wrapped execution error, got %q", err.Error())
	}
	if !strings.Contains(stdout.String(), `"target_name": "exp142"`) {
		t.Fatalf("expected stdout JSON target, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"mode": "live"`) {
		t.Fatalf("expected stdout JSON mode, got %s", stdout.String())
	}

	body, readErr := os.ReadFile(summaryPath)
	if readErr != nil {
		t.Fatalf("read summary file: %v", readErr)
	}
	if !strings.Contains(string(body), "### Failure") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Stage: `execution`") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Error: bootstrap failed") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if !strings.Contains(string(body), "- Target: `exp142`") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
	if strings.Contains(string(body), "Kernel ID:") {
		t.Fatalf("expected no kernel id for fallback summary, got:\n%s", string(body))
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected executeRequest not to write stderr directly, got %q", stderr.String())
	}
}

func TestExecuteRequestSkipsSummaryWhenDisabled(t *testing.T) {
	originalNewRunner := newRunner
	originalGitHubReporter := newGitHubReporter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubReporter = originalGitHubReporter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode:      execution.ModeDryRun,
				Execution: spec.ExecutionSpec{TargetName: "exp142"},
			},
		}
	}
	newGitHubReporter = func() githubExecutionReporter {
		return &fakeGitHubReporter{err: errors.New("should not be called")}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{Target: "exp142", DryRun: true}, &stdout, &stderr, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestExecuteRequestIgnoresMissingGitHubStepSummaryEnv(t *testing.T) {
	originalNewRunner := newRunner
	originalGitHubReporter := newGitHubReporter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubReporter = originalGitHubReporter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode:      execution.ModeDryRun,
				Execution: spec.ExecutionSpec{TargetName: "exp142"},
			},
		}
	}

	reporter := &fakeGitHubReporter{}
	newGitHubReporter = func() githubExecutionReporter {
		return reporter
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{Target: "exp142", DryRun: true}, &stdout, &stderr, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"target_name": "exp142"`) {
		t.Fatalf("expected stdout JSON target, got %s", stdout.String())
	}
	if reporter.calls != 1 {
		t.Fatalf("expected 1 GitHub report write, got %d", reporter.calls)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
