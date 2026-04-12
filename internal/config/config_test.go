package config

import (
	"strings"
	"testing"
)

func TestParseValidConfig(t *testing.T) {
	cfg, err := Parse(DefaultPath, []byte(`
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
    sources:
      competition_sources:
        - playground-series-s6e2
      dataset_sources:
        - yourname/feature-pack-v3
    outputs:
      submission: submission.csv
      metrics: metrics.json
`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := len(cfg.Targets); got != 1 {
		t.Fatalf("expected 1 target, got %d", got)
	}
	if got := cfg.Targets["exp142"].KernelID; got != "yourname/exp142" {
		t.Fatalf("unexpected kernel id %q", got)
	}
}

func TestParseMissingRequiredField(t *testing.T) {
	_, err := Parse(DefaultPath, []byte(`
targets:
  exp142:
    kernel_id: yourname/exp142
    competition: playground-series-s6e2
`))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "targets.exp142.notebook: required") {
		t.Fatalf("expected missing notebook error, got %q", got)
	}
}

func TestParseInvalidTargetsType(t *testing.T) {
	_, err := Parse(DefaultPath, []byte(`
targets: []
`))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "targets: must be a mapping") {
		t.Fatalf("expected targets mapping error, got %q", got)
	}
}
