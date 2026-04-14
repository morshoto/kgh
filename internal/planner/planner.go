package planner

import (
	"fmt"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/parser"
	"github.com/shotomorisk/kgh/internal/spec"
)

// Resolve turns a parsed trigger and repository config into a fully resolved execution spec.
func Resolve(cfg config.Config, trigger parser.Trigger) (spec.ExecutionSpec, error) {
	target, ok := cfg.Targets[trigger.Target]
	if !ok {
		return spec.ExecutionSpec{}, fmt.Errorf("unknown target %q", trigger.Target)
	}

	exec, err := spec.NewExecutionSpec(trigger.Target, target, spec.RuntimeOverrides{
		GPU:      trigger.GPU,
		Internet: trigger.Internet,
	})
	if err != nil {
		return spec.ExecutionSpec{}, fmt.Errorf("resolve target %q: %w", trigger.Target, err)
	}

	return exec, nil
}
