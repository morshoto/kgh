package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/kaggle"
	"github.com/shotomorisk/kgh/internal/parser"
	"github.com/shotomorisk/kgh/internal/planner"
	"github.com/shotomorisk/kgh/internal/spec"
)

const DefaultConfigPath = config.DefaultPath

const (
	ModeDryRun = "dry-run"
	ModeLive   = "live"
)

type Request struct {
	Target       string
	DryRun       bool
	GPU          *bool
	Internet     *bool
	ConfigPath   string
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type Result struct {
	Mode         string             `json:"mode"`
	DryRun       bool               `json:"dry_run"`
	ConfigPath   string             `json:"config_path"`
	PollInterval Duration           `json:"poll_interval"`
	PollTimeout  Duration           `json:"poll_timeout"`
	Execution    spec.ExecutionSpec `json:"execution"`
	Bundle       *BundleResult      `json:"bundle,omitempty"`
	Push         *PushResult        `json:"push,omitempty"`
	Poll         *PollResult        `json:"poll,omitempty"`
	Outputs      *OutputsResult     `json:"outputs,omitempty"`
}

type BundleResult struct {
	WorkDir      string `json:"work_dir"`
	NotebookPath string `json:"notebook_path"`
	MetadataPath string `json:"metadata_path"`
}

type PushResult struct {
	KernelRef string `json:"kernel_ref"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
}

type PollResult struct {
	KernelRef  string                         `json:"kernel_ref"`
	Status     string                         `json:"status"`
	Message    string                         `json:"message,omitempty"`
	Attempts   int                            `json:"attempts"`
	Terminal   kaggle.KernelPollTerminalState `json:"terminal,omitempty"`
	StartedAt  time.Time                      `json:"started_at"`
	FinishedAt time.Time                      `json:"finished_at"`
	Elapsed    Duration                       `json:"elapsed"`
	Raw        kaggle.KernelStatusRawStatus   `json:"raw,omitempty"`
}

type OutputsResult struct {
	OutputDir      string                  `json:"output_dir"`
	SubmissionPath string                  `json:"submission_path"`
	MetricsPath    string                  `json:"metrics_path"`
	Submission     OutputFileResult        `json:"submission"`
	Metrics        OutputFileResult        `json:"metrics"`
	Validation     OutputValidationResult  `json:"validation"`
}

type OutputValidationResult struct {
	Valid           bool     `json:"valid"`
	MissingRequired []string `json:"missing_required"`
	MissingOptional []string `json:"missing_optional"`
}

type OutputFileResult struct {
	ConfiguredPath string `json:"configured_path"`
	ExpectedPath   string `json:"expected_path"`
	Path           string `json:"path"`
	Present        bool   `json:"present"`
	Required       bool   `json:"required"`
	Error          string `json:"error"`
}

type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

type Adapter interface {
	PushKernel(context.Context, kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error)
	PollKernelStatus(context.Context, kaggle.KernelPollRequest) (kaggle.KernelPollResult, error)
	DownloadKernelOutput(context.Context, kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error)
}

type Runner struct {
	loadConfig   func(string) (config.Config, error)
	adapter      Adapter
	pollTimeout  time.Duration
	pollInterval time.Duration
}

func NewRunner(adapter Adapter) *Runner {
	if adapter == nil {
		adapter = kaggle.NewAdapter(kaggle.NewClient()).(Adapter)
	}
	return &Runner{
		loadConfig:   config.Load,
		adapter:      adapter,
		pollTimeout:  30 * time.Minute,
		pollInterval: 5 * time.Second,
	}
}

func (r *Runner) Execute(ctx context.Context, req Request) (Result, error) {
	if r == nil {
		return Result{}, fmt.Errorf("execution runner is nil")
	}
	if r.loadConfig == nil {
		r.loadConfig = config.Load
	}
	if req.ConfigPath == "" {
		req.ConfigPath = DefaultConfigPath
	}

	cfg, err := r.loadConfig(req.ConfigPath)
	if err != nil {
		return Result{}, fmt.Errorf("load config %q: %w", req.ConfigPath, err)
	}

	trigger := parser.Trigger{
		Target:   req.Target,
		GPU:      req.GPU,
		Internet: req.Internet,
	}

	execSpec, err := planner.Resolve(cfg, trigger)
	if err != nil {
		return Result{}, err
	}

	report := Result{
		Mode:         ModeDryRun,
		DryRun:       true,
		ConfigPath:   req.ConfigPath,
		PollInterval: Duration(effectivePollInterval(req.PollInterval, r.pollInterval)),
		PollTimeout:  Duration(effectivePollTimeout(req.PollTimeout, r.pollTimeout)),
		Execution:    execSpec,
	}
	if !req.DryRun {
		return r.executeLive(ctx, execSpec, report)
	}

	return report, nil
}

func (r *Runner) executeLive(ctx context.Context, execSpec spec.ExecutionSpec, report Result) (Result, error) {
	if r.adapter == nil {
		return Result{}, fmt.Errorf("live execution requires a Kaggle adapter")
	}

	bundle, err := kaggle.StageKernelBundle(execSpec)
	if err != nil {
		return Result{}, fmt.Errorf("stage kaggle bundle: %w", err)
	}
	defer func() {
		if bundle.Cleanup != nil {
			_ = bundle.Cleanup()
		}
	}()

	report.Mode = ModeLive
	report.DryRun = false
	report.Bundle = &BundleResult{
		WorkDir:      bundle.WorkDir,
		NotebookPath: bundle.NotebookPath,
		MetadataPath: bundle.MetadataPath,
	}

	pushResp, err := r.adapter.PushKernel(ctx, kaggle.PushKernelRequest{
		WorkDir: bundle.WorkDir,
	})
	if err != nil {
		return report, fmt.Errorf("push kaggle kernel: %w", err)
	}
	report.Push = &PushResult{
		KernelRef: pushResp.KernelRef,
		ExitCode:  pushResp.Output.ExitCode,
		Stdout:    pushResp.Output.Stdout,
		Stderr:    pushResp.Output.Stderr,
	}

	pollResp, err := r.adapter.PollKernelStatus(ctx, kaggle.KernelPollRequest{
		KernelRef: pushResp.KernelRef,
		Interval:  time.Duration(report.PollInterval),
		Timeout:   time.Duration(report.PollTimeout),
	})
	if err != nil {
		return report, fmt.Errorf("poll kaggle kernel: %w", err)
	}
	report.Poll = &PollResult{
		KernelRef:  pollResp.KernelRef,
		Status:     pollResp.Status,
		Message:    pollResp.Message,
		Attempts:   pollResp.Attempts,
		Terminal:   pollResp.Terminal,
		StartedAt:  pollResp.StartedAt,
		FinishedAt: pollResp.FinishedAt,
		Elapsed:    Duration(pollResp.Elapsed),
		Raw:        pollResp.KernelStatusResponse.Raw,
	}

	outputDir, err := createOutputDir()
	if err != nil {
		return report, fmt.Errorf("create output dir: %w", err)
	}

	downloadResp, err := r.adapter.DownloadKernelOutput(ctx, kaggle.DownloadKernelOutputRequest{
		KernelRef: pushResp.KernelRef,
		OutputDir: outputDir,
	})
	if err != nil {
		return report, fmt.Errorf("download kaggle output: %w", err)
	}

	outputs, err := buildOutputsResult(execSpec, downloadResp.OutputDir)
	if err != nil {
		return report, fmt.Errorf("build output handoff: %w", err)
	}
	report.Outputs = &outputs

	return report, nil
}

func effectivePollInterval(requested, fallback time.Duration) time.Duration {
	if requested > 0 {
		return requested
	}
	if fallback > 0 {
		return fallback
	}
	return 5 * time.Second
}

func effectivePollTimeout(requested, fallback time.Duration) time.Duration {
	if requested > 0 {
		return requested
	}
	if fallback > 0 {
		return fallback
	}
	return 30 * time.Minute
}

const outputTempPrefix = "kgh-kernel-output-*"

func createOutputDir() (string, error) {
	outputDir, err := os.MkdirTemp("", outputTempPrefix)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(outputDir, 0o700); err != nil {
		_ = os.RemoveAll(outputDir)
		return "", err
	}
	return outputDir, nil
}

func buildOutputsResult(execSpec spec.ExecutionSpec, outputDir string) (OutputsResult, error) {
	resolvedDir, err := filepath.Abs(filepath.Clean(outputDir))
	if err != nil {
		return OutputsResult{}, err
	}

	info, err := os.Stat(resolvedDir)
	if err != nil {
		return OutputsResult{}, err
	}
	if !info.IsDir() {
		return OutputsResult{}, fmt.Errorf("output dir %q is not a directory", resolvedDir)
	}

	submission := resolveOutputFile(resolvedDir, execSpec.Outputs.Submission, "submission", execSpec.Submit)
	metrics := resolveOutputFile(resolvedDir, execSpec.Outputs.Metrics, "metrics", false)

	result := OutputsResult{
		OutputDir:      resolvedDir,
		SubmissionPath: submission.Path,
		MetricsPath:    metrics.Path,
		Submission:     submission,
		Metrics:        metrics,
		Validation: OutputValidationResult{
			Valid: true,
		},
	}

	applyOutputValidation(&result.Validation, "submission", submission)
	applyOutputValidation(&result.Validation, "metrics", metrics)

	return result, nil
}

func resolveOutputFile(outputDir, configuredPath, label string, required bool) OutputFileResult {
	result := OutputFileResult{
		ConfiguredPath: configuredPath,
		Required:       required,
	}
	if configuredPath == "" {
		result.Error = fmt.Sprintf("%s output is not configured", label)
		return result
	}

	expectedPath, err := filepath.Abs(filepath.Join(outputDir, filepath.Clean(configuredPath)))
	if err != nil {
		result.Error = fmt.Sprintf("resolve %s output path %q: %v", label, configuredPath, err)
		return result
	}
	result.ExpectedPath = expectedPath

	if !pathWithinBase(outputDir, expectedPath) {
		result.Error = fmt.Sprintf("%s output %q resolves outside output dir", label, configuredPath)
		return result
	}

	info, err := os.Stat(expectedPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Error = fmt.Sprintf("%s output is missing: %s", label, expectedPath)
			return result
		}
		result.Error = fmt.Sprintf("stat %s output %q: %v", label, expectedPath, err)
		return result
	}
	if info.IsDir() {
		result.Error = fmt.Sprintf("%s output is a directory: %s", label, expectedPath)
		return result
	}

	result.Path = expectedPath
	result.Present = true
	return result
}

func applyOutputValidation(validation *OutputValidationResult, name string, result OutputFileResult) {
	if validation == nil || result.Present {
		return
	}

	if result.Required {
		validation.Valid = false
		validation.MissingRequired = append(validation.MissingRequired, name)
		return
	}
	validation.MissingOptional = append(validation.MissingOptional, name)
}

func pathWithinBase(baseDir, target string) bool {
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return false
	}
	return rel != ".." && rel != "." && filepath.Dir(rel) != ".." && rel != ""
}
