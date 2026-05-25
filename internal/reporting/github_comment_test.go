package reporting

import (
	"testing"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/spec"
)

func TestRenderGitHubPRCommentShowsPlaceholderComparisonSectionWhenComparisonMissing(t *testing.T) {
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

	assertContains(t, got, "### Comparison")
	assertContains(t, got, "| Baseline | unavailable |")
	assertContains(t, got, "| Baseline Score | unavailable |")
	assertContains(t, got, "| Current Score | unavailable |")
	assertContains(t, got, "| Delta | unavailable |")
}

func TestRenderGitHubPRCommentRendersComparisonCurrentScore(t *testing.T) {
	t.Parallel()

	got := RenderGitHubPRComment(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/exp142.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
		},
		Score: &execution.ScoreResult{
			State:       execution.ScoreStateReady,
			PublicScore: "0.12345",
		},
	}, GitHubCommentOptions{
		Comparison: &GitHubCommentComparison{
			Label:        "latest base branch run",
			CurrentScore: "0.12345",
		},
	})

	assertContains(t, got, "### Comparison")
	assertContains(t, got, "| Baseline | `latest base branch run` |")
	assertContains(t, got, "| Current Score | 0.12345 |")
}

func TestRenderGitHubPRCommentCoversDryRunVariant(t *testing.T) {
	t.Parallel()

	got := RenderGitHubPRComment(execution.Result{
		Mode: execution.ModeDryRun,
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/exp142.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
		},
	}, GitHubCommentOptions{})

	assertContains(t, got, "| Run Status | dry-run |")
	assertContains(t, got, "| Submission Result | dry-run |")
	assertContains(t, got, "| Public Score | unavailable |")
	assertContains(t, got, "### Resolved Configuration")
}

func TestRenderGitHubPRCommentCoversSubmittedVariant(t *testing.T) {
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
	}, GitHubCommentOptions{})

	assertContains(t, got, "| Submission Result | submitted<br>status: complete<br>id: `123` |")
	assertContains(t, got, "| Public Score | 0.12345 |")
	assertContains(t, got, "| Kernel Ref | `yourname/exp142` |")
}

func TestRenderGitHubPRCommentCoversDisabledSubmitVariant(t *testing.T) {
	t.Parallel()

	got := RenderGitHubPRComment(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName: "exp142",
			Submit:     false,
		},
	}, GitHubCommentOptions{})

	assertContains(t, got, "| Submit | false |")
	assertContains(t, got, "| Submission Result | disabled |")
	assertContains(t, got, "| Public Score | unavailable |")
}

func TestRenderGitHubPRCommentRendersWorkflowRunLink(t *testing.T) {
	t.Parallel()

	got := RenderGitHubPRComment(execution.Result{
		Execution: spec.ExecutionSpec{
			TargetName:  "exp142",
			Notebook:    "notebooks/exp142.ipynb",
			KernelID:    "yourname/exp142",
			KernelRef:   "yourname/exp142",
			Competition: "playground-series-s6e2",
			Submit:      true,
		},
		Submission: &execution.SubmissionResult{
			Submitted:    true,
			SubmissionID: "123",
			Status:       "complete",
		},
	}, GitHubCommentOptions{
		RunURL: "https://github.com/shotomorisk/kgh/actions/runs/42",
	})

	assertContains(t, got, "[workflow run](https://github.com/shotomorisk/kgh/actions/runs/42)")
	assertContains(t, got, "kernel: `yourname/exp142`")
	assertContains(t, got, "competition: `playground-series-s6e2`")
}
