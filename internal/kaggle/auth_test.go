package kaggle

import (
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

func TestResolveCredentialsFromEnvToken(t *testing.T) {
	t.Parallel()

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleAPIToken: "token-value",
	}, credentialResolverDeps{})
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

func TestResolveCredentialsFromEnvLegacy(t *testing.T) {
	t.Parallel()

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleUsername: "alice",
		envKaggleKey:      "secret-key",
	}, credentialResolverDeps{})
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
		t.Fatalf("unexpected legacy credentials %#v", creds.Diagnostics())
	}
}

func TestResolveCredentialsFromTokenFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, accessTokenFilename)
	if err := os.WriteFile(path, []byte("token-from-file\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/unused", nil },
		goos:     "darwin",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Mode != AuthModeToken {
		t.Fatalf("unexpected mode %q", creds.Mode)
	}
	if creds.Source != CredentialSourceFileToken {
		t.Fatalf("unexpected source %q", creds.Source)
	}
	if creds.Path != path {
		t.Fatalf("unexpected path %q", creds.Path)
	}
}

func TestResolveCredentialsFromLegacyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, kaggleJSONFilename)
	if err := os.WriteFile(path, []byte(`{"username":"alice","key":"secret-key"}`), 0o644); err != nil {
		t.Fatalf("write kaggle.json: %v", err)
	}

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/unused", nil },
		goos:     "darwin",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Mode != AuthModeLegacy {
		t.Fatalf("unexpected mode %q", creds.Mode)
	}
	if creds.Source != CredentialSourceFileLegacy {
		t.Fatalf("unexpected source %q", creds.Source)
	}
	if creds.Path != path {
		t.Fatalf("unexpected path %q", creds.Path)
	}
}

func TestResolveCredentialsHonorsConfigDirOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, kaggleJSONFilename)
	if err := os.WriteFile(path, []byte(`{"username":"alice","key":"secret-key"}`), 0o644); err != nil {
		t.Fatalf("write kaggle.json: %v", err)
	}

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return filepath.Join(t.TempDir(), "home"), nil },
		goos:     "linux",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Path != path {
		t.Fatalf("expected override path %q, got %q", path, creds.Path)
	}
}

func TestResolveCredentialsFallsBackToXDGOnLinux(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	configDir := filepath.Join(xdg, "kaggle")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	path := filepath.Join(configDir, kaggleJSONFilename)
	if err := os.WriteFile(path, []byte(`{"username":"alice","key":"secret-key"}`), 0o644); err != nil {
		t.Fatalf("write kaggle.json: %v", err)
	}

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envXDGConfigHome: xdg,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return home, nil },
		goos:     "linux",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Path != path {
		t.Fatalf("expected XDG path %q, got %q", path, creds.Path)
	}
}

func TestResolveCredentialsPartialEnvFailsWithoutFallingThrough(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, kaggleJSONFilename)
	if err := os.WriteFile(path, []byte(`{"username":"alice","key":"secret-key"}`), 0o644); err != nil {
		t.Fatalf("write kaggle.json: %v", err)
	}

	_, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleUsername:  "alice",
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/unused", nil },
		goos:     "darwin",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var validationErr *CredentialValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected CredentialValidationError, got %T", err)
	}
	if validationErr.Source != CredentialSourceEnvLegacy {
		t.Fatalf("unexpected source %q", validationErr.Source)
	}
	if !strings.Contains(err.Error(), envKaggleKey) {
		t.Fatalf("expected missing key in error, got %q", err.Error())
	}
}

func TestResolveCredentialsRejectsMalformedLegacyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, kaggleJSONFilename)
	if err := os.WriteFile(path, []byte(`{bad json}`), 0o644); err != nil {
		t.Fatalf("write malformed kaggle.json: %v", err)
	}

	_, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/unused", nil },
		goos:     "darwin",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected path in error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("expected redacted error, got %q", err.Error())
	}
}

func TestResolveCredentialsRejectsIncompleteLegacyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, kaggleJSONFilename), []byte(`{"username":"alice"}`), 0o644); err != nil {
		t.Fatalf("write incomplete kaggle.json: %v", err)
	}

	_, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/unused", nil },
		goos:     "darwin",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Fatalf("expected missing key in error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "alice") {
		t.Fatalf("expected error to redact credential value, got %q", err.Error())
	}
}

func TestResolveCredentialsRejectsEmptyTokenFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, accessTokenFilename), []byte(" \n"), 0o644); err != nil {
		t.Fatalf("write empty token file: %v", err)
	}

	_, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleConfigDir: dir,
	}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/unused", nil },
		goos:     "darwin",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), accessTokenFilename) {
		t.Fatalf("expected token filename in error, got %q", err.Error())
	}
}

func TestResolveCredentialsDiagnosticsAreSafe(t *testing.T) {
	t.Parallel()

	creds, err := resolveCredentialsWithDeps(staticEnvSource{
		envKaggleAPIToken: "secret-token",
	}, credentialResolverDeps{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	diag := creds.Diagnostics()
	if diag.Mode != AuthModeToken || diag.Source != CredentialSourceEnvToken {
		t.Fatalf("unexpected diagnostics %#v", diag)
	}
	if strings.Contains(diag.Path, "secret-token") {
		t.Fatalf("unexpected secret in diagnostics %#v", diag)
	}
}

func TestResolveCredentialsMissingReportsExpectedSources(t *testing.T) {
	t.Parallel()

	_, err := resolveCredentialsWithDeps(staticEnvSource{}, credentialResolverDeps{
		readFile: os.ReadFile,
		stat:     os.Stat,
		homeDir:  func() (string, error) { return "/tmp/home", nil },
		goos:     "darwin",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var missingErr *MissingCredentialsError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected MissingCredentialsError, got %T", err)
	}
	if got := err.Error(); !strings.Contains(got, envKaggleAPIToken) || !strings.Contains(got, envKaggleUsername) || !strings.Contains(got, envKaggleKey) {
		t.Fatalf("expected env vars in error, got %q", got)
	}
}
