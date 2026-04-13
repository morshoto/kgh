package kaggle

import (
	"fmt"
	"strings"
)

const (
	envKaggleUsername = "KAGGLE_USERNAME"
	envKaggleKey      = "KAGGLE_KEY"
)

// Credentials holds the Kaggle CLI credentials sourced from the environment.
type Credentials struct {
	Username string
	Key      string
}

// EnvSource reads environment variables by name.
type EnvSource interface {
	LookupEnv(string) (string, bool)
}

// MissingCredentialsError reports one or more required Kaggle environment variables.
type MissingCredentialsError struct {
	Missing []string
}

func (e *MissingCredentialsError) Error() string {
	if e == nil || len(e.Missing) == 0 {
		return "missing Kaggle credentials"
	}
	return fmt.Sprintf("missing Kaggle credentials: %s", strings.Join(e.Missing, ", "))
}

// LoadCredentials reads Kaggle credentials from the provided environment source.
func LoadCredentials(env EnvSource) (Credentials, error) {
	username, hasUsername := env.LookupEnv(envKaggleUsername)
	key, hasKey := env.LookupEnv(envKaggleKey)

	var missing []string
	if !hasUsername || username == "" {
		missing = append(missing, envKaggleUsername)
	}
	if !hasKey || key == "" {
		missing = append(missing, envKaggleKey)
	}
	if len(missing) > 0 {
		return Credentials{}, &MissingCredentialsError{Missing: missing}
	}

	return Credentials{
		Username: username,
		Key:      key,
	}, nil
}
