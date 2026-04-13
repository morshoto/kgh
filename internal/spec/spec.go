package spec

import "github.com/shotomorisk/kgh/internal/config"

// RuntimeOverrides contains the subset of trigger-level fields that can override a target.
type RuntimeOverrides struct {
	GPU      *bool `json:"gpu,omitempty" yaml:"gpu,omitempty"`
	Internet *bool `json:"internet,omitempty" yaml:"internet,omitempty"`
}

// ExecutionSpec is the fully resolved target configuration used by later execution layers.
type ExecutionSpec struct {
	TargetName  string           `json:"target_name" yaml:"target_name"`
	Notebook    string           `json:"notebook" yaml:"notebook"`
	KernelID    string           `json:"kernel_id" yaml:"kernel_id"`
	Competition string           `json:"competition" yaml:"competition"`
	Submit      bool             `json:"submit" yaml:"submit"`
	Resources   config.Resources `json:"resources" yaml:"resources"`
	Sources     config.Sources   `json:"sources" yaml:"sources"`
	Outputs     config.Outputs   `json:"outputs" yaml:"outputs"`
	Overrides   RuntimeOverrides `json:"overrides,omitempty" yaml:"overrides,omitempty"`
}

// NewExecutionSpec merges a target definition with supported runtime overrides.
func NewExecutionSpec(name string, target config.Target, overrides RuntimeOverrides) ExecutionSpec {
	resources := target.Resources
	if overrides.GPU != nil {
		resources.GPU = *overrides.GPU
	}
	if overrides.Internet != nil {
		resources.Internet = *overrides.Internet
	}

	return ExecutionSpec{
		TargetName:  name,
		Notebook:    target.Notebook,
		KernelID:    target.KernelID,
		Competition: target.Competition,
		Submit:      target.Submit,
		Resources:   resources,
		Sources:     target.Sources,
		Outputs:     target.Outputs,
		Overrides:   overrides,
	}
}
