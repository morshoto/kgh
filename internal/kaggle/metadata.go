package kaggle

import (
	"path/filepath"
	"strings"

	"github.com/shotomorisk/kgh/internal/spec"
)

const (
	defaultLanguage   = "python"
	defaultKernelType = "notebook"
)

// Metadata models Kaggle's kernel-metadata.json payload.
type Metadata struct {
	Title              string   `json:"title"`
	ID                 string   `json:"id"`
	CodeFile           string   `json:"code_file"`
	Language           string   `json:"language"`
	KernelType         string   `json:"kernel_type"`
	EnableGPU          bool     `json:"enable_gpu"`
	EnableInternet     bool     `json:"enable_internet"`
	CompetitionSources []string `json:"competition_sources"`
	DatasetSources     []string `json:"dataset_sources"`
	KernelSources      []string `json:"kernel_sources"`
	ModelSources       []string `json:"model_sources"`
	IsPrivate          bool     `json:"is_private"`
}

// BuildMetadata converts a resolved execution spec into a deterministic Kaggle metadata payload.
func BuildMetadata(exec spec.ExecutionSpec) Metadata {
	return Metadata{
		Title:              notebookTitle(exec.Notebook),
		ID:                 exec.KernelID,
		CodeFile:           filepath.Base(exec.Notebook),
		Language:           defaultLanguage,
		KernelType:         defaultKernelType,
		EnableGPU:          exec.Resources.GPU,
		EnableInternet:     exec.Resources.Internet,
		CompetitionSources: cloneStrings(exec.Sources.CompetitionSources),
		DatasetSources:     cloneStrings(exec.Sources.DatasetSources),
		KernelSources:      emptyStrings(),
		ModelSources:       emptyStrings(),
		IsPrivate:          exec.Resources.Private,
	}
}

func notebookTitle(notebookPath string) string {
	base := filepath.Base(notebookPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return emptyStrings()
	}

	out := make([]string, len(values))
	copy(out, values)
	return out
}

func emptyStrings() []string {
	return make([]string, 0)
}
