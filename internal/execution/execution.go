package execution

import (
	"context"
	"fmt"
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
	Target     string
	DryRun     bool
	GPU        *bool
	Internet   *bool
	ConfigPath string
}

type Result struct {
	Mode       string             `json:"mode"`
	DryRun     bool               `json:"dry_run"`
	ConfigPath string             `json:"config_path"`
	Execution  spec.ExecutionSpec `json:"execution"`
	Bundle     *BundleResult      `json:"bundle,omitempty"`
	Push       *PushResult        `json:"push,omitempty"`
	Poll       *PollResult        `json:"poll,omitempty"`
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
	Elapsed    time.Duration                  `json:"elapsed"`
	Raw        kaggle.KernelStatusRawStatus   `json:"raw,omitempty"`
}

type Adapter interface {
	PushKernel(context.Context, kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error)
	PollKernelStatus(context.Context, kaggle.KernelPollRequest) (kaggle.KernelPollResult, error)
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
		Mode:       ModeDryRun,
		DryRun:     true,
		ConfigPath: req.ConfigPath,
		Execution:  execSpec,
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

	pollTimeout := r.pollTimeout
	if pollTimeout <= 0 {
		pollTimeout = 30 * time.Minute
	}
	pollInterval := r.pollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	pollResp, err := r.adapter.PollKernelStatus(ctx, kaggle.KernelPollRequest{
		KernelRef: pushResp.KernelRef,
		Interval:  pollInterval,
		Timeout:   pollTimeout,
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
		Elapsed:    pollResp.Elapsed,
		Raw:        pollResp.KernelStatusResponse.Raw,
	}

	return report, nil
}
