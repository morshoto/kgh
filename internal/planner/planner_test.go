package planner

import (
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/parser"
)

func TestResolveUsesTargetDefaults(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Targets: map[string]config.Target{
			"exp142": {
				Notebook:    "notebooks/exp142.ipynb",
				KernelID:    "yourname/exp142",
				Competition: "playground-series-s6e2",
				Submit:      true,
				Resources: config.Resources{
					GPU:      true,
					Internet: false,
					Private:  true,
				},
				Sources: config.Sources{
					CompetitionSources: []string{"playground-series-s6e2"},
					DatasetSources:     []string{"yourname/feature-pack-v3"},
				},
				Outputs: config.Outputs{
					Submission: "submission.csv",
					Metrics:    "metrics.json",
				},
			},
		},
	}

	got, err := Resolve(cfg, parser.Trigger{Target: "exp142"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.TargetName != "exp142" {
		t.Fatalf("expected target name %q, got %q", "exp142", got.TargetName)
	}
	if got.Notebook != "notebooks/exp142.ipynb" {
		t.Fatalf("unexpected notebook %q", got.Notebook)
	}
	if got.KernelID != "yourname/exp142" {
		t.Fatalf("unexpected kernel id %q", got.KernelID)
	}
	if got.Resources.GPU != true || got.Resources.Internet != false || got.Resources.Private != true {
		t.Fatalf("unexpected resources: %+v", got.Resources)
	}
	if got.Overrides.GPU != nil || got.Overrides.Internet != nil {
		t.Fatalf("expected no overrides, got %+v", got.Overrides)
	}
}

func TestResolveAppliesOverrides(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Targets: map[string]config.Target{
			"exp142": {
				Notebook:    "notebooks/exp142.ipynb",
				KernelID:    "yourname/exp142",
				Competition: "playground-series-s6e2",
				Submit:      true,
				Resources: config.Resources{
					GPU:      true,
					Internet: false,
					Private:  true,
				},
			},
		},
	}
	gpu := false
	internet := true

	got, err := Resolve(cfg, parser.Trigger{
		Target:   "exp142",
		GPU:      &gpu,
		Internet: &internet,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Resources.GPU != false {
		t.Fatalf("expected gpu override to false, got %+v", got.Resources)
	}
	if got.Resources.Internet != true {
		t.Fatalf("expected internet override to true, got %+v", got.Resources)
	}
	if got.Resources.Private != true {
		t.Fatalf("expected private to remain true, got %+v", got.Resources)
	}
	if got.Overrides.GPU == nil || *got.Overrides.GPU != false {
		t.Fatalf("expected gpu override to be preserved in spec, got %+v", got.Overrides)
	}
	if got.Overrides.Internet == nil || *got.Overrides.Internet != true {
		t.Fatalf("expected internet override to be preserved in spec, got %+v", got.Overrides)
	}
}

func TestResolveUnknownTarget(t *testing.T) {
	t.Parallel()

	_, err := Resolve(config.Config{Targets: map[string]config.Target{}}, parser.Trigger{Target: "missing"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, `unknown target "missing"`) {
		t.Fatalf("expected unknown target error, got %q", got)
	}
}
