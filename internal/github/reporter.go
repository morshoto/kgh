package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/reporting"
)

type commentWriter interface {
	Write(context.Context, ReportContext, string) error
}

type reportContextResolver interface {
	Resolve() (ReportContext, error)
}

// RunReporter publishes execution results to GitHub-facing surfaces.
type RunReporter struct {
	SummaryWriter   SummaryWriter
	ContextResolver reportContextResolver
	CommentWriter   commentWriter
}

func NewRunReporter() RunReporter {
	return RunReporter{
		SummaryWriter:   NewSummaryWriter(),
		ContextResolver: NewReportContextResolver(),
		CommentWriter:   NewPRCommentWriter(),
	}
}

func (r RunReporter) WriteExecutionReport(ctx context.Context, result execution.Result, failure *execution.FailureSummary) error {
	var errs []error

	if err := r.SummaryWriter.WriteExecutionSummary(result, failure); err != nil {
		errs = append(errs, fmt.Errorf("write GitHub summary: %w", err))
	}

	reportCtx, err := r.ContextResolver.Resolve()
	if err != nil {
		if len(errs) == 0 {
			return fmt.Errorf("resolve GitHub reporting context: %w", err)
		}
		errs = append(errs, fmt.Errorf("resolve GitHub reporting context: %w", err))
		return errors.Join(errs...)
	}
	if !reportCtx.HasPullRequest() {
		return errors.Join(errs...)
	}

	if err := r.CommentWriter.Write(ctx, reportCtx, reporting.RenderGitHubPRComment(result, reporting.GitHubCommentOptions{
		RunURL:     reportCtx.RunURL,
		Comparison: buildGitHubCommentComparison(result),
	})); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func buildGitHubCommentComparison(result execution.Result) *reporting.GitHubCommentComparison {
	if result.Score == nil {
		return nil
	}

	comparison := &reporting.GitHubCommentComparison{
		Label: "latest base branch run",
	}
	if currentScore := strings.TrimSpace(result.Score.PublicScore); currentScore != "" {
		comparison.CurrentScore = currentScore
	}
	return comparison
}
