package github

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/execx"
)

func TestTriggerResolverResolvePushEvent(t *testing.T) {
	t.Parallel()

	resolver := TriggerResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "push",
			"GITHUB_SHA":        "abc123",
		}),
		ReadFile: os.ReadFile,
		Runner: fakeRunner{run: func(_ context.Context, cmd execx.Command, opts execx.Options) (execx.Result, error) {
			if cmd.Path != "git" {
				t.Fatalf("unexpected command %q", cmd.Path)
			}
			if got := strings.Join(cmd.Args, " "); got != "show -s --format=%B --no-patch abc123" {
				t.Fatalf("unexpected args %q", got)
			}
			if opts.Dir != "" {
				t.Fatalf("unexpected dir %q", opts.Dir)
			}
			return execx.Result{Stdout: "feat: tune pipeline\n\nsubmit: exp142 gpu=false\n"}, nil
		}},
	}

	trigger, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if trigger.Target != "exp142" {
		t.Fatalf("unexpected target %q", trigger.Target)
	}
	if trigger.GPU == nil || *trigger.GPU != false {
		t.Fatalf("expected gpu override false, got %+v", trigger.GPU)
	}
}

func TestTriggerResolverResolvePullRequestEvent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"pull_request":{"head":{"sha":"deadbeef"}}}`), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}

	resolver := TriggerResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "pull_request",
			"GITHUB_EVENT_PATH": eventPath,
			"GITHUB_WORKSPACE":  "/tmp/workspace",
		}),
		ReadFile: os.ReadFile,
		Runner: fakeRunner{run: func(_ context.Context, cmd execx.Command, opts execx.Options) (execx.Result, error) {
			if got := strings.Join(cmd.Args, " "); got != "show -s --format=%B --no-patch deadbeef" {
				t.Fatalf("unexpected args %q", got)
			}
			if opts.Dir != "/tmp/workspace" {
				t.Fatalf("unexpected workspace %q", opts.Dir)
			}
			return execx.Result{Stdout: "submit: exp142 internet=true\n"}, nil
		}},
	}

	trigger, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if trigger.Target != "exp142" {
		t.Fatalf("unexpected target %q", trigger.Target)
	}
	if trigger.Internet == nil || *trigger.Internet != true {
		t.Fatalf("expected internet override true, got %+v", trigger.Internet)
	}
}

func TestTriggerResolverRejectsUnsupportedEvent(t *testing.T) {
	t.Parallel()

	_, err := TriggerResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "workflow_dispatch",
		}),
	}.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, `unsupported GitHub event "workflow_dispatch"`) {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestTriggerResolverRejectsMissingPullRequestSHA(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	if err := os.WriteFile(eventPath, []byte(`{"pull_request":{"head":{}}}`), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}

	_, err := TriggerResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "pull_request",
			"GITHUB_EVENT_PATH": eventPath,
		}),
		ReadFile: os.ReadFile,
	}.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "pull_request.head.sha is required") {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestTriggerResolverWrapsGitReadFailure(t *testing.T) {
	t.Parallel()

	resolver := TriggerResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "push",
			"GITHUB_SHA":        "abc123",
		}),
		ReadFile: os.ReadFile,
		Runner: fakeRunner{run: func(_ context.Context, _ execx.Command, _ execx.Options) (execx.Result, error) {
			return execx.Result{}, errors.New("git failed")
		}},
	}

	_, err := resolver.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "read commit message for abc123") {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestTriggerResolverWrapsParseFailure(t *testing.T) {
	t.Parallel()

	resolver := TriggerResolver{
		Getenv: envMap(map[string]string{
			"GITHUB_EVENT_NAME": "push",
			"GITHUB_SHA":        "abc123",
		}),
		ReadFile: os.ReadFile,
		Runner: fakeRunner{run: func(_ context.Context, _ execx.Command, _ execx.Options) (execx.Result, error) {
			return execx.Result{Stdout: "feat: no trigger here\n"}, nil
		}},
	}

	_, err := resolver.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "parse commit message for abc123") {
		t.Fatalf("unexpected error %q", got)
	}
}

type fakeRunner struct {
	run func(context.Context, execx.Command, execx.Options) (execx.Result, error)
}

func (f fakeRunner) Run(ctx context.Context, cmd execx.Command, opts execx.Options) (execx.Result, error) {
	return f.run(ctx, cmd, opts)
}

func envMap(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
