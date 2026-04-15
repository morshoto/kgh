package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_Help(t *testing.T) {
	if code := run([]string{"help"}); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRun_StripsCommandNamePrefix(t *testing.T) {
	if code := run([]string{"kgh", "help"}); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunHelpMentionsRun(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	if code := runWithIO([]string{"help"}, stdout, stderr); code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "run       Resolve and execute a Kaggle target") {
		t.Fatalf("expected run command in help output, got %s", stdout.String())
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	if code := run([]string{"wat"}); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestParseRunFlagsRequiresTarget(t *testing.T) {
	t.Parallel()

	if _, err := parseRunFlags([]string{"--dry-run"}); err == nil || !strings.Contains(err.Error(), "--target is required") {
		t.Fatalf("expected target validation error, got %v", err)
	}
}

func TestParseRunFlagsValidatesOverrides(t *testing.T) {
	t.Parallel()

	if _, err := parseRunFlags([]string{"--target", "exp142", "--gpu", "maybe"}); err == nil || !strings.Contains(err.Error(), "invalid value for --gpu") {
		t.Fatalf("expected gpu validation error, got %v", err)
	}
}

func TestParseRunFlagsAcceptsPollingDurations(t *testing.T) {
	t.Parallel()

	flags, err := parseRunFlags([]string{"--target", "exp142", "--poll-interval", "2s", "--timeout", "5m"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if flags.pollInterval != 2*time.Second {
		t.Fatalf("unexpected poll interval %s", flags.pollInterval)
	}
	if flags.timeout != 5*time.Minute {
		t.Fatalf("unexpected timeout %s", flags.timeout)
	}
}

func TestParseRunFlagsRejectsNegativePollingDurations(t *testing.T) {
	t.Parallel()

	if _, err := parseRunFlags([]string{"--target", "exp142", "--poll-interval", "-1s"}); err == nil || !strings.Contains(err.Error(), "--poll-interval must be greater than or equal to 0") {
		t.Fatalf("expected poll interval validation error, got %v", err)
	}
	if _, err := parseRunFlags([]string{"--target", "exp142", "--timeout", "-1s"}); err == nil || !strings.Contains(err.Error(), "--timeout must be greater than or equal to 0") {
		t.Fatalf("expected timeout validation error, got %v", err)
	}
}

func TestRunDryRunOutputsJSON(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".kgh")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`
targets:
  exp142:
    notebook: notebooks/exp142.ipynb
    kernel_id: yourname/exp142
    competition: playground-series-s6e2
    submit: true
    resources:
      gpu: true
      internet: false
      private: true
    outputs:
      submission: submission.csv
      metrics: metrics.json
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if code := runWithIO([]string{"run", "--target", "exp142"}, stdout, stderr); code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}

	if got := stdout.String(); !strings.Contains(got, `"mode": "dry-run"`) {
		t.Fatalf("expected dry-run JSON, got %s", got)
	}
	if !strings.Contains(stdout.String(), `"target_name": "exp142"`) {
		t.Fatalf("expected target name in output, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"poll_interval": "5s"`) {
		t.Fatalf("expected default poll interval in output, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"poll_timeout": "30m0s"`) {
		t.Fatalf("expected default poll timeout in output, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"gpu": true`) {
		t.Fatalf("expected snake_case resources in output, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"competition_sources": []`) {
		t.Fatalf("expected empty sources arrays in output, got %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"overrides": {}`) {
		t.Fatalf("expected empty overrides object in output, got %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %s", stderr.String())
	}
}
