package parser

import (
	"fmt"
	"strings"
)

// Trigger is the parsed commit-message command for a single kgh run.
type Trigger struct {
	Command  string
	Target   string
	GPU      *bool
	Internet *bool
}

// ParseCommitMessage scans a full commit message and extracts a single submit trigger.
func ParseCommitMessage(message string) (Trigger, error) {
	var triggerLine string

	for _, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "submit:") {
			continue
		}
		if triggerLine != "" {
			return Trigger{}, fmt.Errorf("multiple submit commands found")
		}
		triggerLine = trimmed
	}

	if triggerLine == "" {
		return Trigger{}, fmt.Errorf("no submit command found")
	}

	return parseTriggerLine(triggerLine)
}

func parseTriggerLine(line string) (Trigger, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != "submit:" {
		return Trigger{}, fmt.Errorf("invalid submit syntax: expected 'submit:' followed by target")
	}
	if len(fields) == 1 || strings.Contains(fields[1], "=") {
		return Trigger{}, fmt.Errorf("missing target after 'submit:'")
	}

	trigger := Trigger{
		Command: "submit",
		Target:  fields[1],
	}

	for _, token := range fields[2:] {
		key, value, ok := strings.Cut(token, "=")
		if !ok || key == "" || value == "" {
			return Trigger{}, fmt.Errorf("invalid override format %q: expected key=value", token)
		}

		parsed, err := parseBoolOverride(key, value)
		if err != nil {
			return Trigger{}, err
		}

		switch key {
		case "gpu":
			if trigger.GPU != nil {
				return Trigger{}, fmt.Errorf("duplicate override key %q", key)
			}
			trigger.GPU = parsed
		case "internet":
			if trigger.Internet != nil {
				return Trigger{}, fmt.Errorf("duplicate override key %q", key)
			}
			trigger.Internet = parsed
		default:
			return Trigger{}, fmt.Errorf("unsupported override key %q", key)
		}
	}

	return trigger, nil
}

func parseBoolOverride(key, value string) (*bool, error) {
	switch value {
	case "true":
		parsed := true
		return &parsed, nil
	case "false":
		parsed := false
		return &parsed, nil
	default:
		return nil, fmt.Errorf("invalid boolean value for %q: expected true or false, got %q", key, value)
	}
}
