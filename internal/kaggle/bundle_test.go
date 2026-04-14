package kaggle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/spec"
)

func TestStageKernelBundle(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	sourceNotebook := filepath.Join(sourceDir, "notebooks", "exp142.ipynb")
	if err := os.MkdirAll(filepath.Dir(sourceNotebook), 0o755); err != nil {
		t.Fatalf("create notebook dir: %v", err)
	}
	const notebookBody = "{ \"cells\": [] }"
	if err := os.WriteFile(sourceNotebook, []byte(notebookBody), 0o644); err != nil {
		t.Fatalf("write notebook: %v", err)
	}

	exec := spec.ExecutionSpec{
		Notebook:  sourceNotebook,
		KernelID:  "yourname/exp142",
		KernelRef: "yourname/exp142",
		Resources: config.Resources{GPU: true, Internet: true, Private: false},
		Sources: config.Sources{
			CompetitionSources: []string{"playground-series-s6e2"},
			DatasetSources:     []string{"yourname/feature-pack-v3"},
		},
	}

	bundle, err := StageKernelBundle(exec)
	if err != nil {
		t.Fatalf("stage bundle: %v", err)
	}
	defer func() {
		if err := bundle.Cleanup(); err != nil {
			t.Fatalf("cleanup bundle: %v", err)
		}
	}()

	if bundle.WorkDir == "" {
		t.Fatal("expected work dir to be set")
	}
	if got, err := os.Stat(bundle.WorkDir); err != nil || !got.IsDir() {
		t.Fatalf("expected work dir to exist, got info=%v err=%v", got, err)
	}

	wantNotebook := filepath.Join(bundle.WorkDir, "exp142.ipynb")
	if bundle.NotebookPath != wantNotebook {
		t.Fatalf("unexpected notebook path %q", bundle.NotebookPath)
	}
	if bundle.MetadataPath != filepath.Join(bundle.WorkDir, metadataFilename) {
		t.Fatalf("unexpected metadata path %q", bundle.MetadataPath)
	}
	if bundle.Execution.Notebook != exec.Notebook || bundle.Execution.KernelRef != exec.KernelRef {
		t.Fatalf("unexpected execution spec %+v", bundle.Execution)
	}

	gotNotebook, err := os.ReadFile(bundle.NotebookPath)
	if err != nil {
		t.Fatalf("read staged notebook: %v", err)
	}
	if string(gotNotebook) != notebookBody {
		t.Fatalf("unexpected staged notebook body %q", string(gotNotebook))
	}

	gotMetadata, err := os.ReadFile(bundle.MetadataPath)
	if err != nil {
		t.Fatalf("read staged metadata: %v", err)
	}
	wantMetadata := `{"title":"exp142","id":"yourname/exp142","code_file":"exp142.ipynb","language":"python","kernel_type":"notebook","enable_gpu":true,"enable_internet":true,"competition_sources":["playground-series-s6e2"],"dataset_sources":["yourname/feature-pack-v3"],"kernel_sources":[],"model_sources":[],"is_private":false}`
	if string(gotMetadata) != wantMetadata {
		t.Fatalf("unexpected metadata file:\nwant %s\ngot  %s", wantMetadata, string(gotMetadata))
	}

	if err := bundle.Cleanup(); err != nil {
		t.Fatalf("cleanup bundle: %v", err)
	}
	if _, err := os.Stat(bundle.WorkDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected work dir to be removed, got err=%v", err)
	}
}

func TestStageKernelBundleMissingNotebook(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.ipynb")

	_, err := StageKernelBundle(spec.ExecutionSpec{Notebook: missing})
	if err == nil {
		t.Fatal("expected an error")
	}

	var stagingErr *StagingError
	if !errors.As(err, &stagingErr) {
		t.Fatalf("expected StagingError, got %T", err)
	}
	if stagingErr.Op != "check notebook" {
		t.Fatalf("unexpected operation %q", stagingErr.Op)
	}
	if stagingErr.Path != missing {
		t.Fatalf("unexpected path %q", stagingErr.Path)
	}
	if !strings.Contains(err.Error(), "notebook file is missing") {
		t.Fatalf("unexpected error %q", err)
	}
}
