package kaggle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestClientRunSuccess(t *testing.T) {
	t.Parallel()

	runner := &clientFakeRunner{
		t: t,
		runFn: func(ctx context.Context, cmd command) (Result, error) {
			if cmd.Path != "/usr/local/bin/kaggle" {
				t.Fatalf("unexpected path %q", cmd.Path)
			}
			if !reflect.DeepEqual(cmd.Args, []string{"/usr/local/bin/kaggle", "kernels", "status"}) {
				t.Fatalf("unexpected args %#v", cmd.Args)
			}
			if cmd.Dir != "/repo" {
				t.Fatalf("unexpected dir %q", cmd.Dir)
			}
			wantEnv := []string{
				"PATH=/usr/bin",
				"HOME=/tmp/home",
				"KAGGLE_USERNAME=alice",
				"KAGGLE_KEY=secret-key",
			}
			if !reflect.DeepEqual(cmd.Env, wantEnv) {
				t.Fatalf("unexpected env %#v", cmd.Env)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("expected context deadline to be set")
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
		func() []string {
			return []string{
				"PATH=/usr/bin",
				"HOME=/base/home",
			}
		},
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
			runFn: func(ctx context.Context, cmd command) (Result, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline to be set")
				}
				remaining := time.Until(deadline)
				if remaining <= 0 || remaining > 5*time.Second {
					t.Fatalf("unexpected deadline window %s", remaining)
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

func TestClientRunUsesExplicitTimeoutOverride(t *testing.T) {
	t.Parallel()

	client := NewClientWithDeps(
		&clientFakeRunner{
			t: t,
			runFn: func(ctx context.Context, cmd command) (Result, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline to be set")
				}
				remaining := time.Until(deadline)
				if remaining <= 0 || remaining > 1500*time.Millisecond {
					t.Fatalf("unexpected deadline window %s", remaining)
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
			runFn: func(ctx context.Context, cmd command) (Result, error) {
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
			runFn: func(context.Context, command) (Result, error) {
				return Result{
					Stdout:   "partial output",
					Stderr:   "bad things happened",
					ExitCode: 2,
				}, errors.New("exit status 2")
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
		runFn: func(ctx context.Context, cmd command) (Result, error) {
			if !reflect.DeepEqual(cmd.Args, []string{"/usr/local/bin/kaggle", "kernels", "status"}) {
				t.Fatalf("unexpected args %#v", cmd.Args)
			}
			wantEnv := []string{
				"PATH=/usr/bin",
				"KAGGLE_API_TOKEN=secret-token",
			}
			if !reflect.DeepEqual(cmd.Env, wantEnv) {
				t.Fatalf("unexpected env %#v", cmd.Env)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("expected context deadline to be set")
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

func TestClientRunSetsConfigDirForFileCredentials(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, accessTokenFilename), []byte("token-from-file"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	runner := &clientFakeRunner{
		t: t,
		runFn: func(ctx context.Context, cmd command) (Result, error) {
			wantEnv := []string{
				"PATH=/usr/bin",
				"KAGGLE_API_TOKEN=token-from-file",
				"KAGGLE_CONFIG_DIR=" + dir,
			}
			if !reflect.DeepEqual(cmd.Env, wantEnv) {
				t.Fatalf("unexpected env %#v", cmd.Env)
			}
			return Result{ExitCode: 0}, nil
		},
	}

	client := NewClientWithDeps(
		runner,
		staticEnvSource{
			envKaggleConfigDir: dir,
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
	runFn func(context.Context, command) (Result, error)
}

func (f *clientFakeRunner) Run(ctx context.Context, cmd command) (Result, error) {
	if f.runFn == nil {
		f.t.Fatal("runFn must be set")
	}
	return f.runFn(ctx, cmd)
}
