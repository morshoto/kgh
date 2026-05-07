package reporting

import (
	"strings"
	"testing"
	"time"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/kaggle"
	"github.com/shotomorisk/kgh/internal/spec"
)

func TestRenderGitHubSummaryDryRun(t *testing.T) {
	t.Parallel()

	got := RenderGitHubSummary(execution.Result{
		Mode:   execution.ModeDryRun,
		DryRun: true,
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/exp142.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
		},
	})

	assertContains(t, got, "## kgh run summary")
	assertContains(t, got, "| Target | `exp142` |")
	assertContains(t, got, "| Notebook Path | `notebooks/exp142.ipynb` |")
	assertContains(t, got, "| Kernel ID | `yourname/exp142` |")
	assertContains(t, got, "| Run Status | dry-run |")
	assertContains(t, got, "| Submit Status | dry-run |")
	assertContains(t, got, "| Public Score | unavailable |")
	assertContains(t, got, "kernel: `yourname/exp142`")
	assertContains(t, got, "competition: `playground-series-s6e2`")
}

func TestRenderGitHubSummaryLiveSuccess(t *testing.T) {
	t.Parallel()

	got := RenderGitHubSummary(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/config.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
		},
		Bundle: &execution.BundleResult{
			NotebookPath: "/tmp/bundle/notebooks/exp142.ipynb",
		},
		Push: &execution.PushResult{
			KernelRef: "yourname/exp142",
		},
		Poll: &execution.PollResult{
			Status:   "complete",
			Terminal: kaggle.KernelPollTerminalStateSucceeded,
		},
		Submission: &execution.SubmissionResult{
			Submitted:    true,
			SubmissionID: "123",
			Status:       "complete",
		},
		Score: &execution.ScoreResult{
			State:       execution.ScoreStateReady,
			PublicScore: "0.12345",
		},
		Outputs: &execution.OutputsResult{
			Submission: execution.OutputFileResult{Path: "/tmp/output/submission.csv"},
		},
	})

	assertContains(t, got, "| Notebook Path | `/tmp/bundle/notebooks/exp142.ipynb` |")
	assertContains(t, got, "| Run Status | succeeded |")
	assertContains(t, got, "| Submit Status | submitted |")
	assertContains(t, got, "| Public Score | 0.12345 |")
	assertContains(t, got, "submission: `123`")
}

func TestRenderGitHubPRComment(t *testing.T) {
	t.Parallel()

	got := RenderGitHubPRComment(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/exp142.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
			Resources: config.Resources{
				GPU:      true,
				Internet: false,
			},
		},
		Submission: &execution.SubmissionResult{
			Submitted:    true,
			SubmissionID: "123",
			Status:       "complete",
		},
		Score: &execution.ScoreResult{
			State:       execution.ScoreStateReady,
			PublicScore: "0.12345",
		},
	}, GitHubCommentOptions{
		RunURL: "https://github.com/shotomorisk/kgh/actions/runs/42",
	})

	assertContains(t, got, "<!-- kgh:run-report -->")
	assertContains(t, got, "## kgh run report")
	assertContains(t, got, "| Submission Result | submitted<br>status: complete<br>id: `123` |")
	assertContains(t, got, "### Resolved Configuration")
	assertContains(t, got, "| Kernel Ref | `yourname/exp142` |")
	assertContains(t, got, "[workflow run](https://github.com/shotomorisk/kgh/actions/runs/42)")
}

func TestRenderGitHubSummaryPendingScore(t *testing.T) {
	t.Parallel()

	got := RenderGitHubSummary(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName: "exp142",
			Submit:     true,
		},
		Push: &execution.PushResult{KernelRef: "yourname/exp142"},
		Poll: &execution.PollResult{
			Status:    "running",
			Terminal:  kaggle.KernelPollTerminalStateUnknown,
			StartedAt: time.Unix(0, 0),
		},
		Submission: &execution.SubmissionResult{
			Submitted: true,
		},
		Score: &execution.ScoreResult{
			State: execution.ScoreStatePending,
		},
	})

	assertContains(t, got, "| Run Status | running |")
	assertContains(t, got, "| Submit Status | submission metadata unavailable |")
	assertContains(t, got, "| Public Score | pending |")
}

func TestRenderGitHubSummarySubmissionDisabled(t *testing.T) {
	t.Parallel()

	got := RenderGitHubSummary(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName: "exp142",
			Submit:     false,
		},
	})

	assertContains(t, got, "| Submit Status | disabled |")
	assertContains(t, got, "| Public Score | unavailable |")
}

func TestRenderGitHubSummaryPartialResultFallbacks(t *testing.T) {
	t.Parallel()

	got := RenderGitHubSummary(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName: "exp142",
			Submit:     true,
		},
		Push: &execution.PushResult{},
	})

	assertContains(t, got, "| Notebook Path | unavailable |")
	assertContains(t, got, "| Kernel ID | unavailable |")
	assertContains(t, got, "| Run Status | unavailable |")
	assertContains(t, got, "| References | unavailable |")
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, got)
	}
}
