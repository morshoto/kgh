package kaggle

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

type operationLogger interface {
	InfoContext(context.Context, string, ...any)
	ErrorContext(context.Context, string, ...any)
}

func newNoopLogger() operationLogger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func operationName(args []string) string {
	if len(args) == 0 {
		return "command"
	}
	if len(args) == 1 {
		return args[0]
	}
	return args[0] + " " + args[1]
}

func sanitizeArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	safe := make([]string, len(args))
	copy(safe, args)
	return safe
}

func sanitizeEnv(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	safe := make([]string, 0, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			safe = append(safe, entry)
			continue
		}
		if isSensitiveEnvKey(key) {
			safe = append(safe, key+"=[REDACTED]")
			continue
		}
		safe = append(safe, key+"="+value)
	}
	return safe
}

func isSensitiveEnvKey(key string) bool {
	switch key {
	case envKaggleUsername, envKaggleKey, envKaggleAPIToken:
		return true
	default:
		return false
	}
}
