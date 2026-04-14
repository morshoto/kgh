package kaggle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shotomorisk/kgh/internal/execx"
)

func TestClientRunSuccess(t *testing.T) {
	t.Parallel()

	runner := &clientFakeRunner{
		t: t,
		runFn: func(ctx context.Context, cmd command, opts execx.Options) (Result, error) {
			if cmd.Path != "/usr/local/bin/kaggle" {
				t.Fatalf("unexpected path %q", cmd.Path)
			}
			if !equalStrings(cmd.Args, []string{"kernels", "status"}) {
				t.Fatalf("unexpected args %#v", cmd.Args)
			}
			if opts.Dir != "/repo" {
				t.Fatalf("unexpected dir %q", opts.Dir)
			}
			if opts.Timeout != 30*time.Second {
				t.Fatalf("unexpected timeout %s", opts.Timeout)
			}
			assertEnvContains(t, opts.Env, "PATH", "/usr/bin")
			assertEnvContains(t, opts.Env, "HOME", "/tmp/home")
			assertEnvMissing(t, opts.Env, envKaggleAPIToken)
			assertEnvMissing(t, opts.Env, envKaggleUsername)
			assertEnvMissing(t, opts.Env, envKaggleKey)
			if dir := envValue(opts.Env, envKaggleConfigDir); dir == "" {
				t.Fatalf("expected %s to be set in %#v", envKaggleConfigDir, opts.Env)
			} else {
				payload, err := os.ReadFile(filepath.Join(dir, kaggleJSONFilename))
				if err != nil {
					t.Fatalf("read kaggle.json: %v", err)
				}
				if !strings.Contains(string(payload), `"username":"alice"`) || !strings.Contains(string(payload), `"key":"secret-key"`) {
					t.Fatalf("unexpected kaggle.json content %q", string(payload))
				}
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("expected deadline to be set")
			}
			return Result{
				Stdout:   "ok",
				Stderr:   "",
				ExitCode: 0,
			}, nil
		},
	}

	client := NewClientWithDeps(
		runner,
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(name string) (string, error) {
			if name != kaggleBinary {
				t.Fatalf("unexpected executable name %q", name)
			}
			return "/usr/local/bin/kaggle", nil
		},
		func() []string { return []string{"PATH=/usr/bin", "HOME=/base/home"} },
		30*time.Second,
		nil,
	)

	result, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{
		Dir: "/repo",
		Env: []string{"HOME=/tmp/home"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("unexpected stdout %q", result.Stdout)
	}
}

func TestClientRunUsesDefaultTimeout(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(ctx context.Context, _ command, opts execx.Options) (Result, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("expected deadline to be set")
				}
				if opts.Timeout != 2*time.Second {
					t.Fatalf("unexpected timeout %s", opts.Timeout)
				}
				return Result{}, nil
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return nil },
		2*time.Second,
		nil,
	)

	if _, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClientRunAppliesBaseEnvForCustomRunner(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(ctx context.Context, _ command, opts execx.Options) (Result, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("expected deadline to be set")
				}
				assertEnvContains(t, opts.Env, "PATH", "/usr/bin")
				assertEnvContains(t, opts.Env, "HOME", "/override/home")
				return Result{}, nil
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return []string{"PATH=/usr/bin", "HOME=/base/home"} },
		time.Second,
		nil,
	)

	if _, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{
		Env: []string{"HOME=/override/home"},
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClientRunUsesExplicitTimeoutOverride(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(ctx context.Context, _ command, opts execx.Options) (Result, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("expected deadline to be set")
				}
				if opts.Timeout != time.Second {
					t.Fatalf("unexpected timeout %s", opts.Timeout)
				}
				return Result{}, nil
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return nil },
		10*time.Second,
		nil,
	)

	if _, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{
		Timeout: time.Second,
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClientRunClassifiesTimeout(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(ctx context.Context, _ command, _ execx.Options) (Result, error) {
				<-ctx.Done()
				return Result{}, ctx.Err()
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return nil },
		10*time.Millisecond,
		nil,
	)

	_, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected TimeoutError, got %T", err)
	}
	if timeoutErr.Timeout != 10*time.Millisecond {
		t.Fatalf("unexpected timeout %s", timeoutErr.Timeout)
	}
}

func TestClientRunClassifiesCommandFailure(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(context.Context, command, execx.Options) (Result, error) {
				result := Result{
					Stdout:   "partial output",
					Stderr:   "bad things happened",
					ExitCode: 2,
				}
				return result, &execx.ExitError{
					Result: result,
					Err:    errors.New("exit status 2"),
				}
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return nil },
		time.Second,
		nil,
	)

	result, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}
	if commandErr.ExitCode != 2 {
		t.Fatalf("unexpected exit code %d", commandErr.ExitCode)
	}
	if commandErr.Stderr != "bad things happened" {
		t.Fatalf("unexpected stderr %q", commandErr.Stderr)
	}
	if result.Stdout != "partial output" {
		t.Fatalf("unexpected result stdout %q", result.Stdout)
	}
}

