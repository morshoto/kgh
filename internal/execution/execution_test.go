package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerExecuteDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, ".kgh")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
targets:
  exp142:
    notebook: notebooks/exp142.ipynb
    kernel_id: yourname/exp142
    competition: playground-series-s6e2
    submit: true
    resources:
      gpu: true
      internet: false
      private: true
    sources:
      competition_sources:
        - playground-series-s6e2
      dataset_sources:
        - yourname/feature-pack-v3
    outputs:
      submission: submission.csv
      metrics: metrics.json
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	gpu := false
	internet := true

	runner := NewRunner(nil)
	report, err := runner.Execute(context.Background(), Request{
		Target:     "exp142",
		DryRun:     true,
		GPU:        &gpu,
		Internet:   &internet,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if report.Mode != ModeDryRun {
		t.Fatalf("unexpected mode %q", report.Mode)
	}
	if !report.DryRun {
		t.Fatalf("expected dry-run flag to be true")
	}
	if report.Execution.TargetName != "exp142" {
		t.Fatalf("unexpected target name %q", report.Execution.TargetName)
	}
	if report.Execution.Notebook != "notebooks/exp142.ipynb" {
		t.Fatalf("unexpected notebook %q", report.Execution.Notebook)
	}
	if report.Execution.KernelID != "yourname/exp142" {
		t.Fatalf("unexpected kernel id %q", report.Execution.KernelID)
	}
	if report.Execution.Resources.GPU {
		t.Fatalf("expected gpu override to be false")
	}
	if !report.Execution.Resources.Internet {
		t.Fatalf("expected internet override to be true")
	}
	if report.Execution.Outputs.Submission != "submission.csv" {
		t.Fatalf("unexpected submission output %q", report.Execution.Outputs.Submission)
	}
	if report.Execution.Outputs.Metrics != "metrics.json" {
		t.Fatalf("unexpected metrics output %q", report.Execution.Outputs.Metrics)
	}
}

func TestRunnerExecuteMissingTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
targets:
  exp142:
    notebook: notebooks/exp142.ipynb
    kernel_id: yourname/exp142
    competition: playground-series-s6e2
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := NewRunner(nil).Execute(context.Background(), Request{
		Target:     "missing",
		DryRun:     true,
		ConfigPath: configPath,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, `unknown target "missing"`) {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestRunnerExecuteConfigLoadFailure(t *testing.T) {
	t.Parallel()

	_, err := NewRunner(nil).Execute(context.Background(), Request{
		Target:     "exp142",
		DryRun:     true,
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "load config") {
		t.Fatalf("unexpected error %q", got)
	}
}
