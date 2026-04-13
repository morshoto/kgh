package kaggle

import (
	"context"
	"errors"
	"testing"
)

func TestStubAdapterImplementsAdapter(t *testing.T) {
	t.Parallel()

	var adapter Adapter = StubAdapter{}
	if adapter == nil {
		t.Fatal("expected adapter to be non-nil")
	}
}

func TestStubAdapterReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	adapter := StubAdapter{}
	ctx := context.Background()

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "push kernel",
			run: func() error {
				_, err := adapter.PushKernel(ctx, PushKernelRequest{})
				return err
			},
		},
		{
			name: "kernel status",
			run: func() error {
				_, err := adapter.KernelStatus(ctx, KernelStatusRequest{})
				return err
			},
		},
		{
			name: "download output",
			run: func() error {
				_, err := adapter.DownloadKernelOutput(ctx, DownloadKernelOutputRequest{})
				return err
			},
		},
		{
			name: "submit competition",
			run: func() error {
				_, err := adapter.SubmitCompetition(ctx, CompetitionSubmitRequest{})
				return err
			},
		},
		{
			name: "list submissions",
			run: func() error {
				_, err := adapter.ListCompetitionSubmissions(ctx, CompetitionSubmissionsRequest{})
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if !errors.Is(err, ErrNotImplemented) {
				t.Fatalf("expected ErrNotImplemented, got %v", err)
			}
		})
	}
}
