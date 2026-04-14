package kaggle

import (
	"encoding/json"
	"testing"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/spec"
)

func TestBuildMetadata(t *testing.T) {
	t.Parallel()

	exec := spec.ExecutionSpec{
		Notebook:  "notebooks/exp142.ipynb",
		KernelID:  "yourname/exp142",
		KernelRef: "yourname/exp142",
		Resources: config.Resources{GPU: true, Internet: false, Private: true},
		Sources:   config.Sources{CompetitionSources: []string{"playground-series-s6e2"}, DatasetSources: []string{"yourname/feature-pack-v3"}},
	}

	got := BuildMetadata(exec)

	if got.Title != "exp142" {
		t.Fatalf("expected title %q, got %q", "exp142", got.Title)
	}
	if got.ID != "yourname/exp142" {
		t.Fatalf("expected id %q, got %q", "yourname/exp142", got.ID)
	}
	if got.CodeFile != "exp142.ipynb" {
		t.Fatalf("expected code_file %q, got %q", "exp142.ipynb", got.CodeFile)
	}
	if got.Language != defaultLanguage {
		t.Fatalf("expected language %q, got %q", defaultLanguage, got.Language)
	}
	if got.KernelType != defaultKernelType {
		t.Fatalf("expected kernel_type %q, got %q", defaultKernelType, got.KernelType)
	}
	if !got.EnableGPU || got.EnableInternet {
		t.Fatalf("unexpected resource flags: %+v", got)
	}
	if !got.IsPrivate {
		t.Fatalf("expected is_private to be true, got %+v", got)
	}
	if len(got.KernelSources) != 0 || len(got.ModelSources) != 0 {
		t.Fatalf("expected empty kernel/model sources, got %+v", got)
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	const want = `{"title":"exp142","id":"yourname/exp142","code_file":"exp142.ipynb","language":"python","kernel_type":"notebook","enable_gpu":true,"enable_internet":false,"competition_sources":["playground-series-s6e2"],"dataset_sources":["yourname/feature-pack-v3"],"kernel_sources":[],"model_sources":[],"is_private":true}`
	if gotJSON := string(b); gotJSON != want {
		t.Fatalf("unexpected json:\nwant %s\ngot  %s", want, gotJSON)
	}
}

func TestBuildMetadataDeterministic(t *testing.T) {
	t.Parallel()

	exec := spec.ExecutionSpec{
		Notebook:  "notebooks/exp142.ipynb",
		KernelID:  "yourname/exp142",
		KernelRef: "yourname/exp142",
		Sources: config.Sources{
			CompetitionSources: []string{"b", "a"},
			DatasetSources:     []string{"x", "y"},
		},
	}

	first, err := json.Marshal(BuildMetadata(exec))
	if err != nil {
		t.Fatalf("marshal first metadata: %v", err)
	}
	second, err := json.Marshal(BuildMetadata(exec))
	if err != nil {
		t.Fatalf("marshal second metadata: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("expected deterministic json, got %q and %q", string(first), string(second))
	}
}
