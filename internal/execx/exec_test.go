package execx

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestRunnerSuccess(t *testing.T) {
	t.Parallel()

	runner := NewRunnerWithBaseEnv(func() []string {
		return []string{"BASE=1", "KEEP=ok"}
	})

	result, err := runner.Run(context.Background(), helperCommand("success"), Options{
		Env: []string{"GO_WANT_HELPER_PROCESS=1", "BASE=2", "ADDED=yes"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code %d", result.ExitCode)
	}
	if result.Stdout != "stdout:BASE=2,ADDED=yes\n" {
		t.Fatalf("unexpected stdout %q", result.Stdout)
	}
	if result.Stderr != "stderr:KEEP=ok\n" {
		t.Fatalf("unexpected stderr %q", result.Stderr)
	}
}

func TestRunnerNonZeroExit(t *testing.T) {
	t.Parallel()

	runner := NewRunnerWithBaseEnv(func() []string { return nil })

	result, err := runner.Run(context.Background(), helperCommand("exit"), Options{
		Env: []string{"GO_WANT_HELPER_PROCESS=1"},
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("unexpected exit code %d", result.ExitCode)
	}
	if result.Stdout != "partial stdout\n" {
		t.Fatalf("unexpected stdout %q", result.Stdout)
	}
	if result.Stderr != "partial stderr\n" {
		t.Fatalf("unexpected stderr %q", result.Stderr)
	}
}

func TestRunnerTimeout(t *testing.T) {
	t.Parallel()

	runner := NewRunnerWithBaseEnv(func() []string { return nil })

	result, err := runner.Run(context.Background(), helperCommand("sleep"), Options{
		Env:     []string{"GO_WANT_HELPER_PROCESS=1"},
		Timeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T", err)
	}
	if timeoutErr.Timeout != 50*time.Millisecond {
		t.Fatalf("unexpected timeout %s", timeoutErr.Timeout)
	}
	if result.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code after timeout, got %d", result.ExitCode)
	}
}

func TestRunnerContextCancellation(t *testing.T) {
	t.Parallel()

	runner := NewRunnerWithBaseEnv(func() []string { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	_, err := runner.Run(ctx, helperCommand("sleep"), Options{
		Env:     []string{"GO_WANT_HELPER_PROCESS=1"},
		Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		t.Fatalf("expected context cancellation, got timeout classification: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestMergeEnvDoesNotMutateBase(t *testing.T) {
	t.Parallel()

	base := []string{"PATH=/usr/bin", "HOME=/tmp/home"}
	got := mergeEnv(base, []string{"HOME=/override/home"})
	if !reflect.DeepEqual(base, []string{"PATH=/usr/bin", "HOME=/tmp/home"}) {
		t.Fatalf("base environment was mutated: %#v", base)
	}
	if !reflect.DeepEqual(got, []string{"PATH=/usr/bin", "HOME=/override/home"}) {
		t.Fatalf("unexpected merged environment %#v", got)
	}
}

func helperCommand(mode string) Command {
	return Command{
		Path: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--", mode},
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}

	switch mode {
	case "success":
		_, _ = os.Stdout.WriteString("stdout:BASE=" + os.Getenv("BASE") + ",ADDED=" + os.Getenv("ADDED") + "\n")
		_, _ = os.Stderr.WriteString("stderr:KEEP=" + os.Getenv("KEEP") + "\n")
		os.Exit(0)
	case "exit":
		_, _ = os.Stdout.WriteString("partial stdout\n")
		_, _ = os.Stderr.WriteString("partial stderr\n")
		os.Exit(7)
	case "sleep":
		for {
			time.Sleep(10 * time.Millisecond)
		}
	default:
		_, _ = os.Stderr.WriteString("unknown helper mode: " + mode + "\n")
		os.Exit(2)
	}
}
