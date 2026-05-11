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
	"github.com/shotomorisk/kgh/internal/spec"
)

type fakeExecutionRunner struct {
	result execution.Result
	err    error
}

type fakeSummaryWriter struct {
	err error
}

func (f fakeExecutionRunner) Execute(context.Context, execution.Request) (execution.Result, error) {
	return f.result, f.err
}

func (f fakeSummaryWriter) WriteExecutionSummary(execution.Result) error {
	return f.err
}

func TestExecuteRequestWritesJSONAndGitHubSummary(t *testing.T) {
	originalNewRunner := newRunner
	t.Cleanup(func() {
		newRunner = originalNewRunner
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
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)
	t.Setenv("GITHUB_EVENT_NAME", "push")
	t.Setenv("GITHUB_REPOSITORY", "shotomorisk/kgh")

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

	body, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary file: %v", err)
	}
	if !strings.Contains(string(body), "| Target | `exp142` |") {
		t.Fatalf("unexpected summary body:\n%s", string(body))
	}
}

func TestExecuteRequestSummaryWriteFailureIsFatalAfterJSON(t *testing.T) {
	originalNewRunner := newRunner
	originalSummaryWriter := newGitHubSummaryWriter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubSummaryWriter = originalSummaryWriter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode:      execution.ModeDryRun,
				Execution: spec.ExecutionSpec{TargetName: "exp142"},
			},
		}
	}
	newGitHubSummaryWriter = func() githubExecutionSummaryWriter {
		return fakeSummaryWriter{err: errors.New("disk full")}
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
	if !strings.Contains(err.Error(), "write GitHub summary: disk full") {
		t.Fatalf("expected wrapped summary error, got %q", err.Error())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected stdout JSON to still be written")
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected executeRequest not to write stderr directly, got %q", stderr.String())
	}
}

func TestExecuteRequestSkipsSummaryWhenDisabled(t *testing.T) {
	originalNewRunner := newRunner
	originalSummaryWriter := newGitHubSummaryWriter
	t.Cleanup(func() {
		newRunner = originalNewRunner
		newGitHubSummaryWriter = originalSummaryWriter
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode:      execution.ModeDryRun,
				Execution: spec.ExecutionSpec{TargetName: "exp142"},
			},
		}
	}
	newGitHubSummaryWriter = func() githubExecutionSummaryWriter {
		return fakeSummaryWriter{err: errors.New("should not be called")}
	}

	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := executeRequest(context.Background(), execution.Request{Target: "exp142", DryRun: true}, &stdout, &stderr, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if _, err := os.Stat(summaryPath); !os.IsNotExist(err) {
		t.Fatalf("expected no summary file, stat err=%v", err)
	}
}

func TestExecuteRequestIgnoresMissingGitHubStepSummaryEnv(t *testing.T) {
	originalNewRunner := newRunner
	t.Cleanup(func() {
		newRunner = originalNewRunner
	})

	newRunner = func(execution.Adapter) executionRunner {
		return fakeExecutionRunner{
			result: execution.Result{
				Mode:      execution.ModeDryRun,
				Execution: spec.ExecutionSpec{TargetName: "exp142"},
			},
		}
	}

	t.Setenv("GITHUB_STEP_SUMMARY", "")

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
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
