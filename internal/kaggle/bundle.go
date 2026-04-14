package kaggle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shotomorisk/kgh/internal/spec"
)

const bundleTempPrefix = "kgh-kaggle-bundle-*"

// StagedBundle describes a prepared Kaggle push directory.
type StagedBundle struct {
	WorkDir      string
	NotebookPath string
	MetadataPath string
	Execution    spec.ExecutionSpec
	Cleanup      func() error
}

// StagingError reports a failure while preparing a local Kaggle bundle.
type StagingError struct {
	Op   string
	Path string
	Err  error
}

func (e *StagingError) Error() string {
	if e == nil {
		return "prepare kaggle bundle"
	}
	if e.Path != "" {
		return fmt.Sprintf("prepare kaggle bundle: %s %s: %v", e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("prepare kaggle bundle: %s: %v", e.Op, e.Err)
}

func (e *StagingError) Unwrap() error { return e.Err }

// StageKernelBundle creates a temporary Kaggle bundle directory for the resolved execution spec.
func StageKernelBundle(exec spec.ExecutionSpec) (StagedBundle, error) {
	if err := validateNotebookSource(exec.Notebook); err != nil {
		return StagedBundle{}, err
	}

	workDir, cleanup, err := createBundleDir()
	if err != nil {
		return StagedBundle{}, err
	}

	bundle := StagedBundle{
		WorkDir:   workDir,
		Execution: exec,
		Cleanup:   cleanup,
	}

	notebookPath, err := copyNotebookToBundle(exec.Notebook, workDir)
	if err != nil {
		_ = cleanup()
		return StagedBundle{}, err
	}
	bundle.NotebookPath = notebookPath

	metadataPath, err := WriteKernelMetadata(workDir, exec)
	if err != nil {
		_ = cleanup()
		return StagedBundle{}, &StagingError{
			Op:   "write metadata",
			Path: filepath.Join(workDir, metadataFilename),
			Err:  err,
		}
	}
	bundle.MetadataPath = metadataPath

	if err := verifyBundleFiles(bundle.NotebookPath, bundle.MetadataPath); err != nil {
		_ = cleanup()
		return StagedBundle{}, err
	}

	return bundle, nil
}

func validateNotebookSource(path string) error {
	if strings.TrimSpace(path) == "" {
		return &StagingError{
			Op:  "validate notebook",
			Err: fmt.Errorf("notebook path is required"),
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &StagingError{
				Op:   "check notebook",
				Path: path,
				Err:  fmt.Errorf("notebook file is missing"),
			}
		}
		return &StagingError{
			Op:   "check notebook",
			Path: path,
			Err:  err,
		}
	}
	if info.IsDir() {
		return &StagingError{
			Op:   "check notebook",
			Path: path,
			Err:  fmt.Errorf("notebook path must be a file"),
		}
	}

	return nil
}

func createBundleDir() (string, func() error, error) {
	workDir, err := os.MkdirTemp("", bundleTempPrefix)
	if err != nil {
		return "", nil, &StagingError{
			Op:  "create temp dir",
			Err: err,
		}
	}

	if err := os.Chmod(workDir, 0o700); err != nil {
		_ = os.RemoveAll(workDir)
		return "", nil, &StagingError{
			Op:   "chmod temp dir",
			Path: workDir,
			Err:  err,
		}
	}

	cleanup := func() error {
		if err := os.RemoveAll(workDir); err != nil {
			return &StagingError{
				Op:   "cleanup",
				Path: workDir,
				Err:  err,
			}
		}
		return nil
	}

	return workDir, cleanup, nil
}

func copyNotebookToBundle(src, workDir string) (string, error) {
	dst := filepath.Join(workDir, filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return "", &StagingError{
			Op:   "copy notebook",
			Path: src,
			Err:  err,
		}
	}
	return dst, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func verifyBundleFiles(paths ...string) error {
	for _, path := range paths {
		if err := verifyFile(path); err != nil {
			return err
		}
	}
	return nil
}

func verifyFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &StagingError{
				Op:   "verify file",
				Path: path,
				Err:  fmt.Errorf("required file is missing"),
			}
		}
		return &StagingError{
			Op:   "verify file",
			Path: path,
			Err:  err,
		}
	}
	if info.IsDir() {
		return &StagingError{
			Op:   "verify file",
			Path: path,
			Err:  fmt.Errorf("expected a file, found a directory"),
		}
	}
	return nil
}
