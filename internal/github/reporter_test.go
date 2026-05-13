package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/spec"
)

type fakeCommentWriter struct {
	calls int
	body  string
	err   error
}

type fakeReportContextResolver struct {
	ctx ReportContext
	err error
}

func (f *fakeCommentWriter) Write(_ context.Context, _ ReportContext, body string) error {
	f.calls++
	f.body = body
	return f.err
}

func (f fakeReportContextResolver) Resolve() (ReportContext, error) {
	return f.ctx, f.err
}

func TestRunReporterWritesPullRequestComment(t *testing.T) {
	t.Parallel()

	commentWriter := &fakeCommentWriter{}
	reporter := RunReporter{
		SummaryWriter: SummaryWriter{
			Getenv: func(string) string { return "" },
		},
		ContextResolver: fakeReportContextResolver{
			ctx: ReportContext{
				PullRequestNumber: 17,
				RunURL:            "https://github.com/shotomorisk/kgh/actions/runs/42",
			},
		},
		CommentWriter: commentWriter,
	}

	err := reporter.WriteExecutionReport(context.Background(), execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/exp142.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if commentWriter.calls != 1 {
		t.Fatalf("expected 1 comment write, got %d", commentWriter.calls)
	}
	if !strings.Contains(commentWriter.body, commentMarker) {
		t.Fatalf("expected comment marker, got %q", commentWriter.body)
	}
	if !strings.Contains(commentWriter.body, "[workflow run](https://github.com/shotomorisk/kgh/actions/runs/42)") {
		t.Fatalf("expected run URL in body, got %q", commentWriter.body)
	}
}

func TestRunReporterSkipsCommentOutsidePullRequest(t *testing.T) {
	t.Parallel()

	commentWriter := &fakeCommentWriter{}
	reporter := RunReporter{
		SummaryWriter: SummaryWriter{
			Getenv: func(string) string { return "" },
		},
		ContextResolver: fakeReportContextResolver{
			ctx: ReportContext{EventName: "push"},
		},
		CommentWriter: commentWriter,
	}

	err := reporter.WriteExecutionReport(context.Background(), execution.Result{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if commentWriter.calls != 0 {
		t.Fatalf("expected no comment writes, got %d", commentWriter.calls)
	}
}

func TestRunReporterAggregatesErrors(t *testing.T) {
	t.Parallel()

	reporter := RunReporter{
		SummaryWriter: SummaryWriter{
			Getenv:     func(string) string { return "/tmp/summary" },
			AppendFile: func(string, []byte) error { return errors.New("disk full") },
		},
		ContextResolver: fakeReportContextResolver{
			err: fmt.Errorf("bad event payload"),
		},
		CommentWriter: &fakeCommentWriter{},
	}

	err := reporter.WriteExecutionReport(context.Background(), execution.Result{}, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "write GitHub summary: disk full") {
		t.Fatalf("expected summary error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "resolve GitHub reporting context: bad event payload") {
		t.Fatalf("expected context error, got %q", err.Error())
	}
}
