package kaggle

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/shotomorisk/kgh/internal/execx"
)

const kaggleBinary = "kaggle"

// RunOptions configures a Kaggle CLI process execution.
type RunOptions struct {
	Dir     string
	Env     []string
	Timeout time.Duration
	Debug   bool
}

// Result captures the observable output from a Kaggle CLI process.
type Result = execx.Result

type command = execx.Command

// Runner executes a prepared process invocation.
type Runner = execx.Runner

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

func currentEnv() []string {
	return os.Environ()
}
