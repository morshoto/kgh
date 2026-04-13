package kaggle

import (
	"strings"
	"testing"
)

type staticEnvSource map[string]string

func (s staticEnvSource) LookupEnv(key string) (string, bool) {
	v, ok := s[key]
	return v, ok
}

func TestLoadCredentials(t *testing.T) {
	t.Parallel()

	creds, err := LoadCredentials(staticEnvSource{
		envKaggleUsername: "alice",
		envKaggleKey:      "secret-key",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if creds.Username != "alice" {
		t.Fatalf("unexpected username %q", creds.Username)
	}
	if creds.Key != "secret-key" {
		t.Fatalf("unexpected key %q", creds.Key)
	}
}

func TestLoadCredentialsMissingUsername(t *testing.T) {
	t.Parallel()

	_, err := LoadCredentials(staticEnvSource{
		envKaggleKey: "secret-key",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var missingErr *MissingCredentialsError
	if !strings.Contains(err.Error(), envKaggleUsername) {
		t.Fatalf("expected missing username in error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("expected error to redact credential value, got %q", err.Error())
	}
	if !errorAs(err, &missingErr) {
		t.Fatalf("expected MissingCredentialsError, got %T", err)
	}
	if got := len(missingErr.Missing); got != 1 || missingErr.Missing[0] != envKaggleUsername {
		t.Fatalf("unexpected missing variables %#v", missingErr.Missing)
	}
}

func TestLoadCredentialsMissingKey(t *testing.T) {
	t.Parallel()

	_, err := LoadCredentials(staticEnvSource{
		envKaggleUsername: "alice",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), envKaggleKey) {
		t.Fatalf("expected missing key in error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "alice") {
		t.Fatalf("expected error to redact credential value, got %q", err.Error())
	}
}

func TestLoadCredentialsMissingBoth(t *testing.T) {
	t.Parallel()

	_, err := LoadCredentials(staticEnvSource{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, envKaggleUsername) || !strings.Contains(got, envKaggleKey) {
		t.Fatalf("expected both missing variables in error, got %q", got)
	}
}

func TestLoadCredentialsEmptyValuesAreMissing(t *testing.T) {
	t.Parallel()

	_, err := LoadCredentials(staticEnvSource{
		envKaggleUsername: "",
		envKaggleKey:      "",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, envKaggleUsername) || !strings.Contains(got, envKaggleKey) {
		t.Fatalf("expected both empty variables to be reported, got %q", got)
	}
}

func errorAs(err error, target any) bool {
	switch t := target.(type) {
	case **MissingCredentialsError:
		typed, ok := err.(*MissingCredentialsError)
		if !ok {
			return false
		}
		*t = typed
		return true
	default:
		return false
	}
}
