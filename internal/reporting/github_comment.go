package reporting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shotomorisk/kgh/internal/execution"
)

// GitHubCommentComparison is an optional comparison block for GitHub report surfaces.
type GitHubCommentComparison struct {
	Label         string
	BaselineScore string
	CurrentScore  string
	Delta         string
}

// GitHubCommentOptions configures the GitHub PR comment renderer.
type GitHubCommentOptions struct {
	RunURL     string
	Comparison *GitHubCommentComparison
}

// RenderGitHubPRComment renders a stable Markdown PR comment for a run result.
func RenderGitHubPRComment(result execution.Result, opts GitHubCommentOptions) string {
	var b strings.Builder
	comparison := opts.Comparison
	if comparison == nil {
		comparison = &GitHubCommentComparison{}
	}

	b.WriteString("<!-- kgh:run-report -->\n")
	b.WriteString("## kgh run report\n")
	b.WriteString("\n### Overview\n")
	b.WriteString("| Details | Score |\n")
	b.WriteString("| --- | --- |\n")
	writeOverviewRow(&b, renderCommentOverviewDetails(result, opts.RunURL), renderCommentScore(result, comparison))

	b.WriteString("\n### Resolved Configuration\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("| --- | --- |\n")
	writeCommentRow(&b, "Notebook", formatIdentifier(valueOr(result.Execution.Notebook, stateUnavailable)))
	writeCommentRow(&b, "Kernel ID", formatIdentifier(valueOr(result.Execution.KernelID, stateUnavailable)))
	writeCommentRow(&b, "Kernel Ref", formatIdentifier(valueOr(result.Execution.KernelRef, stateUnavailable)))
	writeCommentRow(&b, "Competition", formatIdentifier(valueOr(result.Execution.Competition, stateUnavailable)))
	writeCommentRow(&b, "Submit", strconv.FormatBool(result.Execution.Submit))
	writeCommentRow(&b, "GPU", strconv.FormatBool(result.Execution.Resources.GPU))
	writeCommentRow(&b, "Internet", strconv.FormatBool(result.Execution.Resources.Internet))
	writeCommentRow(&b, "Submission Output", formatIdentifier(valueOr(result.Execution.Outputs.Submission, stateUnavailable)))
	writeCommentRow(&b, "Metrics Output", formatIdentifier(valueOr(result.Execution.Outputs.Metrics, stateUnavailable)))

	b.WriteString("\n### Comparison\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("| --- | --- |\n")
	writeCommentRow(&b, "Baseline", formatIdentifier(valueOr(comparison.Label, stateUnavailable)))
	writeCommentRow(&b, "Baseline Score", valueOr(strings.TrimSpace(comparison.BaselineScore), stateUnavailable))
	writeCommentRow(&b, "Current Score", valueOr(strings.TrimSpace(comparison.CurrentScore), stateUnavailable))
	writeCommentRow(&b, "Delta", valueOr(strings.TrimSpace(comparison.Delta), stateUnavailable))

	return b.String()
}

func writeCommentRow(b *strings.Builder, field, value string) {
	fmt.Fprintf(b, "| %s | %s |\n", escapeMarkdown(field), value)
}

func writeOverviewRow(b *strings.Builder, details, score string) {
	fmt.Fprintf(b, "| %s | %s |\n", details, score)
}

func renderCommentOverviewDetails(result execution.Result, runURL string) string {
	parts := make([]string, 0, 4)
	parts = append(parts, fmt.Sprintf("Target: %s", formatIdentifier(valueOr(result.Execution.TargetName, stateUnavailable))))
	parts = append(parts, fmt.Sprintf("Run Status: %s", resolveRunStatus(result)))
	parts = append(parts, fmt.Sprintf("Submission Result: %s", resolveSubmissionResult(result)))
	parts = append(parts, fmt.Sprintf("References: %s", formatCommentReferences(result, runURL)))
	return strings.Join(parts, "<br>")
}

func renderCommentScore(result execution.Result, comparison *GitHubCommentComparison) string {
	if comparison != nil && strings.TrimSpace(comparison.CurrentScore) != "" {
		return fmt.Sprintf("Public Score: %s", strings.TrimSpace(comparison.CurrentScore))
	}
	return fmt.Sprintf("Public Score: %s", resolvePublicScore(result))
}

func resolveSubmissionResult(result execution.Result) string {
	if result.Mode == execution.ModeDryRun {
		return stateDryRun
	}
	if !result.Execution.Submit {
		return stateDisabled
	}
	if result.Submission == nil {
		return stateNotSubmitted
	}

	parts := make([]string, 0, 3)
	switch {
	case result.Submission.Submitted:
		parts = append(parts, "submitted")
	case result.Submission.Attempted:
		parts = append(parts, "attempted")
	default:
		parts = append(parts, stateNotSubmitted)
	}
	if status := strings.TrimSpace(result.Submission.Status); status != "" {
		parts = append(parts, "status: "+escapeMarkdown(status))
	}
	if submissionID := strings.TrimSpace(result.Submission.SubmissionID); submissionID != "" {
		parts = append(parts, "id: "+formatIdentifier(submissionID))
	}
	if message := strings.TrimSpace(result.Submission.Message); message != "" {
		parts = append(parts, "message: "+escapeMarkdown(message))
	}
	return strings.Join(parts, "<br>")
}

func formatCommentReferences(result execution.Result, runURL string) string {
	parts := make([]string, 0, 4)
	if kernelRef := strings.TrimSpace(resolveKernelRef(result)); kernelRef != "" {
		parts = append(parts, fmt.Sprintf("kernel: %s", formatIdentifier(kernelRef)))
	}
	if submissionID := strings.TrimSpace(resolveSubmissionID(result)); submissionID != "" {
		parts = append(parts, fmt.Sprintf("submission: %s", formatIdentifier(submissionID)))
	}
	if competition := strings.TrimSpace(result.Execution.Competition); competition != "" {
		parts = append(parts, fmt.Sprintf("competition: %s", formatIdentifier(competition)))
	}
	if runURL = strings.TrimSpace(runURL); runURL != "" {
		parts = append(parts, fmt.Sprintf("[workflow run](%s)", escapeMarkdown(runURL)))
	}
	if len(parts) == 0 {
		return stateUnavailable
	}
	return strings.Join(parts, "<br>")
}
