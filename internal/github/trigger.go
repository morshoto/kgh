package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/shotomorisk/kgh/internal/execx"
	"github.com/shotomorisk/kgh/internal/parser"
)

// TriggerResolver resolves a single submit trigger from GitHub Actions context.
type TriggerResolver struct {
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
	Runner   execx.Runner
}

// NewTriggerResolver constructs a production resolver backed by process env, file reads, and git.
func NewTriggerResolver() TriggerResolver {
	return TriggerResolver{
		Getenv:   os.Getenv,
		ReadFile: os.ReadFile,
		Runner:   execx.NewRunner(),
	}
}

// Resolve reads GitHub Actions metadata, loads the selected commit message, and parses one trigger.
func (r TriggerResolver) Resolve(ctx context.Context) (parser.Trigger, error) {
	if r.Getenv == nil {
		r.Getenv = os.Getenv
	}
	if r.ReadFile == nil {
		r.ReadFile = os.ReadFile
	}
	if r.Runner == nil {
		r.Runner = execx.NewRunner()
	}

	eventName := strings.TrimSpace(r.Getenv("GITHUB_EVENT_NAME"))
	if eventName == "" {
		return parser.Trigger{}, fmt.Errorf("GITHUB_EVENT_NAME is required")
	}

	sha, err := r.resolveSHA(eventName)
	if err != nil {
		return parser.Trigger{}, err
	}

	message, err := r.readCommitMessage(ctx, sha)
	if err != nil {
		return parser.Trigger{}, err
	}

	trigger, err := parser.ParseCommitMessage(message)
	if err != nil {
		return parser.Trigger{}, fmt.Errorf("parse commit message for %s: %w", sha, err)
	}

	return trigger, nil
}

func (r TriggerResolver) resolveSHA(eventName string) (string, error) {
	switch eventName {
	case "push":
		sha := strings.TrimSpace(r.Getenv("GITHUB_SHA"))
		if sha == "" {
			return "", fmt.Errorf("GITHUB_SHA is required for push events")
		}
		return sha, nil
	case "pull_request", "pull_request_target":
		eventPath := strings.TrimSpace(r.Getenv("GITHUB_EVENT_PATH"))
		if eventPath == "" {
			return "", fmt.Errorf("GITHUB_EVENT_PATH is required for %s events", eventName)
		}
		body, err := r.ReadFile(eventPath)
		if err != nil {
			return "", fmt.Errorf("read GitHub event payload %q: %w", eventPath, err)
		}

		var payload struct {
			PullRequest struct {
				Head struct {
					SHA string `json:"sha"`
				} `json:"head"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", fmt.Errorf("parse GitHub event payload %q: %w", eventPath, err)
		}

		sha := strings.TrimSpace(payload.PullRequest.Head.SHA)
		if sha == "" {
			return "", fmt.Errorf("pull_request.head.sha is required for %s events", eventName)
		}
		return sha, nil
	default:
		return "", fmt.Errorf("unsupported GitHub event %q", eventName)
	}
}

func (r TriggerResolver) readCommitMessage(ctx context.Context, sha string) (string, error) {
	opts := execx.Options{}
	if workspace := strings.TrimSpace(r.Getenv("GITHUB_WORKSPACE")); workspace != "" {
		opts.Dir = workspace
	}

	result, err := r.Runner.Run(ctx, execx.Command{
		Path: "git",
		Args: []string{"show", "-s", "--format=%B", "--no-patch", sha},
	}, opts)
	if err != nil {
		return "", fmt.Errorf("read commit message for %s: %w", sha, err)
	}

	return strings.TrimRight(result.Stdout, "\n"), nil
}
