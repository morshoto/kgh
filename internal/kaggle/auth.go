package kaggle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envKaggleUsername  = "KAGGLE_USERNAME"
	envKaggleKey       = "KAGGLE_KEY"
	envKaggleAPIToken  = "KAGGLE_API_TOKEN"
	envKaggleConfigDir = "KAGGLE_CONFIG_DIR"
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
	CredentialSourceEnvLegacy CredentialSource = "env-legacy"
	CredentialSourceEnvToken  CredentialSource = "env-token"
)

// Credentials holds the Kaggle auth material resolved from environment input.
type Credentials struct {
	Mode   AuthMode
	Source CredentialSource

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
}

func (e *MissingCredentialsError) Error() string {
	if e == nil || len(e.Missing) == 0 {
		return "missing Kaggle credentials"
	}
	return fmt.Sprintf("missing Kaggle credentials: %s", strings.Join(e.Missing, ", "))
}

type CredentialValidationError struct {
	Fields  []string
	Problem string
}

func (e *CredentialValidationError) Error() string {
	if e == nil {
		return "invalid Kaggle credentials"
	}
	if len(e.Fields) > 0 {
		return fmt.Sprintf("invalid Kaggle credentials: %s", strings.Join(e.Fields, ", "))
	}
	if e.Problem != "" {
		return fmt.Sprintf("invalid Kaggle credentials: %s", e.Problem)
	}
	return "invalid Kaggle credentials"
}

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

// LoadCredentials reads Kaggle credentials from the provided environment source.
func LoadCredentials(env EnvSource) (Credentials, error) {
	return ResolveCredentials(env)
}

// ResolveCredentials validates Kaggle credentials sourced from the process environment.
func ResolveCredentials(env EnvSource) (Credentials, error) {
	if env == nil {
		return Credentials{}, &MissingCredentialsError{Missing: []string{envKaggleAPIToken, envKaggleUsername, envKaggleKey}}
	}

	if token, ok := env.LookupEnv(envKaggleAPIToken); ok {
		token = strings.TrimSpace(token)
		if token == "" {
			return Credentials{}, &CredentialValidationError{Fields: []string{envKaggleAPIToken}}
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
			return Credentials{}, &CredentialValidationError{Fields: invalid}
		}
		return Credentials{
			Mode:     AuthModeLegacy,
			Source:   CredentialSourceEnvLegacy,
			Username: username,
			Key:      key,
		}, nil
	}

	return Credentials{}, &MissingCredentialsError{Missing: []string{envKaggleAPIToken, envKaggleUsername, envKaggleKey}}
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

func writeSecretFile(path string, content []byte, deps runtimeSetupDeps) error {
	if err := deps.writeFile(path, content, 0o600); err != nil {
		return &RuntimeSetupError{Op: "write", Path: path, Err: err}
	}
	if err := deps.chmod(path, 0o600); err != nil {
		return &RuntimeSetupError{Op: "chmod", Path: path, Err: err}
	}
	return nil
}
