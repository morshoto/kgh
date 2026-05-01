package execution

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/kaggle"
	"github.com/shotomorisk/kgh/internal/spec"
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
	if time.Duration(report.PollInterval) != 5*time.Second {
		t.Fatalf("unexpected poll interval %s", report.PollInterval)
	}
	if time.Duration(report.PollTimeout) != 30*time.Minute {
		t.Fatalf("unexpected poll timeout %s", report.PollTimeout)
	}
	if report.Submission != nil {
		t.Fatalf("expected no submission details for dry-run, got %+v", report.Submission)
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
		downloadFn: func(_ context.Context, req kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error) {
			if err := os.WriteFile(filepath.Join(req.OutputDir, "submission.csv"), []byte("id,label\n1,0\n"), 0o644); err != nil {
				t.Fatalf("write submission output: %v", err)
			}
			if err := os.WriteFile(filepath.Join(req.OutputDir, "metrics.json"), []byte(`{"score":0.5}`), 0o644); err != nil {
				t.Fatalf("write metrics output: %v", err)
			}
			return kaggle.DownloadKernelOutputResponse{OutputDir: req.OutputDir}, nil
		},
		submitFn: func(_ context.Context, req kaggle.CompetitionSubmitRequest) (kaggle.CompetitionSubmitResponse, error) {
			if req.Competition != "playground-series-s6e2" {
				t.Fatalf("unexpected competition %q", req.Competition)
			}
			if filepath.Base(req.FilePath) != "submission.csv" {
				t.Fatalf("unexpected submission file %q", req.FilePath)
			}
			if req.Message != "kgh target=exp142 kernel=yourname/exp142" {
				t.Fatalf("unexpected submission message %q", req.Message)
			}
			return kaggle.CompetitionSubmitResponse{
				Competition: req.Competition,
				Submitted:   true,
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
	if report.Outputs == nil {
		t.Fatalf("expected live outputs to be populated: %+v", report)
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
	if time.Duration(report.PollInterval) != 2*time.Second {
		t.Fatalf("unexpected effective poll interval %s", report.PollInterval)
	}
	if time.Duration(report.PollTimeout) != 15*time.Second {
		t.Fatalf("unexpected effective poll timeout %s", report.PollTimeout)
	}
	if report.Outputs.OutputDir == "" {
		t.Fatal("expected output dir to be populated")
	}
	if !report.Outputs.Submission.Present {
		t.Fatalf("expected submission output to be present: %+v", report.Outputs.Submission)
	}
	if !report.Outputs.Metrics.Present {
		t.Fatalf("expected metrics output to be present: %+v", report.Outputs.Metrics)
	}
	if report.Outputs.SubmissionPath == "" || report.Outputs.MetricsPath == "" {
		t.Fatalf("expected canonical output paths to be populated: %+v", report.Outputs)
	}
	if !report.Outputs.Validation.Valid {
		t.Fatalf("expected validation to succeed: %+v", report.Outputs.Validation)
	}
	if len(report.Outputs.Validation.MissingRequired) != 0 || len(report.Outputs.Validation.MissingOptional) != 0 {
		t.Fatalf("expected no missing outputs, got %+v", report.Outputs.Validation)
	}
	if !report.Outputs.Submission.Required || report.Outputs.Submission.Error != "" {
		t.Fatalf("expected required submission with no error: %+v", report.Outputs.Submission)
	}
	if report.Outputs.Metrics.Required || report.Outputs.Metrics.Error != "" {
		t.Fatalf("expected optional metrics with no error: %+v", report.Outputs.Metrics)
	}
	if report.Submission == nil {
		t.Fatalf("expected submission details to be populated: %+v", report)
	}
	if !report.Submission.Enabled || report.Submission.Skipped {
		t.Fatalf("expected enabled non-skipped submission: %+v", report.Submission)
	}
	if !report.Submission.Submitted {
		t.Fatalf("expected submission to succeed: %+v", report.Submission)
	}
	if report.Submission.Competition != "playground-series-s6e2" {
		t.Fatalf("unexpected submission competition %q", report.Submission.Competition)
	}
	if report.Submission.FilePath != report.Outputs.SubmissionPath {
		t.Fatalf("expected submission file path to match outputs, got %+v vs %+v", report.Submission, report.Outputs)
	}
	if report.Submission.Message != "kgh target=exp142 kernel=yourname/exp142" {
		t.Fatalf("unexpected submission message %q", report.Submission.Message)
	}
}

func TestRunnerExecuteLiveMissingRequiredSubmissionFailsValidation(t *testing.T) {
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
      private: true
    outputs:
      submission: submission.csv
      metrics: metrics.json
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	adapter := &liveAdapter{
		t: t,
		pushFn: func(_ context.Context, _ kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error) {
			return kaggle.PushKernelResponse{KernelRef: "yourname/exp142"}, nil
		},
		pollFn: func(_ context.Context, _ kaggle.KernelPollRequest) (kaggle.KernelPollResult, error) {
			return kaggle.KernelPollResult{
				KernelStatusResponse: kaggle.KernelStatusResponse{
					KernelRef: "yourname/exp142",
					Status:    "complete",
				},
				Terminal: kaggle.KernelPollTerminalStateSucceeded,
			}, nil
		},
		downloadFn: func(_ context.Context, req kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error) {
			if err := os.WriteFile(filepath.Join(req.OutputDir, "metrics.json"), []byte(`{"score":0.5}`), 0o644); err != nil {
				t.Fatalf("write metrics output: %v", err)
			}
			return kaggle.DownloadKernelOutputResponse{OutputDir: req.OutputDir}, nil
		},
	}

	report, err := NewRunner(adapter).Execute(context.Background(), Request{
		Target:     "exp142",
		DryRun:     false,
		ConfigPath: configPath,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "submit enabled but submission artifact is missing") {
		t.Fatalf("unexpected error %q", got)
	}
	if report.Outputs == nil {
		t.Fatalf("expected outputs handoff, got %+v", report)
	}
	if report.Submission == nil {
		t.Fatalf("expected submission details on failure, got %+v", report)
	}
	if report.Outputs.Submission.Present {
		t.Fatalf("expected submission to be missing: %+v", report.Outputs.Submission)
	}
	if !report.Outputs.Metrics.Present {
		t.Fatalf("expected metrics to be present: %+v", report.Outputs.Metrics)
	}
	if report.Outputs.SubmissionPath != "" {
		t.Fatalf("expected empty submission path for missing output, got %q", report.Outputs.SubmissionPath)
	}
	if report.Outputs.MetricsPath == "" {
		t.Fatalf("expected metrics path to be set: %+v", report.Outputs)
	}
	if report.Outputs.Validation.Valid {
		t.Fatalf("expected validation to fail: %+v", report.Outputs.Validation)
	}
	if len(report.Outputs.Validation.MissingRequired) != 1 || report.Outputs.Validation.MissingRequired[0] != "submission" {
		t.Fatalf("expected submission in missing required list, got %+v", report.Outputs.Validation)
	}
	if len(report.Outputs.Validation.MissingOptional) != 0 {
		t.Fatalf("expected no missing optional outputs, got %+v", report.Outputs.Validation)
	}
	if report.Outputs.Submission.Error == "" || !strings.Contains(report.Outputs.Submission.Error, "submission output is missing") {
		t.Fatalf("unexpected submission error %+v", report.Outputs.Submission)
	}
	if !report.Submission.Enabled || report.Submission.Submitted {
		t.Fatalf("expected attempted but unsuccessful submission metadata: %+v", report.Submission)
	}
	if report.Submission.FilePath != "" {
		t.Fatalf("expected empty submission file path when output is missing, got %+v", report.Submission)
	}
	if report.Submission.Reason == "" || !strings.Contains(report.Submission.Reason, "submission output is missing") {
		t.Fatalf("unexpected submission reason %+v", report.Submission)
	}
}

func TestRunnerExecuteLiveMissingOptionalMetricsDoesNotFailValidation(t *testing.T) {
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
    submit: false
    outputs:
      submission: submission.csv
      metrics: metrics.json
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	adapter := &liveAdapter{
		t: t,
		pushFn: func(_ context.Context, _ kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error) {
			return kaggle.PushKernelResponse{KernelRef: "yourname/exp142"}, nil
		},
		pollFn: func(_ context.Context, _ kaggle.KernelPollRequest) (kaggle.KernelPollResult, error) {
			return kaggle.KernelPollResult{
				KernelStatusResponse: kaggle.KernelStatusResponse{
					KernelRef: "yourname/exp142",
					Status:    "complete",
				},
				Terminal: kaggle.KernelPollTerminalStateSucceeded,
			}, nil
		},
		downloadFn: func(_ context.Context, req kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error) {
			if err := os.WriteFile(filepath.Join(req.OutputDir, "submission.csv"), []byte("id,label\n1,0\n"), 0o644); err != nil {
				t.Fatalf("write submission output: %v", err)
			}
			return kaggle.DownloadKernelOutputResponse{OutputDir: req.OutputDir}, nil
		},
		submitFn: func(_ context.Context, _ kaggle.CompetitionSubmitRequest) (kaggle.CompetitionSubmitResponse, error) {
			t.Fatal("did not expect submission when submit=false")
			return kaggle.CompetitionSubmitResponse{}, nil
		},
	}

	report, err := NewRunner(adapter).Execute(context.Background(), Request{
		Target:     "exp142",
		DryRun:     false,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Outputs == nil {
		t.Fatalf("expected outputs handoff, got %+v", report)
	}
	if !report.Outputs.Validation.Valid {
		t.Fatalf("expected validation to succeed for optional metrics: %+v", report.Outputs.Validation)
	}
	if len(report.Outputs.Validation.MissingRequired) != 0 {
		t.Fatalf("expected no missing required outputs, got %+v", report.Outputs.Validation)
	}
	if len(report.Outputs.Validation.MissingOptional) != 1 || report.Outputs.Validation.MissingOptional[0] != "metrics" {
		t.Fatalf("expected metrics in missing optional list, got %+v", report.Outputs.Validation)
	}
	if report.Outputs.Metrics.Required {
		t.Fatalf("expected metrics to remain optional: %+v", report.Outputs.Metrics)
	}
	if report.Outputs.Metrics.Error == "" || !strings.Contains(report.Outputs.Metrics.Error, "metrics output is missing") {
		t.Fatalf("unexpected metrics error %+v", report.Outputs.Metrics)
	}
	if report.Submission == nil {
		t.Fatalf("expected submission details to explain skip: %+v", report)
	}
	if report.Submission.Enabled || !report.Submission.Skipped || report.Submission.Reason != "submission disabled for target" {
		t.Fatalf("unexpected submission skip details %+v", report.Submission)
	}
}

func TestRunnerExecuteLiveMissingSubmissionIsOptionalWhenSubmitDisabled(t *testing.T) {
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
    submit: false
    outputs:
      submission: submission.csv
      metrics: metrics.json
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	adapter := &liveAdapter{
		t: t,
		pushFn: func(_ context.Context, _ kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error) {
			return kaggle.PushKernelResponse{KernelRef: "yourname/exp142"}, nil
		},
		pollFn: func(_ context.Context, _ kaggle.KernelPollRequest) (kaggle.KernelPollResult, error) {
			return kaggle.KernelPollResult{
				KernelStatusResponse: kaggle.KernelStatusResponse{
					KernelRef: "yourname/exp142",
					Status:    "complete",
				},
				Terminal: kaggle.KernelPollTerminalStateSucceeded,
			}, nil
		},
		downloadFn: func(_ context.Context, req kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error) {
			if err := os.WriteFile(filepath.Join(req.OutputDir, "metrics.json"), []byte(`{"score":0.5}`), 0o644); err != nil {
				t.Fatalf("write metrics output: %v", err)
			}
			return kaggle.DownloadKernelOutputResponse{OutputDir: req.OutputDir}, nil
		},
		submitFn: func(_ context.Context, _ kaggle.CompetitionSubmitRequest) (kaggle.CompetitionSubmitResponse, error) {
			t.Fatal("did not expect submission when submit=false")
			return kaggle.CompetitionSubmitResponse{}, nil
		},
	}

	report, err := NewRunner(adapter).Execute(context.Background(), Request{
		Target:     "exp142",
		DryRun:     false,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Outputs == nil {
		t.Fatalf("expected outputs handoff, got %+v", report)
	}
	if !report.Outputs.Validation.Valid {
		t.Fatalf("expected validation to succeed for optional submission: %+v", report.Outputs.Validation)
	}
	if report.Outputs.Submission.Required {
		t.Fatalf("expected submission to be optional when submit=false: %+v", report.Outputs.Submission)
	}
	if len(report.Outputs.Validation.MissingOptional) != 1 || report.Outputs.Validation.MissingOptional[0] != "submission" {
		t.Fatalf("expected submission in missing optional list, got %+v", report.Outputs.Validation)
	}
	if report.Outputs.Submission.Error == "" || !strings.Contains(report.Outputs.Submission.Error, "submission output is missing") {
		t.Fatalf("unexpected submission error %+v", report.Outputs.Submission)
	}
	if report.Submission == nil {
		t.Fatalf("expected submission details to explain skip: %+v", report)
	}
	if report.Submission.Enabled || !report.Submission.Skipped || report.Submission.Reason != "submission disabled for target" {
		t.Fatalf("unexpected submission skip details %+v", report.Submission)
	}
}

func TestBuildOutputsResultDeterministicJSON(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "submission.csv"), []byte("id,label\n1,0\n"), 0o644); err != nil {
		t.Fatalf("write submission output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "metrics.json"), []byte(`{"score":0.5}`), 0o644); err != nil {
		t.Fatalf("write metrics output: %v", err)
	}

	execSpec := testExecutionSpec()
	first, err := buildOutputsResult(execSpec, outputDir)
	if err != nil {
		t.Fatalf("build outputs result: %v", err)
	}
	second, err := buildOutputsResult(execSpec, outputDir)
	if err != nil {
		t.Fatalf("build outputs result: %v", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first outputs result: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second outputs result: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("expected deterministic outputs json, got %q and %q", string(firstJSON), string(secondJSON))
	}
}

func TestBuildOutputsResultRejectsEscapingPaths(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	execSpec := testExecutionSpec()
	execSpec.Outputs.Submission = "../submission.csv"

	result, err := buildOutputsResult(execSpec, outputDir)
	if err != nil {
		t.Fatalf("build outputs result: %v", err)
	}
	if result.Submission.Present {
		t.Fatalf("expected escaping submission path to be rejected: %+v", result.Submission)
	}
	if result.Validation.Valid {
		t.Fatalf("expected validation to fail, got %+v", result.Validation)
	}
	if len(result.Validation.MissingRequired) != 1 || result.Validation.MissingRequired[0] != "submission" {
		t.Fatalf("expected submission in missing required list, got %+v", result.Validation)
	}
	if got := result.Submission.Error; !strings.Contains(got, "resolves outside output dir") {
		t.Fatalf("unexpected submission error %q", got)
	}
}

type liveAdapter struct {
	t          *testing.T
	pushFn     func(context.Context, kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error)
	pollFn     func(context.Context, kaggle.KernelPollRequest) (kaggle.KernelPollResult, error)
	downloadFn func(context.Context, kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error)
	submitFn   func(context.Context, kaggle.CompetitionSubmitRequest) (kaggle.CompetitionSubmitResponse, error)
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

func (a *liveAdapter) DownloadKernelOutput(ctx context.Context, req kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error) {
	if a.downloadFn == nil {
		a.t.Fatal("downloadFn must be set")
	}
	return a.downloadFn(ctx, req)
}

func (a *liveAdapter) SubmitCompetition(ctx context.Context, req kaggle.CompetitionSubmitRequest) (kaggle.CompetitionSubmitResponse, error) {
	if a.submitFn == nil {
		a.t.Fatal("submitFn must be set")
	}
	return a.submitFn(ctx, req)
}

func testExecutionSpec() spec.ExecutionSpec {
	return spec.ExecutionSpec{
		Submit: true,
		Outputs: config.Outputs{
			Submission: "submission.csv",
			Metrics:    "metrics.json",
		},
	}
}
