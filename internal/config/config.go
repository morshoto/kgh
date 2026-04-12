package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultPath = ".kgh/config.yaml"

type Config struct {
	Targets map[string]Target `yaml:"targets"`
}

type Target struct {
	Notebook    string    `yaml:"notebook"`
	KernelID    string    `yaml:"kernel_id"`
	Competition string    `yaml:"competition"`
	Submit      bool      `yaml:"submit"`
	Resources   Resources `yaml:"resources"`
	Sources     Sources   `yaml:"sources"`
	Outputs     Outputs   `yaml:"outputs"`
}

type Resources struct {
	GPU      bool `yaml:"gpu"`
	Internet bool `yaml:"internet"`
	Private  bool `yaml:"private"`
}

type Sources struct {
	CompetitionSources []string `yaml:"competition_sources"`
	DatasetSources     []string `yaml:"dataset_sources"`
}

type Outputs struct {
	Submission string `yaml:"submission"`
	Metrics    string `yaml:"metrics"`
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type Error struct {
	Path  string
	Issue string
	Err   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Path, e.Issue, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Issue)
}

func (e *Error) Unwrap() error { return e.Err }

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, &Error{Path: path, Issue: "read config", Err: err}
	}

	return Parse(path, b)
}

func Parse(path string, data []byte) (Config, error) {
	var raw any
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return Config{}, &Error{Path: path, Issue: "parse yaml", Err: err}
	}

	node := mapNode(raw)
	if node == nil {
		return Config{}, &Error{Path: path, Issue: "invalid config", Err: errors.New("expected mapping at document root")}
	}

	cfg := Config{Targets: map[string]Target{}}
	targets, ok := node["targets"]
	if !ok {
		return Config{}, &Error{Path: path, Issue: "validate config", Err: ValidationError{Field: "targets", Message: "required"}}
	}
	targetMap, ok := targets.(map[string]any)
	if !ok {
		return Config{}, &Error{Path: path, Issue: "validate config", Err: ValidationError{Field: "targets", Message: "must be a mapping"}}
	}

	var errs []error
	for name, value := range targetMap {
		tm, ok := value.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Field: "targets." + name, Message: "must be a mapping"})
			continue
		}

		target, err := parseTarget(name, tm)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cfg.Targets[name] = target
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, &Error{Path: path, Issue: "validate config", Err: err}
	}

	return cfg, nil
}

func parseTarget(name string, m map[string]any) (Target, error) {
	t := Target{}
	var errs []error

	t.Notebook = stringValue(m, "notebook")
	t.KernelID = stringValue(m, "kernel_id")
	t.Competition = stringValue(m, "competition")
	t.Submit = boolValue(m, "submit")

	if t.Notebook == "" {
		errs = append(errs, ValidationError{Field: "targets." + name + ".notebook", Message: "required"})
	}
	if t.KernelID == "" {
		errs = append(errs, ValidationError{Field: "targets." + name + ".kernel_id", Message: "required"})
	}
	if t.Competition == "" {
		errs = append(errs, ValidationError{Field: "targets." + name + ".competition", Message: "required"})
	}

	if v, ok := m["resources"]; ok {
		rm, ok := v.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Field: "targets." + name + ".resources", Message: "must be a mapping"})
		} else {
			t.Resources = Resources{
				GPU:      boolValue(rm, "gpu"),
				Internet: boolValue(rm, "internet"),
				Private:  boolValue(rm, "private"),
			}
		}
	}

	if v, ok := m["sources"]; ok {
		sm, ok := v.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Field: "targets." + name + ".sources", Message: "must be a mapping"})
		} else {
			t.Sources = Sources{
				CompetitionSources: stringSliceValue(sm, "competition_sources"),
				DatasetSources:     stringSliceValue(sm, "dataset_sources"),
			}
		}
	}

	if v, ok := m["outputs"]; ok {
		om, ok := v.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Field: "targets." + name + ".outputs", Message: "must be a mapping"})
		} else {
			t.Outputs = Outputs{
				Submission: stringValue(om, "submission"),
				Metrics:    stringValue(om, "metrics"),
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		return Target{}, err
	}
	return t, nil
}

func stringValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func boolValue(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	b, _ := v.(bool)
	return b
}

func stringSliceValue(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func mapNode(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		return x
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			out[ks] = normalize(val)
		}
		return out
	default:
		return nil
	}
}

func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalize(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			out[ks] = normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalize(val)
		}
		return out
	default:
		return v
	}
}

func ConfigPath(root string) string {
	return filepath.Join(root, DefaultPath)
}

func TargetNames(cfg Config) []string {
	names := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
