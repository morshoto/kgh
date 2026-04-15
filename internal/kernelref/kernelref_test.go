package kernelref

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "raw ref",
			input: "alice/exp142",
			want:  "alice/exp142",
		},
		{
			name:  "url ref",
			input: "https://www.kaggle.com/code/alice/exp142",
			want:  "alice/exp142",
		},
		{
			name:    "empty",
			input:   " ",
			wantErr: true,
		},
		{
			name:    "invalid",
			input:   "exp142",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Normalize(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestExtractFromText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "url",
			input: "Kernel URL: https://www.kaggle.com/code/alice/exp142",
			want:  "alice/exp142",
		},
		{
			name:  "raw ref",
			input: "kernel identity: alice/exp142",
			want:  "alice/exp142",
		},
		{
			name: "prefer kaggle url over unrelated url-like tokens",
			input: "source: https://en.wikipedia.org/wiki/Foo\n" +
				"profile: https://www.kaggle.com/alice\n" +
				"Kernel URL: https://www.kaggle.com/code/alice/exp142\n",
			want: "alice/exp142",
		},
		{
			name:    "missing",
			input:   "Kernel pushed successfully",
			wantErr: true,
		},
		{
			name:    "ambiguous",
			input:   "first alice/exp142\nsecond bob/exp143",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractFromText(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
