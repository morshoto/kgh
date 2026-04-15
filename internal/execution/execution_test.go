package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shotomorisk/kgh/internal/kaggle"
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
	if report.PollInterval != 5*time.Second {
		t.Fatalf("unexpected poll interval %s", report.PollInterval)
	}
	if report.PollTimeout != 30*time.Minute {
		t.Fatalf("unexpected poll timeout %s", report.PollTimeout)
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

func TestRunnerExecuteLive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notebook := filepath.Join(dir, "notebooks", "exp142.ipynb")
	if err := os.MkdirAll(filepath.Dir(notebook), 0o755); err != nil {
		t.Fatalf("mkdir notebook dir: %v", err)
	}
	if err := os.WriteFile(notebook, []byte(`{"cells":[]}`), 0o644); err != nil {
		t.Fatalf("write notebook: %v", err)
	}

	configPath := filepath.Join(dir, ".kgh", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`
targets:
  exp142:
    notebook: `+notebook+`
    kernel_id: yourname/exp142
    competition: playground-series-s6e2
    submit: true
    resources:
      gpu: true
      internet: false
      private: true
    outputs:
      submission: submission.csv
      metrics: metrics.json
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	adapter := &liveAdapter{
		t: t,
		pushFn: func(_ context.Context, req kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error) {
			if req.WorkDir == "" {
				t.Fatal("expected work dir to be set")
			}
			return kaggle.PushKernelResponse{
				KernelRef: "yourname/exp142",
				Output: kaggle.Result{
					Stdout:   "Kernel URL: https://www.kaggle.com/code/yourname/exp142\n",
					Stderr:   "",
					ExitCode: 0,
				},
			}, nil
		},
		pollFn: func(_ context.Context, req kaggle.KernelPollRequest) (kaggle.KernelPollResult, error) {
			if req.KernelRef != "yourname/exp142" {
				t.Fatalf("unexpected kernel ref %q", req.KernelRef)
			}
			if req.Interval != 2*time.Second {
				t.Fatalf("unexpected poll interval %s", req.Interval)
			}
			if req.Timeout != 15*time.Second {
				t.Fatalf("unexpected poll timeout %s", req.Timeout)
			}
			return kaggle.KernelPollResult{
				KernelStatusResponse: kaggle.KernelStatusResponse{
					KernelRef: "yourname/exp142",
					Status:    "complete",
					Message:   "finished",
					Raw: kaggle.KernelStatusRawStatus{
						Fields: map[string]string{"status": "complete"},
					},
				},
				Attempts:   2,
				StartedAt:  time.Unix(0, 0),
				FinishedAt: time.Unix(5, 0),
				Elapsed:    5 * time.Second,
				Terminal:   kaggle.KernelPollTerminalStateSucceeded,
			}, nil
		},
	}

	runner := NewRunner(adapter)
	runner.pollTimeout = 10 * time.Second
	runner.pollInterval = time.Second

	report, err := runner.Execute(context.Background(), Request{
		Target:       "exp142",
		DryRun:       false,
		ConfigPath:   configPath,
		PollInterval: 2 * time.Second,
		PollTimeout:  15 * time.Second,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if report.Mode != ModeLive {
		t.Fatalf("unexpected mode %q", report.Mode)
	}
	if report.DryRun {
		t.Fatalf("expected dry-run flag to be false")
	}
	if report.Bundle == nil || report.Push == nil || report.Poll == nil {
		t.Fatalf("expected live report sections to be populated: %+v", report)
	}
	if report.Push.KernelRef != "yourname/exp142" {
		t.Fatalf("unexpected pushed kernel ref %q", report.Push.KernelRef)
	}
	if report.Poll.Status != "complete" {
		t.Fatalf("unexpected poll status %q", report.Poll.Status)
	}
	if report.Poll.Terminal != kaggle.KernelPollTerminalStateSucceeded {
		t.Fatalf("unexpected terminal state %q", report.Poll.Terminal)
	}
	if report.PollInterval != 2*time.Second {
		t.Fatalf("unexpected effective poll interval %s", report.PollInterval)
	}
	if report.PollTimeout != 15*time.Second {
		t.Fatalf("unexpected effective poll timeout %s", report.PollTimeout)
	}
}

type liveAdapter struct {
	t      *testing.T
	pushFn func(context.Context, kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error)
	pollFn func(context.Context, kaggle.KernelPollRequest) (kaggle.KernelPollResult, error)
}

func (a *liveAdapter) PushKernel(ctx context.Context, req kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error) {
	if a.pushFn == nil {
		a.t.Fatal("pushFn must be set")
	}
	return a.pushFn(ctx, req)
}

func (a *liveAdapter) PollKernelStatus(ctx context.Context, req kaggle.KernelPollRequest) (kaggle.KernelPollResult, error) {
	if a.pollFn == nil {
		a.t.Fatal("pollFn must be set")
	}
	return a.pollFn(ctx, req)
}
