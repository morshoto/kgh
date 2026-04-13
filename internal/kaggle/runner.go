package kaggle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"
)

const kaggleBinary = "kaggle"

// RunOptions configures a Kaggle CLI process execution.
type RunOptions struct {
	Timeout time.Duration
	Dir     string
	Env     []string
}

// Result captures the observable output from a Kaggle CLI process.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type command struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

// Runner executes a prepared process invocation.
type Runner interface {
	Run(context.Context, command) (Result, error)
}

// LookPathFunc resolves an executable from PATH.
type LookPathFunc func(string) (string, error)

// ExecutableNotFoundError reports a missing executable lookup failure.
type ExecutableNotFoundError struct {
	Name string
}

func (e *ExecutableNotFoundError) Error() string {
	if e == nil || e.Name == "" {
		return "required executable not found"
	}
	return fmt.Sprintf("required executable %q not found on PATH", e.Name)
}

func resolveExecutable(name string, lookPath LookPathFunc) (string, error) {
	path, err := lookPath(name)
	if err == nil {
		return path, nil
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return "", &ExecutableNotFoundError{Name: name}
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", &ExecutableNotFoundError{Name: name}
	}
	return "", fmt.Errorf("resolve executable %q: %w", name, err)
}

func buildCommand(path string, args []string, baseEnv []string, opts RunOptions) command {
	return command{
		Path: path,
		Args: append([]string{path}, args...),
		Dir:  opts.Dir,
		Env:  mergeEnv(baseEnv, opts.Env),
	}
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

type execRunner struct{}

func (execRunner) Run(ctx context.Context, cmd command) (Result, error) {
	execCmd := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	execCmd.Dir = cmd.Dir
	execCmd.Env = cmd.Env

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

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		}
		return result, err
	}

	return result, nil
}

func currentEnv() []string {
	return os.Environ()
}
