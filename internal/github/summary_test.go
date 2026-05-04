package github

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/spec"
)

func TestSummaryWriterNoopWithoutEnv(t *testing.T) {
	t.Parallel()

	called := false
	writer := SummaryWriter{
		Getenv: func(string) string { return "" },
		AppendFile: func(string, []byte) error {
			called = true
			return nil
		},
	}

	if err := writer.WriteExecutionSummary(execution.Result{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if called {
		t.Fatal("expected append not to be called")
	}
}

func TestSummaryWriterAppendsRenderedMarkdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(path, []byte("preflight\n"), 0o644); err != nil {
		t.Fatalf("write summary fixture: %v", err)
	}

	writer := SummaryWriter{
		Getenv: func(string) string { return path },
		AppendFile: func(path string, body []byte) error {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.Write(body)
			return err
		},
	}

	err := writer.WriteExecutionSummary(execution.Result{
		Mode:   execution.ModeDryRun,
		DryRun: true,
		Execution: spec.ExecutionSpec{
			TargetName: "exp142",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "preflight\n## kgh run summary") {
		t.Fatalf("expected appended summary, got:\n%s", got)
	}
	if !strings.Contains(got, "| Target | `exp142` |") {
		t.Fatalf("expected rendered target row, got:\n%s", got)
	}
}

func TestSummaryWriterReturnsAppendError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("append failed")
	writer := SummaryWriter{
		Getenv:     func(string) string { return "/tmp/summary" },
		AppendFile: func(string, []byte) error { return wantErr },
	}

	err := writer.WriteExecutionSummary(execution.Result{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected append error, got %v", err)
	}
}
