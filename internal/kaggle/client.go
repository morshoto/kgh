package kaggle

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/shotomorisk/kgh/internal/execx"
)

const defaultTimeout = 30 * time.Second

// Client is a thin, testable adapter around the Kaggle CLI.
type Client struct {
	runner         Runner
	env            EnvSource
	lookPath       LookPathFunc
	baseEnv        func() []string
	defaultTimeout time.Duration
}

type osEnvSource struct{}

func (osEnvSource) LookupEnv(key string) (string, bool) {
	return lookupEnv(key)
}

func lookupEnv(key string) (string, bool) {
	return syscallLookupEnv(key)
}

// CommandError reports a non-zero Kaggle CLI process exit.
type CommandError struct {
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	if e == nil {
		return "kaggle command failed"
	}
	return fmt.Sprintf("kaggle command failed (exit code %d): %v", e.ExitCode, e.Err)
}

func (e *CommandError) Unwrap() error { return e.Err }

// TimeoutError reports a Kaggle CLI invocation that exceeded its timeout.
type TimeoutError struct {
	Args    []string
	Timeout time.Duration
	Err     error
}

func (e *TimeoutError) Error() string {
	if e == nil {
		return "kaggle command timed out"
	}
	return fmt.Sprintf("kaggle command timed out after %s", e.Timeout)
}

func (e *TimeoutError) Unwrap() error { return e.Err }

// NewClient constructs a Kaggle CLI adapter with production dependencies.
func NewClient() *Client {
	return NewClientWithDeps(nil, osEnvSource{}, exec.LookPath, currentEnv, defaultTimeout)
}

// NewClientWithDeps constructs a Kaggle CLI adapter with injected dependencies for tests.
func NewClientWithDeps(runner Runner, env EnvSource, lookPath LookPathFunc, baseEnv func() []string, timeout time.Duration) *Client {
	if baseEnv == nil {
		baseEnv = currentEnv
	}
	if runner == nil {
		runner = execx.NewRunnerWithBaseEnv(func() []string { return nil })
	}
	if env == nil {
		env = osEnvSource{}
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Client{
		runner:         runner,
		env:            env,
		lookPath:       lookPath,
		baseEnv:        baseEnv,
		defaultTimeout: timeout,
	}
}

// Run invokes the Kaggle CLI with the provided arguments and execution options.
func (c *Client) Run(ctx context.Context, args []string, opts RunOptions) (Result, error) {
	if c == nil {
		return Result{}, fmt.Errorf("kaggle client is nil")
	}

	runtimeSetup, err := PrepareRuntime(c.env)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if runtimeSetup.Cleanup != nil {
			_ = runtimeSetup.Cleanup()
		}
	}()

	binaryPath, err := resolveExecutable(kaggleBinary, c.lookPath)
	if err != nil {
		return Result{}, err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = c.defaultTimeout
	}

	command := command{
		Path: binaryPath,
		Args: append([]string(nil), args...),
	}

	result, err := c.runner.Run(ctx, command, RunOptions{
		Dir:     opts.Dir,
		Env:     execx.MergeEnv(c.baseEnv(), append(runtimeSetup.Env, opts.Env...)),
		Timeout: timeout,
	})
	if err == nil {
		return result, nil
	}

	var timeoutErr *execx.TimeoutError
	if errors.As(err, &timeoutErr) {
		return result, &TimeoutError{
			Args:    append([]string(nil), args...),
			Timeout: timeout,
			Err:     err,
		}
	}

	var exitErr *execx.ExitError
	if errors.As(err, &exitErr) {
		return result, &CommandError{
			Args:     append([]string(nil), args...),
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
			Err:      err,
		}
	}

	return result, fmt.Errorf("run kaggle command: %w", err)
}
