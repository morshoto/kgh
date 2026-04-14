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
		return Result{}, fmt.Errorf("live execution is not implemented yet")
	}

	return report, nil
}
