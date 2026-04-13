package kaggle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type staticEnvSource map[string]string

func (s staticEnvSource) LookupEnv(key string) (string, bool) {
	v, ok := s[key]
	return v, ok
}

func TestResolveCredentialsToken(t *testing.T) {
	t.Parallel()

	creds, err := ResolveCredentials(staticEnvSource{
		envKaggleAPIToken: "token-value",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Mode != AuthModeToken {
		t.Fatalf("unexpected mode %q", creds.Mode)
	}
	if creds.Source != CredentialSourceEnvToken {
		t.Fatalf("unexpected source %q", creds.Source)
	}
	if creds.Token != "token-value" {
		t.Fatalf("unexpected token %q", creds.Token)
	}
}

func TestResolveCredentialsLegacy(t *testing.T) {
	t.Parallel()

	creds, err := ResolveCredentials(staticEnvSource{
		envKaggleUsername: "alice",
		envKaggleKey:      "secret-key",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Mode != AuthModeLegacy {
		t.Fatalf("unexpected mode %q", creds.Mode)
	}
	if creds.Source != CredentialSourceEnvLegacy {
		t.Fatalf("unexpected source %q", creds.Source)
	}
	if creds.Username != "alice" || creds.Key != "secret-key" {
		t.Fatalf("unexpected credentials %+v", creds)
	}
}

func TestResolveCredentialsMissing(t *testing.T) {
	t.Parallel()

	_, err := ResolveCredentials(staticEnvSource{})
	if err == nil {
		t.Fatal("expected an error")
	}

	var missingErr *MissingCredentialsError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected MissingCredentialsError, got %T", err)
	}
	if got := err.Error(); !strings.Contains(got, envKaggleAPIToken) || !strings.Contains(got, envKaggleUsername) || !strings.Contains(got, envKaggleKey) {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestResolveCredentialsRejectsEmptyToken(t *testing.T) {
	t.Parallel()

	_, err := ResolveCredentials(staticEnvSource{
		envKaggleAPIToken: "   ",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), envKaggleAPIToken) {
		t.Fatalf("unexpected error %q", err.Error())
	}
}

func TestResolveCredentialsRejectsPartialLegacy(t *testing.T) {
	t.Parallel()

	_, err := ResolveCredentials(staticEnvSource{
		envKaggleUsername: "alice",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), envKaggleKey) {
		t.Fatalf("unexpected error %q", err.Error())
	}
	if strings.Contains(err.Error(), "alice") {
		t.Fatalf("expected redacted error, got %q", err.Error())
	}
}

func TestPrepareRuntimeToken(t *testing.T) {
	t.Parallel()

	setup, err := PrepareRuntime(staticEnvSource{
		envKaggleAPIToken: "token-value",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		if err := setup.Cleanup(); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}()

	if setup.AuthMode != AuthModeToken || setup.Source != CredentialSourceEnvToken {
		t.Fatalf("unexpected setup metadata %+v", setup)
	}
	assertEnvContains(t, setup.Env, envKaggleConfigDir, setup.ConfigDir)
	assertEnvContains(t, setup.Env, envKaggleAPIToken, "token-value")

	dirInfo, err := os.Stat(setup.ConfigDir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("unexpected dir mode %o", got)
	}

	tokenPath := filepath.Join(setup.ConfigDir, accessTokenFilename)
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if string(data) != "token-value" {
		t.Fatalf("unexpected token file content %q", string(data))
	}
	fileInfo, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("unexpected token file mode %o", got)
	}
}

func TestPrepareRuntimeLegacy(t *testing.T) {
	t.Parallel()

	setup, err := PrepareRuntime(staticEnvSource{
		envKaggleUsername: "alice",
		envKaggleKey:      "secret-key",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		if err := setup.Cleanup(); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}()

	assertEnvContains(t, setup.Env, envKaggleConfigDir, setup.ConfigDir)
	assertEnvContains(t, setup.Env, envKaggleUsername, "alice")
	assertEnvContains(t, setup.Env, envKaggleKey, "secret-key")

	configPath := filepath.Join(setup.ConfigDir, kaggleJSONFilename)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read kaggle.json: %v", err)
	}

	var payload struct {
		Username string `json:"username"`
		Key      string `json:"key"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal kaggle.json: %v", err)
	}
	if payload.Username != "alice" || payload.Key != "secret-key" {
		t.Fatalf("unexpected payload %+v", payload)
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat kaggle.json: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("unexpected kaggle.json mode %o", got)
	}
}

func TestPrepareRuntimeCleanupRemovesTempDir(t *testing.T) {
	t.Parallel()

	setup, err := PrepareRuntime(staticEnvSource{
		envKaggleAPIToken: "token-value",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := setup.Cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := os.Stat(setup.ConfigDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config dir removal, got %v", err)
	}
}

func TestPrepareRuntimeFailsBeforeFilesystemWrite(t *testing.T) {
	t.Parallel()

	wrote := false
	_, err := prepareRuntimeWithDeps(staticEnvSource{
		envKaggleUsername: "alice",
	}, runtimeSetupDeps{
		mkdirTemp: func(string, string) (string, error) {
			t.Fatal("did not expect temp dir creation")
			return "", nil
		},
		writeFile: func(string, []byte, os.FileMode) error {
			wrote = true
			return nil
		},
		chmod:     func(string, os.FileMode) error { return nil },
		removeAll: func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if wrote {
		t.Fatal("did not expect filesystem write")
	}
}

func TestPrepareRuntimeWriteFailureRedactsSecrets(t *testing.T) {
	t.Parallel()

	_, err := prepareRuntimeWithDeps(staticEnvSource{
		envKaggleAPIToken: "secret-token",
	}, runtimeSetupDeps{
		mkdirTemp: func(string, string) (string, error) { return "/tmp/runtime", nil },
		writeFile: func(string, []byte, os.FileMode) error { return errors.New("disk full") },
		chmod:     func(string, os.FileMode) error { return nil },
		removeAll: func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("expected redacted error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), accessTokenFilename) {
		t.Fatalf("expected safe path context, got %q", err.Error())
	}
}

func assertEnvContains(t *testing.T, env []string, key, value string) {
	t.Helper()

	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			got := strings.TrimPrefix(entry, prefix)
			if got != value {
				t.Fatalf("unexpected %s value %q", key, got)
			}
			return
		}
	}
	t.Fatalf("missing %s in env %#v", key, env)
}
