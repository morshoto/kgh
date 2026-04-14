package kaggle

import (
	"errors"
	"os/exec"
	"testing"
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
