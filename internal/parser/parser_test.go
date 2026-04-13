package parser

import (
	"strings"
	"testing"
)

func TestParseCommitMessageValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		message  string
		target   string
		gpu      *bool
		internet *bool
	}{
		{
			name:    "target only",
			message: "submit: exp142",
			target:  "exp142",
		},
		{
			name:    "gpu override",
			message: "submit: exp142 gpu=false",
			target:  "exp142",
			gpu:     boolPtr(false),
		},
		{
			name:     "internet override",
			message:  "submit: exp142 internet=true",
			target:   "exp142",
			internet: boolPtr(true),
		},
		{
			name:     "both overrides",
			message:  "submit: exp142 gpu=false internet=true",
			target:   "exp142",
			gpu:      boolPtr(false),
			internet: boolPtr(true),
		},
		{
			name:     "both overrides reversed",
			message:  "submit: exp142 internet=true gpu=false",
			target:   "exp142",
			gpu:      boolPtr(false),
			internet: boolPtr(true),
		},
		{
			name: "multiline commit message",
			message: strings.Join([]string{
				"feat: tune notebook thresholds",
				"",
				"Run the latest experiment on Kaggle.",
				"submit: exp142 gpu=false",
			}, "\n"),
			target: "exp142",
			gpu:    boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseCommitMessage(tt.message)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got.Command != "submit" {
				t.Fatalf("expected command submit, got %q", got.Command)
			}
			if got.Target != tt.target {
				t.Fatalf("expected target %q, got %q", tt.target, got.Target)
			}
			assertOptionalBool(t, "gpu", got.GPU, tt.gpu)
			assertOptionalBool(t, "internet", got.Internet, tt.internet)
		})
	}
}

func TestParseCommitMessageInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		wantErr string
	}{
		{
			name:    "no trigger",
			message: "feat: tune notebook thresholds",
			wantErr: "no submit command found",
		},
		{
			name:    "missing target",
			message: "submit:",
			wantErr: "missing target after 'submit:'",
		},
		{
			name:    "missing target before override",
			message: "submit: gpu=false",
			wantErr: "missing target after 'submit:'",
		},
		{
			name:    "multiple targets",
			message: "submit: exp142 exp143",
			wantErr: "invalid override format \"exp143\": expected key=value",
		},
		{
			name:    "malformed override",
			message: "submit: exp142 gpu",
			wantErr: "invalid override format \"gpu\": expected key=value",
		},
		{
			name:    "unknown override",
			message: "submit: exp142 private=true",
			wantErr: "unsupported override key \"private\"",
		},
		{
			name:    "duplicate gpu override",
			message: "submit: exp142 gpu=false gpu=true",
			wantErr: "duplicate override key \"gpu\"",
		},
		{
			name:    "duplicate internet override",
			message: "submit: exp142 internet=false internet=true",
			wantErr: "duplicate override key \"internet\"",
		},
		{
			name:    "invalid boolean",
			message: "submit: exp142 gpu=TRUE",
			wantErr: "invalid boolean value for \"gpu\": expected true or false, got \"TRUE\"",
		},
		{
			name: "multiple submit commands",
			message: strings.Join([]string{
				"submit: exp142",
				"",
				"submit: exp143",
			}, "\n"),
			wantErr: "multiple submit commands found",
		},
		{
			name:    "missing space after command",
			message: "submit:exp142",
			wantErr: "invalid submit syntax: expected 'submit:' followed by target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseCommitMessage(tt.message)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, got)
			}
		})
	}
}

func assertOptionalBool(t *testing.T, name string, got, want *bool) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("expected %s %v, got %v", name, valueOrNil(want), valueOrNil(got))
	case *got != *want:
		t.Fatalf("expected %s %t, got %t", name, *want, *got)
	}
}

func valueOrNil(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}

func boolPtr(v bool) *bool {
	return &v
}
