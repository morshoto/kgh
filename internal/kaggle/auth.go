package kaggle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	envKaggleUsername  = "KAGGLE_USERNAME"
	envKaggleKey       = "KAGGLE_KEY"
	envKaggleAPIToken  = "KAGGLE_API_TOKEN"
	envKaggleConfigDir = "KAGGLE_CONFIG_DIR"
	envXDGConfigHome   = "XDG_CONFIG_HOME"
)

const (
	accessTokenFilename = "access_token"
	kaggleJSONFilename  = "kaggle.json"
)

type AuthMode string

const (
	AuthModeLegacy AuthMode = "legacy"
	AuthModeToken  AuthMode = "token"
)

type CredentialSource string

const (
	CredentialSourceEnvToken   CredentialSource = "env-token"
	CredentialSourceEnvLegacy  CredentialSource = "env-legacy"
	CredentialSourceFileToken  CredentialSource = "file-token"
	CredentialSourceFileLegacy CredentialSource = "file-legacy"
)

// Credentials holds the resolved Kaggle credentials plus safe source metadata.
type Credentials struct {
	Mode   AuthMode
	Source CredentialSource
	Path   string

	Username string
	Key      string
	Token    string
}

type CredentialDiagnostics struct {
	Mode   AuthMode
	Source CredentialSource
	Path   string
}

func (c Credentials) Diagnostics() CredentialDiagnostics {
	return CredentialDiagnostics{
		Mode:   c.Mode,
		Source: c.Source,
		Path:   c.Path,
	}
}

// EnvSource reads environment variables by name.
type EnvSource interface {
	LookupEnv(string) (string, bool)
}

type MissingCredentialsError struct {
	Missing []string
	Paths   []string
}

func (e *MissingCredentialsError) Error() string {
	if e == nil {
		return "missing Kaggle credentials"
	}

	parts := make([]string, 0, 2)
	if len(e.Missing) > 0 {
		parts = append(parts, fmt.Sprintf("expected one of %s", strings.Join(e.Missing, ", ")))
	}
	if len(e.Paths) > 0 {
		parts = append(parts, fmt.Sprintf("checked %s", strings.Join(e.Paths, ", ")))
	}
	if len(parts) == 0 {
		return "missing Kaggle credentials"
	}
	return fmt.Sprintf("missing Kaggle credentials: %s", strings.Join(parts, "; "))
}

type CredentialValidationError struct {
	Source  CredentialSource
	Path    string
	Missing []string
	Problem string
	Err     error
}

func (e *CredentialValidationError) Error() string {
	if e == nil {
		return "invalid Kaggle credentials"
	}

	scope := string(e.Source)
	if e.Path != "" {
		scope = fmt.Sprintf("%s (%s)", scope, e.Path)
	}

	if len(e.Missing) > 0 {
		return fmt.Sprintf("invalid Kaggle credentials from %s: missing %s", scope, strings.Join(e.Missing, ", "))
	}
	if e.Problem != "" {
		return fmt.Sprintf("invalid Kaggle credentials from %s: %s", scope, e.Problem)
	}
	if e.Err != nil {
		return fmt.Sprintf("invalid Kaggle credentials from %s: %v", scope, e.Err)
	}
	return fmt.Sprintf("invalid Kaggle credentials from %s", scope)
}

func (e *CredentialValidationError) Unwrap() error { return e.Err }

type UnsupportedAuthModeError struct {
	Mode AuthMode
}

func (e *UnsupportedAuthModeError) Error() string {
	if e == nil || e.Mode == "" {
		return "unsupported Kaggle auth mode"
	}
	return fmt.Sprintf("unsupported Kaggle auth mode: %s", e.Mode)
}

type credentialResolverDeps struct {
	readFile func(string) ([]byte, error)
	stat     func(string) (os.FileInfo, error)
	homeDir  func() (string, error)
	goos     string
}

func defaultCredentialResolverDeps() credentialResolverDeps {
	return credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  os.UserHomeDir,
		goos:     runtime.GOOS,
	}
}

// ResolveCredentials discovers Kaggle credentials from environment variables
// and Kaggle config file locations without exposing secret values.
func ResolveCredentials(env EnvSource) (Credentials, error) {
	return resolveCredentialsWithDeps(env, defaultCredentialResolverDeps())
}

// LoadCredentials preserves the historical helper name while delegating to the
// multi-source credential resolver.
func LoadCredentials(env EnvSource) (Credentials, error) {
	return ResolveCredentials(env)
}

