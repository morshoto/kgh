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
	CredentialSourceEnvLegacy  CredentialSource = "env-legacy"
	CredentialSourceEnvToken   CredentialSource = "env-token"
	CredentialSourceFileLegacy CredentialSource = "file-legacy"
	CredentialSourceFileToken  CredentialSource = "file-token"
)

// Credentials holds the Kaggle auth material resolved from environment input.
type Credentials struct {
	Mode   AuthMode
	Source CredentialSource
	Path   string

	Username string
	Key      string
	Token    string
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
		parts = append(parts, strings.Join(e.Missing, ", "))
	}
	if len(e.Paths) > 0 {
		parts = append(parts, "checked "+strings.Join(e.Paths, ", "))
	}
	if len(parts) == 0 {
		return "missing Kaggle credentials"
	}
	return fmt.Sprintf("missing Kaggle credentials: %s", strings.Join(parts, "; "))
}

type CredentialValidationError struct {
	Fields  []string
	Problem string
	Source  CredentialSource
	Path    string
	Err     error
}

func (e *CredentialValidationError) Error() string {
	if e == nil {
		return "invalid Kaggle credentials"
	}
	scope := "credentials"
	if e.Source != "" {
		scope = string(e.Source)
	}
	if e.Path != "" {
		scope += " (" + e.Path + ")"
	}
	if len(e.Fields) > 0 {
		return fmt.Sprintf("invalid Kaggle credentials from %s: %s", scope, strings.Join(e.Fields, ", "))
	}
	if e.Problem != "" {
		return fmt.Sprintf("invalid Kaggle credentials from %s: %s", scope, e.Problem)
	}
	if e.Err != nil {
		return fmt.Sprintf("invalid Kaggle credentials from %s: %v", scope, e.Err)
	}
	return "invalid Kaggle credentials"
}

func (e *CredentialValidationError) Unwrap() error { return e.Err }

type RuntimeSetup struct {
	Env       []string
	ConfigDir string
	AuthMode  AuthMode
	Source    CredentialSource
	Cleanup   func() error
}

type RuntimeSetupError struct {
	Op   string
	Path string
	Err  error
}

