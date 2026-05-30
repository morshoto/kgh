package github

import (
	"context"
	"errors"
	"fmt"

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
	SummaryWriter    SummaryWriter
	ContextResolver  reportContextResolver
	CommentWriter    commentWriter
	BaselineResolver baselineResolver
}

func NewRunReporter() RunReporter {
	return RunReporter{
		SummaryWriter:    NewSummaryWriter(),
		ContextResolver:  NewReportContextResolver(),
		CommentWriter:    NewPRCommentWriter(),
		BaselineResolver: NewBaselineResolver(),
	}
}

func (r RunReporter) PrepareExecutionReport(ctx context.Context, result execution.Result) (execution.Result, error) {
	return r.enrichComparison(ctx, result)
}

func (r RunReporter) WriteExecutionReport(ctx context.Context, result execution.Result, failure *execution.FailureSummary) error {
	var errs []error

	enrichedResult, err := r.enrichComparison(ctx, result)
	if err == nil {
		result = enrichedResult
	}

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
		Comparison: result.Comparison,
	})); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (r RunReporter) enrichComparison(ctx context.Context, result execution.Result) (execution.Result, error) {
	if result.Comparison != nil {
		return result, nil
	}

	reportCtx, err := r.ContextResolver.Resolve()
	if err != nil || !reportCtx.HasPullRequest() {
		return result, nil
	}

	comparison, err := r.BaselineResolver.Resolve(ctx, reportCtx, result)
	if err != nil || comparison == nil {
		return result, nil
	}
	result.Comparison = comparison
	return result, nil
}
