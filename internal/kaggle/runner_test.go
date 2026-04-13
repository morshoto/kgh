package kaggle

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func TestResolveExecutable(t *testing.T) {
	t.Parallel()

	path, err := resolveExecutable(kaggleBinary, func(name string) (string, error) {
		if name != kaggleBinary {
			t.Fatalf("unexpected executable lookup %q", name)
		}
		return "/usr/local/bin/kaggle", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if path != "/usr/local/bin/kaggle" {
		t.Fatalf("unexpected path %q", path)
	}
}

func TestResolveExecutableMissing(t *testing.T) {
	t.Parallel()

	_, err := resolveExecutable(kaggleBinary, func(string) (string, error) {
		return "", &exec.Error{Name: kaggleBinary, Err: exec.ErrNotFound}
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var notFoundErr *ExecutableNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected ExecutableNotFoundError, got %T", err)
	}
}

func TestBuildCommandMergesEnvironment(t *testing.T) {
	t.Parallel()

	cmd := buildCommand("/usr/local/bin/kaggle", []string{"kernels", "status"}, []string{
		"PATH=/usr/bin",
		"HOME=/tmp/home",
	}, RunOptions{
		Dir: "/work",
		Env: []string{
			"HOME=/override/home",
			"KAGGLE_USERNAME=alice",
		},
		Timeout: 5 * time.Second,
	})

	if cmd.Path != "/usr/local/bin/kaggle" {
		t.Fatalf("unexpected path %q", cmd.Path)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"/usr/local/bin/kaggle", "kernels", "status"}) {
		t.Fatalf("unexpected args %#v", cmd.Args)
	}
	if cmd.Dir != "/work" {
		t.Fatalf("unexpected dir %q", cmd.Dir)
	}
	wantEnv := []string{
		"PATH=/usr/bin",
		"HOME=/override/home",
		"KAGGLE_USERNAME=alice",
	}
	if !reflect.DeepEqual(cmd.Env, wantEnv) {
		t.Fatalf("unexpected env %#v", cmd.Env)
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

type fakeRunner struct {
	t      *testing.T
	result Result
	err    error
	seen   command
}

func (f *fakeRunner) Run(_ context.Context, cmd command) (Result, error) {
	if f.t == nil {
		panic("fakeRunner.t must be set")
	}
	f.seen = cmd
	return f.result, f.err
}