func resolveCredentialsWithDeps(env EnvSource, deps credentialResolverDeps) (Credentials, error) {
	if env == nil {
		env = emptyEnvSource{}
	}

	if token, ok := env.LookupEnv(envKaggleAPIToken); ok {
		token = strings.TrimSpace(token)
		if token == "" {
			return Credentials{}, &CredentialValidationError{
				Source:  CredentialSourceEnvToken,
				Missing: []string{envKaggleAPIToken},
			}
		}
		return Credentials{
			Mode:   AuthModeToken,
			Source: CredentialSourceEnvToken,
			Token:  token,
		}, nil
	}

	username, hasUsername := env.LookupEnv(envKaggleUsername)
	key, hasKey := env.LookupEnv(envKaggleKey)
	if hasUsername || hasKey {
		var missing []string
		if strings.TrimSpace(username) == "" {
			missing = append(missing, envKaggleUsername)
		}
		if strings.TrimSpace(key) == "" {
			missing = append(missing, envKaggleKey)
		}
		if len(missing) > 0 {
			return Credentials{}, &CredentialValidationError{
				Source:  CredentialSourceEnvLegacy,
				Missing: missing,
			}
		}
		return Credentials{
			Mode:     AuthModeLegacy,
			Source:   CredentialSourceEnvLegacy,
			Username: username,
			Key:      key,
		}, nil
	}

	configDirs, err := resolveConfigDirs(env, deps)
	if err != nil {
		return Credentials{}, err
	}

	checkedPaths := make([]string, 0, len(configDirs)*2)
	for _, configDir := range configDirs {
		tokenPath := filepath.Join(configDir, accessTokenFilename)
		checkedPaths = append(checkedPaths, tokenPath)
		if exists, err := pathExists(tokenPath, deps.stat); err != nil {
			return Credentials{}, fmt.Errorf("check Kaggle token file: %w", err)
		} else if exists {
			token, err := deps.readFile(tokenPath)
			if err != nil {
				return Credentials{}, fmt.Errorf("read Kaggle token file: %w", err)
			}
			value := strings.TrimSpace(string(token))
			if value == "" {
				return Credentials{}, &CredentialValidationError{
					Source:  CredentialSourceFileToken,
					Path:    tokenPath,
					Missing: []string{accessTokenFilename},
				}
			}
			return Credentials{
				Mode:   AuthModeToken,
				Source: CredentialSourceFileToken,
				Path:   tokenPath,
				Token:  value,
			}, nil
		}

		legacyPath := filepath.Join(configDir, kaggleJSONFilename)
		checkedPaths = append(checkedPaths, legacyPath)
		if exists, err := pathExists(legacyPath, deps.stat); err != nil {
			return Credentials{}, fmt.Errorf("check Kaggle config file: %w", err)
		} else if exists {
			data, err := deps.readFile(legacyPath)
			if err != nil {
				return Credentials{}, fmt.Errorf("read Kaggle config file: %w", err)
			}
			var payload struct {
				Username string `json:"username"`
				Key      string `json:"key"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				return Credentials{}, &CredentialValidationError{
					Source:  CredentialSourceFileLegacy,
					Path:    legacyPath,
					Problem: "malformed kaggle.json",
					Err:     err,
				}
			}

			var missing []string
			if strings.TrimSpace(payload.Username) == "" {
				missing = append(missing, "username")
			}
			if strings.TrimSpace(payload.Key) == "" {
				missing = append(missing, "key")
			}
			if len(missing) > 0 {
				return Credentials{}, &CredentialValidationError{
					Source:  CredentialSourceFileLegacy,
					Path:    legacyPath,
					Missing: missing,
				}
			}

			return Credentials{
				Mode:     AuthModeLegacy,
				Source:   CredentialSourceFileLegacy,
				Path:     legacyPath,
				Username: payload.Username,
				Key:      payload.Key,
			}, nil
		}
	}

	return Credentials{}, &MissingCredentialsError{
		Missing: []string{envKaggleAPIToken, envKaggleUsername, envKaggleKey},
		Paths:   checkedPaths,
	}
}

func resolveConfigDirs(env EnvSource, deps credentialResolverDeps) ([]string, error) {
	if value, ok := env.LookupEnv(envKaggleConfigDir); ok {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, &CredentialValidationError{
				Source:  CredentialSourceFileLegacy,
				Missing: []string{envKaggleConfigDir},
			}
		}
		return []string{value}, nil
	}

	home, err := deps.homeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	defaultDir := filepath.Join(home, ".kaggle")
	if deps.goos != "linux" {
		return []string{defaultDir}, nil
	}

	xdgRoot, ok := env.LookupEnv(envXDGConfigHome)
	if !ok || strings.TrimSpace(xdgRoot) == "" {
		xdgRoot = filepath.Join(home, ".config")
	}
	xdgDir := filepath.Join(xdgRoot, "kaggle")
	if xdgDir == defaultDir {
		return []string{defaultDir}, nil
	}
	return []string{defaultDir, xdgDir}, nil
}

func pathExists(path string, stat func(string) (os.FileInfo, error)) (bool, error) {
	_, err := stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

type emptyEnvSource struct{}

func (emptyEnvSource) LookupEnv(string) (string, bool) {
	return "", false
}