func (e *RuntimeSetupError) Error() string {
	if e == nil {
		return "prepare Kaggle runtime"
	}
	if e.Path != "" {
		return fmt.Sprintf("prepare Kaggle runtime: %s %s: %v", e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("prepare Kaggle runtime: %s: %v", e.Op, e.Err)
}

func (e *RuntimeSetupError) Unwrap() error { return e.Err }

type runtimeSetupDeps struct {
	mkdirTemp func(string, string) (string, error)
	writeFile func(string, []byte, os.FileMode) error
	chmod     func(string, os.FileMode) error
	removeAll func(string) error
}

func defaultRuntimeSetupDeps() runtimeSetupDeps {
	return runtimeSetupDeps{
		mkdirTemp: os.MkdirTemp,
		writeFile: os.WriteFile,
		chmod:     os.Chmod,
		removeAll: os.RemoveAll,
	}
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

// LoadCredentials reads Kaggle credentials from the provided environment source.
func LoadCredentials(env EnvSource) (Credentials, error) {
	return ResolveCredentials(env)
}

// ResolveCredentials discovers Kaggle credentials from the process environment and Kaggle config files.
func ResolveCredentials(env EnvSource) (Credentials, error) {
	return resolveCredentialsWithDeps(env, defaultCredentialResolverDeps())
}

func resolveCredentialsWithDeps(env EnvSource, deps credentialResolverDeps) (Credentials, error) {
	if env == nil {
		env = emptyEnvSource{}
	}

	if token, ok := env.LookupEnv(envKaggleAPIToken); ok {
		token = strings.TrimSpace(token)
		if token == "" {
			return Credentials{}, &CredentialValidationError{
				Fields: []string{envKaggleAPIToken},
				Source: CredentialSourceEnvToken,
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
		var invalid []string
		if strings.TrimSpace(username) == "" {
			invalid = append(invalid, envKaggleUsername)
		}
		if strings.TrimSpace(key) == "" {
			invalid = append(invalid, envKaggleKey)
		}
		if len(invalid) > 0 {
			return Credentials{}, &CredentialValidationError{
				Fields: invalid,
				Source: CredentialSourceEnvLegacy,
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
	for _, dir := range configDirs {
		tokenPath := filepath.Join(dir, accessTokenFilename)
		checkedPaths = append(checkedPaths, tokenPath)
		if exists, err := pathExists(tokenPath, deps.stat); err != nil {
			return Credentials{}, fmt.Errorf("check Kaggle token file: %w", err)
		} else if exists {
			content, err := deps.readFile(tokenPath)
			if err != nil {
				return Credentials{}, fmt.Errorf("read Kaggle token file: %w", err)
			}
			token := strings.TrimSpace(string(content))
			if token == "" {
				return Credentials{}, &CredentialValidationError{
					Fields: []string{accessTokenFilename},
					Source: CredentialSourceFileToken,
					Path:   tokenPath,
				}
			}
			return Credentials{
				Mode:   AuthModeToken,
				Source: CredentialSourceFileToken,
				Path:   tokenPath,
				Token:  token,
			}, nil
		}

		configPath := filepath.Join(dir, kaggleJSONFilename)
		checkedPaths = append(checkedPaths, configPath)
		if exists, err := pathExists(configPath, deps.stat); err != nil {
			return Credentials{}, fmt.Errorf("check Kaggle config file: %w", err)
		} else if exists {
			content, err := deps.readFile(configPath)
			if err != nil {
				return Credentials{}, fmt.Errorf("read Kaggle config file: %w", err)
			}
			var payload struct {
				Username string `json:"username"`
				Key      string `json:"key"`
			}
			if err := json.Unmarshal(content, &payload); err != nil {
				return Credentials{}, &CredentialValidationError{
					Problem: "malformed kaggle.json",
					Source:  CredentialSourceFileLegacy,
					Path:    configPath,
					Err:     err,
				}
			}
			var invalid []string
			if strings.TrimSpace(payload.Username) == "" {
				invalid = append(invalid, "username")
			}
			if strings.TrimSpace(payload.Key) == "" {
				invalid = append(invalid, "key")
			}
			if len(invalid) > 0 {
				return Credentials{}, &CredentialValidationError{
					Fields: invalid,
					Source: CredentialSourceFileLegacy,
					Path:   configPath,
				}
			}
			return Credentials{
				Mode:     AuthModeLegacy,
				Source:   CredentialSourceFileLegacy,
				Path:     configPath,
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

// PrepareRuntime converts GitHub Actions environment credentials into a Kaggle CLI-ready runtime.
func PrepareRuntime(env EnvSource) (RuntimeSetup, error) {
	return prepareRuntimeWithDeps(env, defaultRuntimeSetupDeps())
}

func prepareRuntimeWithDeps(env EnvSource, deps runtimeSetupDeps) (RuntimeSetup, error) {
	creds, err := ResolveCredentials(env)
	if err != nil {
		return RuntimeSetup{}, err
	}

	dir, err := deps.mkdirTemp("", "kgh-kaggle-*")
	if err != nil {
		return RuntimeSetup{}, &RuntimeSetupError{Op: "create temp dir", Err: err}
	}
	cleanup := func() error {
		if err := deps.removeAll(dir); err != nil {
			return &RuntimeSetupError{Op: "cleanup", Path: dir, Err: err}
		}
		return nil
	}

	if err := deps.chmod(dir, 0o700); err != nil {
		_ = cleanup()
		return RuntimeSetup{}, &RuntimeSetupError{Op: "chmod", Path: dir, Err: err}
	}

	setup := RuntimeSetup{
		ConfigDir: dir,
		AuthMode:  creds.Mode,
		Source:    creds.Source,
		Cleanup:   cleanup,
	}

	switch creds.Mode {
	case AuthModeToken:
		tokenPath := filepath.Join(dir, accessTokenFilename)
		if err := writeSecretFile(tokenPath, []byte(creds.Token), deps); err != nil {
			_ = cleanup()
			return RuntimeSetup{}, err
		}
		setup.Env = []string{
			envKaggleConfigDir + "=" + dir,
			envKaggleAPIToken + "=" + creds.Token,
		}
	case AuthModeLegacy:
		payload, err := json.Marshal(struct {
			Username string `json:"username"`
			Key      string `json:"key"`
		}{
			Username: creds.Username,
			Key:      creds.Key,
		})
		if err != nil {
			_ = cleanup()
			return RuntimeSetup{}, &RuntimeSetupError{Op: "marshal", Path: filepath.Join(dir, kaggleJSONFilename), Err: err}
		}
		configPath := filepath.Join(dir, kaggleJSONFilename)
		if err := writeSecretFile(configPath, payload, deps); err != nil {
			_ = cleanup()
			return RuntimeSetup{}, err
		}
		setup.Env = []string{
			envKaggleConfigDir + "=" + dir,
			envKaggleUsername + "=" + creds.Username,
			envKaggleKey + "=" + creds.Key,
		}
	default:
		_ = cleanup()
		return RuntimeSetup{}, &CredentialValidationError{Problem: "unsupported auth mode"}
	}

	return setup, nil
}

func resolveConfigDirs(env EnvSource, deps credentialResolverDeps) ([]string, error) {
	if value, ok := env.LookupEnv(envKaggleConfigDir); ok {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, &CredentialValidationError{
				Fields: []string{envKaggleConfigDir},
				Source: CredentialSourceFileLegacy,
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

func writeSecretFile(path string, content []byte, deps runtimeSetupDeps) error {
	if err := deps.writeFile(path, content, 0o600); err != nil {
		return &RuntimeSetupError{Op: "write", Path: path, Err: err}
	}
	if err := deps.chmod(path, 0o600); err != nil {
		return &RuntimeSetupError{Op: "chmod", Path: path, Err: err}
	}
	return nil
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
