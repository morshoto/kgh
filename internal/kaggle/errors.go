package kaggle

import (
	"errors"
	"fmt"
	"strings"
)

type ErrorCategory string

const (
	ErrorCategoryMissingCredentials ErrorCategory = "missing_credentials"
	ErrorCategoryInvalidCredentials ErrorCategory = "invalid_credentials"
	ErrorCategoryPermissionDenied   ErrorCategory = "permission_denied"
	ErrorCategoryInvalidReference   ErrorCategory = "invalid_reference"
	ErrorCategoryCommandFailed      ErrorCategory = "command_failed"
	ErrorCategoryUnexpectedOutput   ErrorCategory = "unexpected_output"
	ErrorCategoryTimeout            ErrorCategory = "timeout"
)

// AdapterError reports a normalized Kaggle adapter failure for higher layers.
type AdapterError struct {
	Operation string
	Category  ErrorCategory
	Message   string
	Stdout    string
	Stderr    string
	Err       error
}

func (e *AdapterError) Error() string {
	if e == nil {
		return "kaggle adapter error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Operation != "" && e.Category != "" {
		return fmt.Sprintf("kaggle %s failed: %s", e.Operation, e.Category)
	}
	if e.Category != "" {
		return fmt.Sprintf("kaggle adapter error: %s", e.Category)
	}
	return "kaggle adapter error"
}

func (e *AdapterError) Unwrap() error { return e.Err }

func normalizeAdapterError(operation string, err error) error {
	if err == nil {
		return nil
	}

	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		if adapterErr.Operation == "" {
			adapterErr.Operation = operation
		}
		return adapterErr
	}

	var missingErr *MissingCredentialsError
	if errors.As(err, &missingErr) {
		return &AdapterError{
			Operation: operation,
			Category:  ErrorCategoryMissingCredentials,
			Message:   fmt.Sprintf("kaggle %s failed: Kaggle credentials are missing", operation),
			Err:       err,
		}
	}

	var validationErr *CredentialValidationError
	if errors.As(err, &validationErr) {
		return &AdapterError{
			Operation: operation,
			Category:  ErrorCategoryInvalidCredentials,
			Message:   fmt.Sprintf("kaggle %s failed: Kaggle credentials are invalid", operation),
			Err:       err,
		}
	}

	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return &AdapterError{
			Operation: operation,
			Category:  ErrorCategoryTimeout,
			Message:   fmt.Sprintf("kaggle %s timed out", operation),
			Err:       err,
		}
	}

	var execErr *ExecutableNotFoundError
	if errors.As(err, &execErr) {
		return &AdapterError{
			Operation: operation,
			Category:  ErrorCategoryCommandFailed,
			Message:   fmt.Sprintf("kaggle %s failed: Kaggle CLI is not available", operation),
			Err:       err,
		}
	}

	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		category, message := classifyCommandFailure(operation, commandErr)
		return &AdapterError{
			Operation: operation,
			Category:  category,
			Message:   message,
			Stdout:    commandErr.Stdout,
			Stderr:    commandErr.Stderr,
			Err:       err,
		}
	}

	return &AdapterError{
		Operation: operation,
		Category:  ErrorCategoryCommandFailed,
		Message:   fmt.Sprintf("kaggle %s failed", operation),
		Err:       err,
	}
}

func unexpectedOutputError(operation string, result Result, problem string) error {
	message := fmt.Sprintf("kaggle %s failed: unexpected CLI output", operation)
	if strings.TrimSpace(problem) != "" {
		message += ": " + strings.TrimSpace(problem)
	}
	return &AdapterError{
		Operation: operation,
		Category:  ErrorCategoryUnexpectedOutput,
		Message:   message,
		Stdout:    result.Stdout,
		Stderr:    result.Stderr,
	}
}

func classifyCommandFailure(operation string, err *CommandError) (ErrorCategory, string) {
	stderr := strings.ToLower(strings.TrimSpace(err.Stderr))
	stdout := strings.ToLower(strings.TrimSpace(err.Stdout))
	combined := strings.TrimSpace(stderr + "\n" + stdout)

	switch {
	case containsAny(combined, "401", "unauthorized", "api credentials", "invalid credentials", "could not find kaggle.json", "credentials not found", "forbidden: you must provide a key"):
		return ErrorCategoryInvalidCredentials, fmt.Sprintf("kaggle %s failed: Kaggle credentials were rejected", operation)
	case containsAny(combined, "403", "forbidden", "permission", "not allowed", "must accept", "rules", "join the competition", "not have permission"):
		return ErrorCategoryPermissionDenied, fmt.Sprintf("kaggle %s failed: Kaggle denied permission for this operation", operation)
	case containsAny(combined, "404", "not found", "was not found", "invalid competition", "invalid kernel", "no such competition", "no such kernel"):
		return ErrorCategoryInvalidReference, fmt.Sprintf("kaggle %s failed: Kaggle reference is invalid or does not exist", operation)
	default:
		return ErrorCategoryCommandFailed, fmt.Sprintf("kaggle %s failed: Kaggle CLI command exited with an error", operation)
	}
}

func containsAny(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}
