package kaggle

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/shotomorisk/kgh/internal/spec"
)

const metadataFilename = "kernel-metadata.json"

// WriteKernelMetadata writes a kernel-metadata.json file into workDir and returns the path.
func WriteKernelMetadata(workDir string, exec spec.ExecutionSpec) (string, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}

	payload, err := json.Marshal(BuildMetadata(exec))
	if err != nil {
		return "", err
	}

	path := filepath.Join(workDir, metadataFilename)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", err
	}

	return path, nil
}
