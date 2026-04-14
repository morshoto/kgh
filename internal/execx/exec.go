package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

var errCommandTimeout = errors.New("command timed out")

// Command describes an external process invocation.
type Command struct {
	Path string
	Args []string
}

// Options configures process execution.
type Options struct {
	Dir     string
	Env     []string
	Timeout time.Duration
}

// Result captures the observable process result.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes external commands.
type Runner interface {
	Run(context.Context, Command, Options) (Result, error)
}

// ExitError reports a completed process with a non-zero exit status.
type ExitError struct {
	Result Result
	Err    error
}

func (e *ExitError) Error() string {
	if e == nil || e.Err == nil {
		return "command failed"
	}
	return fmt.Sprintf("command failed with exit code %d: %v", e.Result.ExitCode, e.Err)
}

func (e *ExitError) Unwrap() error { return e.Err }

// TimeoutError reports a process that exceeded an enforced timeout.
type TimeoutError struct {
	Result  Result
	Timeout time.Duration
	Err     error
}

func (e *TimeoutError) Error() string {
	if e == nil {
		return "command timed out"
	}
	if e.Timeout <= 0 {
		return "command timed out"
	}
	return fmt.Sprintf("command timed out after %s", e.Timeout)
}

func (e *TimeoutError) Unwrap() error { return e.Err }

// NewRunner constructs a production Runner implementation.
func NewRunner() Runner {
	return NewRunnerWithBaseEnv(os.Environ)
}

// NewRunnerWithBaseEnv constructs a Runner with an injected base environment.
func NewRunnerWithBaseEnv(baseEnv func() []string) Runner {
	if baseEnv == nil {
		baseEnv = os.Environ
	}
	return runner{baseEnv: baseEnv}
}

type runner struct {
	baseEnv func() []string
}

func (r runner) Run(ctx context.Context, cmd Command, opts Options) (Result, error) {
	if cmd.Path == "" {
		return Result{}, fmt.Errorf("command path is required")
	}

	runCtx := ctx
	cancel := func() {}
	if opts.Timeout > 0 {
		runCtx, cancel = context.WithTimeoutCause(ctx, opts.Timeout, errCommandTimeout)
	}
	defer cancel()

	execCmd := exec.CommandContext(runCtx, cmd.Path, cmd.Args...)
	execCmd.Dir = opts.Dir
	execCmd.Env = mergeEnv(r.baseEnv(), opts.Env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()

	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if execCmd.ProcessState != nil {
		result.ExitCode = execCmd.ProcessState.ExitCode()
	}

	if err == nil {
		return result, nil
	}

	if errors.Is(context.Cause(runCtx), errCommandTimeout) {
		return result, &TimeoutError{
			Result:  result,
			Timeout: opts.Timeout,
			Err:     err,
		}
	}

	if runErr := runCtx.Err(); runErr != nil {
		return result, runErr
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return result, &ExitError{
			Result: result,
			Err:    err,
		}
	}

	return result, err
}

func mergeEnv(base []string, extra []string) []string {
	merged := slices.Clone(base)
	if len(extra) == 0 {
		return merged
	}

	indexByKey := make(map[string]int, len(merged))
	for i, entry := range merged {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		indexByKey[key] = i
	}

	for _, entry := range extra {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			merged = append(merged, entry)
			continue
		}
		if idx, exists := indexByKey[key]; exists {
			merged[idx] = entry
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, entry)
	}

	return merged
}
