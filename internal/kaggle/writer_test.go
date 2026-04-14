package kaggle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/spec"
)

func TestWriteKernelMetadata(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	exec := spec.ExecutionSpec{
		Notebook:  "notebooks/exp142.ipynb",
		KernelID:  "yourname/exp142",
		KernelRef: "yourname/exp142",
		Resources: config.Resources{GPU: true, Internet: true, Private: false},
		Sources:   config.Sources{CompetitionSources: []string{"playground-series-s6e2"}, DatasetSources: []string{"yourname/feature-pack-v3"}},
	}

	gotPath, err := WriteKernelMetadata(workDir, exec)
	if err != nil {
		t.Fatalf("write kernel metadata: %v", err)
	}

	wantPath := filepath.Join(workDir, metadataFilename)
	if gotPath != wantPath {
		t.Fatalf("expected path %q, got %q", wantPath, gotPath)
	}

	b, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	want := `{"title":"exp142","id":"yourname/exp142","code_file":"exp142.ipynb","language":"python","kernel_type":"notebook","enable_gpu":true,"enable_internet":true,"competition_sources":["playground-series-s6e2"],"dataset_sources":["yourname/feature-pack-v3"],"kernel_sources":[],"model_sources":[],"is_private":false}`
	if got := string(b); got != want {
		t.Fatalf("unexpected metadata file:\nwant %s\ngot  %s", want, got)
	}
}