func TestClientRunReturnsExecutableLookupError(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{t: t},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) {
			return "", &ExecutableNotFoundError{Name: kaggleBinary}
		},
		func() []string { return nil },
		time.Second,
		nil,
	)

	_, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}

	var notFoundErr *ExecutableNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected ExecutableNotFoundError, got %T", err)
	}
}

func TestClientRunSupportsTokenAuth(t *testing.T) {
	t.Parallel()

	runner := &clientFakeRunner{
		t: t,
		runFn: func(_ context.Context, cmd command, opts execx.Options) (Result, error) {
			if !equalStrings(cmd.Args, []string{"kernels", "status"}) {
				t.Fatalf("unexpected args %#v", cmd.Args)
			}
			if opts.Timeout != time.Second {
				t.Fatalf("unexpected timeout %s", opts.Timeout)
			}
			assertEnvMissing(t, opts.Env, envKaggleAPIToken)
			assertEnvMissing(t, opts.Env, envKaggleUsername)
			assertEnvMissing(t, opts.Env, envKaggleKey)
			if dir := envValue(opts.Env, envKaggleConfigDir); dir == "" {
				t.Fatalf("expected %s to be set in %#v", envKaggleConfigDir, opts.Env)
			} else {
				token, err := os.ReadFile(filepath.Join(dir, accessTokenFilename))
				if err != nil {
					t.Fatalf("read access_token: %v", err)
				}
				if string(token) != "secret-token" {
					t.Fatalf("unexpected token file content %q", string(token))
				}
			}
			return Result{ExitCode: 0}, nil
		},
	}

	client := NewClientWithDeps(
		runner,
		staticEnvSource{
			envKaggleAPIToken: "secret-token",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return []string{"PATH=/usr/bin"} },
		time.Second,
		nil,
	)

	if _, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClientRunLogsDebugContextWithRedaction(t *testing.T) {
	t.Parallel()

	logger := &fakeOperationLogger{}
	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(_ context.Context, _ command, _ execx.Options) (Result, error) {
				return Result{ExitCode: 0}, nil
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return []string{"PATH=/usr/bin", envKaggleAPIToken + "=base-secret"} },
		time.Second,
		logger,
	)

	if _, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{
		Debug: true,
		Env: []string{
			envKaggleUsername + "=alice",
			"HOME=/tmp/home",
		},
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(logger.infoLogs) < 2 {
		t.Fatalf("expected info logs, got %+v", logger.infoLogs)
	}
	start := strings.Join(logger.infoLogs[0].attrs, " ")
	if !strings.Contains(start, "operation kernels status") {
		t.Fatalf("unexpected log attrs %q", start)
	}
	if !strings.Contains(start, envKaggleUsername+"=[REDACTED]") {
		t.Fatalf("expected redacted username in %q", start)
	}
	if !strings.Contains(start, envKaggleAPIToken+"=[REDACTED]") {
		t.Fatalf("expected redacted token in %q", start)
	}
	if strings.Contains(start, "secret-key") || strings.Contains(start, "base-secret") {
		t.Fatalf("unexpected secret leakage in %q", start)
	}
}

func TestClientRunLogsActionableFailure(t *testing.T) {
	t.Parallel()

	logger := &fakeOperationLogger{}
	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(_ context.Context, _ command, _ execx.Options) (Result, error) {
				result := Result{ExitCode: 2, Stderr: "permission denied"}
				return result, &execx.ExitError{Result: result, Err: errors.New("exit status 2")}
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return nil },
		time.Second,
		logger,
	)

	if _, err := client.Run(context.Background(), []string{"competitions", "submit"}, RunOptions{}); err == nil {
		t.Fatal("expected an error")
	}

	if len(logger.errorLogs) == 0 {
		t.Fatal("expected error logs")
	}
	entry := strings.Join(logger.errorLogs[0].attrs, " ")
	if !strings.Contains(entry, "exit_code 2") || !strings.Contains(entry, "permission denied") {
		t.Fatalf("unexpected error log attrs %q", entry)
	}
}

type clientFakeRunner struct {
	t     *testing.T
	runFn func(context.Context, command, execx.Options) (Result, error)
}

func (f *clientFakeRunner) Run(ctx context.Context, cmd command, opts execx.Options) (Result, error) {
	if f.runFn == nil {
		f.t.Fatal("runFn must be set")
	}
	return f.runFn(ctx, cmd, opts)
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

type fakeOperationLogger struct {
	infoLogs  []fakeLogEntry
	errorLogs []fakeLogEntry
}

type fakeLogEntry struct {
	msg   string
	attrs []string
}

func (l *fakeOperationLogger) InfoContext(_ context.Context, msg string, args ...any) {
	l.infoLogs = append(l.infoLogs, fakeLogEntry{msg: msg, attrs: stringifyAttrs(args)})
}

func (l *fakeOperationLogger) ErrorContext(_ context.Context, msg string, args ...any) {
	l.errorLogs = append(l.errorLogs, fakeLogEntry{msg: msg, attrs: stringifyAttrs(args)})
}

func stringifyAttrs(args []any) []string {
	values := make([]string, 0, len(args))
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			values = append(values, fmt.Sprint(args[i]))
			continue
		}
		values = append(values, fmt.Sprintf("%v %v", args[i], args[i+1]))
	}
	return values
}
