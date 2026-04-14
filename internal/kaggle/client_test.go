package kaggle

import (
	"context"
	"errors"
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
		runFn: func(_ context.Context, cmd command, opts RunOptions) (Result, error) {
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
			runFn: func(_ context.Context, _ command, opts RunOptions) (Result, error) {
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
			runFn: func(_ context.Context, _ command, opts RunOptions) (Result, error) {
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
			runFn: func(_ context.Context, _ command, opts RunOptions) (Result, error) {
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
			runFn: func(context.Context, command, RunOptions) (Result, error) {
				return Result{}, &execx.TimeoutError{
					Timeout: 10 * time.Millisecond,
					Err:     context.DeadlineExceeded,
				}
			},
		},
		staticEnvSource{
			envKaggleUsername: "alice",
			envKaggleKey:      "secret-key",
		},
		func(string) (string, error) { return "/usr/local/bin/kaggle", nil },
		func() []string { return nil },
		10*time.Millisecond,
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
			runFn: func(context.Context, command, RunOptions) (Result, error) {
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
		runFn: func(_ context.Context, cmd command, opts RunOptions) (Result, error) {
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
	)

	if _, err := client.Run(context.Background(), []string{"kernels", "status"}, RunOptions{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

type clientFakeRunner struct {
	t     *testing.T
	runFn func(context.Context, command, RunOptions) (Result, error)
}

func (f *clientFakeRunner) Run(ctx context.Context, cmd command, opts RunOptions) (Result, error) {
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
