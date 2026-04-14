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

func TestNewAdapterImplementsAdapter(t *testing.T) {
	t.Parallel()

	var adapter Adapter = NewAdapter(NewClient())
	if adapter == nil {
		t.Fatal("expected adapter to be non-nil")
	}
}

func TestCLIAdapterPushKernel(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, args []string, opts RunOptions) (Result, error) {
			if !equalStrings(args, []string{"kernels", "push", "-p", "/tmp/work tree"}) {
				t.Fatalf("unexpected args %#v", args)
			}
			assertZeroRunOptions(t, opts)
			return Result{}, nil
		},
	}

	resp, err := (&CLIAdapter{client: fake}).PushKernel(context.Background(), PushKernelRequest{
		WorkDir: "/tmp/work tree",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.KernelRef != "" {
		t.Fatalf("expected empty kernel ref, got %q", resp.KernelRef)
	}
}

func TestCLIAdapterKernelStatus(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, args []string, opts RunOptions) (Result, error) {
			if !equalStrings(args, []string{"kernels", "status", "-p", "alice/exp142"}) {
				t.Fatalf("unexpected args %#v", args)
			}
			assertZeroRunOptions(t, opts)
			return Result{Stdout: "status: complete\nmessage: finished\n"}, nil
		},
	}

	resp, err := (&CLIAdapter{client: fake}).KernelStatus(context.Background(), KernelStatusRequest{
		KernelRef: "alice/exp142",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.KernelRef != "alice/exp142" {
		t.Fatalf("unexpected kernel ref %q", resp.KernelRef)
	}
	if resp.Status != "complete" || resp.Message != "finished" {
		t.Fatalf("unexpected status response %+v", resp)
	}
}

func TestCLIAdapterDownloadKernelOutput(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, args []string, opts RunOptions) (Result, error) {
			if !equalStrings(args, []string{"kernels", "output", "alice/exp142", "-p", "/tmp/output dir"}) {
				t.Fatalf("unexpected args %#v", args)
			}
			assertZeroRunOptions(t, opts)
			return Result{}, nil
		},
	}

	resp, err := (&CLIAdapter{client: fake}).DownloadKernelOutput(context.Background(), DownloadKernelOutputRequest{
		KernelRef: "alice/exp142",
		OutputDir: "/tmp/output dir",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.OutputDir != "/tmp/output dir" {
		t.Fatalf("unexpected output dir %q", resp.OutputDir)
	}
}

func TestCLIAdapterSubmitCompetition(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, args []string, opts RunOptions) (Result, error) {
			want := []string{"competitions", "submit", "-c", "playground-series-s6e2", "-f", "/tmp/submission csv", "-m", "submit from PR #12"}
			if !equalStrings(args, want) {
				t.Fatalf("unexpected args %#v", args)
			}
			assertZeroRunOptions(t, opts)
			return Result{}, nil
		},
	}

	resp, err := (&CLIAdapter{client: fake}).SubmitCompetition(context.Background(), CompetitionSubmitRequest{
		Competition: "playground-series-s6e2",
		FilePath:    "/tmp/submission csv",
		Message:     "submit from PR #12",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Competition != "playground-series-s6e2" || !resp.Submitted {
		t.Fatalf("unexpected response %+v", resp)
	}
}

func TestCLIAdapterListCompetitionSubmissions(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, args []string, opts RunOptions) (Result, error) {
			if !equalStrings(args, []string{"competitions", "submissions", "-c", "playground-series-s6e2"}) {
				t.Fatalf("unexpected args %#v", args)
			}
			assertZeroRunOptions(t, opts)
			return Result{Stdout: "file,description,date,status,publicScore\nsubmission.csv,submit from PR #12,2026-04-14T10:00:00Z,complete,0.12345\n"}, nil
		},
	}

	resp, err := (&CLIAdapter{client: fake}).ListCompetitionSubmissions(context.Background(), CompetitionSubmissionsRequest{
		Competition: "playground-series-s6e2",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Submissions) != 1 {
		t.Fatalf("expected one submission, got %+v", resp.Submissions)
	}
	if resp.Submissions[0].FileName != "submission.csv" || resp.Submissions[0].PublicScore != "0.12345" {
		t.Fatalf("unexpected submission %+v", resp.Submissions[0])
	}
}

func TestCLIAdapterPropagatesClientErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, args []string, opts RunOptions) (Result, error) {
			if !equalStrings(args, []string{"kernels", "status", "-p", "alice/exp142"}) {
				t.Fatalf("unexpected args %#v", args)
			}
			assertZeroRunOptions(t, opts)
			return Result{}, wantErr
		},
	}

	_, err := (&CLIAdapter{client: fake}).KernelStatus(context.Background(), KernelStatusRequest{
		KernelRef: "alice/exp142",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryCommandFailed {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

func TestCLIAdapterNormalizesMissingCredentials(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, _ []string, _ RunOptions) (Result, error) {
			return Result{}, &MissingCredentialsError{Missing: []string{envKaggleUsername}}
		},
	}

	_, err := (&CLIAdapter{client: fake}).PushKernel(context.Background(), PushKernelRequest{WorkDir: "/tmp/work"})
	if err == nil {
		t.Fatal("expected an error")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryMissingCredentials {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

func TestCLIAdapterNormalizesPermissionDenied(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, _ []string, _ RunOptions) (Result, error) {
			return Result{}, &CommandError{
				ExitCode: 1,
				Stderr:   "403 Forbidden: you must accept the competition rules",
				Err:      errors.New("exit status 1"),
			}
		},
	}

	_, err := (&CLIAdapter{client: fake}).SubmitCompetition(context.Background(), CompetitionSubmitRequest{
		Competition: "playground-series-s6e2",
		FilePath:    "/tmp/submission.csv",
		Message:     "submit",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryPermissionDenied {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
	if adapterErr.Stderr == "" {
		t.Fatal("expected stderr to be preserved")
	}
}

func TestCLIAdapterNormalizesInvalidReference(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, _ []string, _ RunOptions) (Result, error) {
			return Result{}, &CommandError{
				ExitCode: 1,
				Stderr:   "404 Not Found: invalid competition slug",
				Err:      errors.New("exit status 1"),
			}
		},
	}

	_, err := (&CLIAdapter{client: fake}).ListCompetitionSubmissions(context.Background(), CompetitionSubmissionsRequest{
		Competition: "missing-comp",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryInvalidReference {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

func TestCLIAdapterReportsUnexpectedKernelStatusOutput(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, _ []string, _ RunOptions) (Result, error) {
			return Result{Stdout: "kernel is doing something\n"}, nil
		},
	}

	_, err := (&CLIAdapter{client: fake}).KernelStatus(context.Background(), KernelStatusRequest{
		KernelRef: "alice/exp142",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryUnexpectedOutput {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

func TestCLIAdapterReportsUnexpectedSubmissionsOutput(t *testing.T) {
	t.Parallel()

	fake := &adapterFakeClient{
		t: t,
		runFn: func(_ context.Context, _ []string, _ RunOptions) (Result, error) {
			return Result{Stdout: "submission.csv only\n"}, nil
		},
	}

	_, err := (&CLIAdapter{client: fake}).ListCompetitionSubmissions(context.Background(), CompetitionSubmissionsRequest{
		Competition: "playground-series-s6e2",
	})
	if err == nil {
		t.Fatal("expected an error")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryUnexpectedOutput {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

func TestCLIAdapterFailsWithNilClient(t *testing.T) {
	t.Parallel()

	_, err := (&CLIAdapter{}).PushKernel(context.Background(), PushKernelRequest{
		WorkDir: "/tmp/work",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "kaggle adapter client is nil" {
		t.Fatalf("unexpected error %q", err)
	}
}

func TestBuildPushKernelCommand(t *testing.T) {
	t.Parallel()

	got, err := buildPushKernelCommand(PushKernelRequest{WorkDir: "/tmp/work tree"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !equalStrings(got, []string{"kernels", "push", "-p", "/tmp/work tree"}) {
		t.Fatalf("unexpected args %#v", got)
	}
}

func TestBuildKernelStatusCommand(t *testing.T) {
	t.Parallel()

	got, err := buildKernelStatusCommand(KernelStatusRequest{KernelRef: "alice/exp142"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !equalStrings(got, []string{"kernels", "status", "-p", "alice/exp142"}) {
		t.Fatalf("unexpected args %#v", got)
	}
}

func TestBuildDownloadKernelOutputCommand(t *testing.T) {
	t.Parallel()

	got, err := buildDownloadKernelOutputCommand(DownloadKernelOutputRequest{
		KernelRef: "alice/exp142",
		OutputDir: "/tmp/output dir",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !equalStrings(got, []string{"kernels", "output", "alice/exp142", "-p", "/tmp/output dir"}) {
		t.Fatalf("unexpected args %#v", got)
	}
}

func TestBuildCompetitionSubmitCommand(t *testing.T) {
	t.Parallel()

	got, err := buildCompetitionSubmitCommand(CompetitionSubmitRequest{
		Competition: "playground-series-s6e2",
		FilePath:    "/tmp/submission csv",
		Message:     "submit from PR #12",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	want := []string{"competitions", "submit", "-c", "playground-series-s6e2", "-f", "/tmp/submission csv", "-m", "submit from PR #12"}
	if !equalStrings(got, want) {
		t.Fatalf("unexpected args %#v", got)
	}
}

func TestBuildCompetitionSubmissionsCommand(t *testing.T) {
	t.Parallel()

	got, err := buildCompetitionSubmissionsCommand(CompetitionSubmissionsRequest{
		Competition: "playground-series-s6e2",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !equalStrings(got, []string{"competitions", "submissions", "-c", "playground-series-s6e2"}) {
		t.Fatalf("unexpected args %#v", got)
	}
}

func TestBuildCommandsValidateRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func() error
		wantErr string
	}{
		{
			name: "push kernel work dir",
			run: func() error {
				_, err := buildPushKernelCommand(PushKernelRequest{})
				return err
			},
			wantErr: "work dir is required",
		},
		{
			name: "kernel status ref",
			run: func() error {
				_, err := buildKernelStatusCommand(KernelStatusRequest{})
				return err
			},
			wantErr: "kernel ref is required",
		},
		{
			name: "download output dir",
			run: func() error {
				_, err := buildDownloadKernelOutputCommand(DownloadKernelOutputRequest{KernelRef: "alice/exp142"})
				return err
			},
			wantErr: "output dir is required",
		},
		{
			name: "competition",
			run: func() error {
				_, err := buildCompetitionSubmitCommand(CompetitionSubmitRequest{
					FilePath: "/tmp/submission.csv",
					Message:  "submit",
				})
				return err
			},
			wantErr: "competition is required",
		},
		{
			name: "file path",
			run: func() error {
				_, err := buildCompetitionSubmitCommand(CompetitionSubmitRequest{
					Competition: "playground-series-s6e2",
					Message:     "submit",
				})
				return err
			},
			wantErr: "file path is required",
		},
		{
			name: "message",
			run: func() error {
				_, err := buildCompetitionSubmitCommand(CompetitionSubmitRequest{
					Competition: "playground-series-s6e2",
					FilePath:    "/tmp/submission.csv",
				})
				return err
			},
			wantErr: "message is required",
		},
		{
			name: "competition submissions competition",
			run: func() error {
				_, err := buildCompetitionSubmissionsCommand(CompetitionSubmissionsRequest{})
				return err
			},
			wantErr: "competition is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if err == nil {
				t.Fatal("expected an error")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestBuildCompetitionSubmissionsCommandRejectsLimit(t *testing.T) {
	t.Parallel()

	_, err := buildCompetitionSubmissionsCommand(CompetitionSubmissionsRequest{
		Competition: "playground-series-s6e2",
		Limit:       5,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrUnsupportedRequest) {
		t.Fatalf("expected ErrUnsupportedRequest, got %v", err)
	}
	if err.Error() != "kaggle adapter request not supported: competition submissions limit" {
		t.Fatalf("unexpected error %q", err.Error())
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

type adapterFakeClient struct {
	t     *testing.T
	runFn func(context.Context, []string, RunOptions) (Result, error)
}

func (f *adapterFakeClient) Run(ctx context.Context, args []string, opts RunOptions) (Result, error) {
	if f.runFn == nil {
		f.t.Fatal("runFn must be set")
	}
	return f.runFn(ctx, args, opts)
}

func assertZeroRunOptions(t *testing.T, opts RunOptions) {
	t.Helper()

	if opts.Dir != "" {
		t.Fatalf("unexpected dir %q", opts.Dir)
	}
	if len(opts.Env) != 0 {
		t.Fatalf("unexpected env %#v", opts.Env)
	}
	if opts.Timeout != 0 {
		t.Fatalf("unexpected timeout %s", opts.Timeout)
	}
	if opts.Debug {
		t.Fatal("unexpected debug flag")
	}
}
