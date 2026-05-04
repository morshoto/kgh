package reporting

import (
	"fmt"
	"strings"

	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/kaggle"
)

const (
	stateDryRun                        = "dry-run"
	stateDisabled                      = "disabled"
	statePending                       = "pending"
	stateUnavailable                   = "unavailable"
	stateNotSubmitted                  = "not submitted"
	stateSubmissionMetadataUnavailable = "submission metadata unavailable"
)

// RenderGitHubSummary renders an execution result into Markdown for GitHub step summaries.
func RenderGitHubSummary(result execution.Result) string {
	rows := []summaryRow{
		{Field: "Target", Value: formatIdentifier(valueOr(result.Execution.TargetName, stateUnavailable))},
		{Field: "Notebook Path", Value: formatIdentifier(resolveNotebookPath(result))},
		{Field: "Kernel ID", Value: formatIdentifier(resolveKernelID(result))},
		{Field: "Run Status", Value: resolveRunStatus(result)},
		{Field: "Submit Status", Value: resolveSubmitStatus(result)},
		{Field: "Public Score", Value: resolvePublicScore(result)},
		{Field: "References", Value: formatReferences(result)},
	}

	var b strings.Builder
	b.WriteString("## kgh run summary\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("| --- | --- |\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s | %s |\n", escapeMarkdown(row.Field), row.Value)
	}
	return b.String()
}

type summaryRow struct {
	Field string
	Value string
}

func resolveNotebookPath(result execution.Result) string {
	if result.Bundle != nil && strings.TrimSpace(result.Bundle.NotebookPath) != "" {
		return result.Bundle.NotebookPath
	}
	if strings.TrimSpace(result.Execution.Notebook) != "" {
		return result.Execution.Notebook
	}
	return stateUnavailable
}

func resolveKernelID(result execution.Result) string {
	if strings.TrimSpace(result.Execution.KernelID) != "" {
		return result.Execution.KernelID
	}
	if result.Push != nil && strings.TrimSpace(result.Push.KernelRef) != "" {
		return result.Push.KernelRef
	}
	return stateUnavailable
}

func resolveRunStatus(result execution.Result) string {
	if result.Mode == execution.ModeDryRun {
		return stateDryRun
	}
	if result.Poll != nil {
		switch result.Poll.Terminal {
		case kaggle.KernelPollTerminalStateSucceeded:
			return string(kaggle.KernelPollTerminalStateSucceeded)
		case kaggle.KernelPollTerminalStateFailed:
			return string(kaggle.KernelPollTerminalStateFailed)
		case kaggle.KernelPollTerminalStateCancelled:
			return string(kaggle.KernelPollTerminalStateCancelled)
		}
		if status := normalizeStatus(result.Poll.Status); status != "" {
			return status
		}
	}
	return stateUnavailable
}

func resolveSubmitStatus(result execution.Result) string {
	if result.Mode == execution.ModeDryRun {
		return stateDryRun
	}
	if !result.Execution.Submit {
		return stateDisabled
	}
	if result.Submission == nil {
		return stateNotSubmitted
	}
	if result.Submission.Submitted && (strings.TrimSpace(result.Submission.SubmissionID) == "" || strings.TrimSpace(result.Submission.Status) == "") {
		return stateSubmissionMetadataUnavailable
	}
	if result.Submission.Submitted {
		return "submitted"
	}
	return stateNotSubmitted
}

func resolvePublicScore(result execution.Result) string {
	if result.Score != nil && strings.TrimSpace(result.Score.PublicScore) != "" {
		return result.Score.PublicScore
	}
	if result.Score != nil && result.Score.State == execution.ScoreStatePending {
		return statePending
	}
	return stateUnavailable
}

func formatReferences(result execution.Result) string {
	parts := make([]string, 0, 3)
	if kernelRef := strings.TrimSpace(resolveKernelRef(result)); kernelRef != "" {
		parts = append(parts, fmt.Sprintf("kernel: %s", formatIdentifier(kernelRef)))
	}
	if submissionID := strings.TrimSpace(resolveSubmissionID(result)); submissionID != "" {
		parts = append(parts, fmt.Sprintf("submission: %s", formatIdentifier(submissionID)))
	}
	if competition := strings.TrimSpace(result.Execution.Competition); competition != "" {
		parts = append(parts, fmt.Sprintf("competition: %s", formatIdentifier(competition)))
	}
	if len(parts) == 0 {
		return stateUnavailable
	}
	return strings.Join(parts, "<br>")
}

func resolveKernelRef(result execution.Result) string {
	if result.Push != nil && strings.TrimSpace(result.Push.KernelRef) != "" {
		return result.Push.KernelRef
	}
	if strings.TrimSpace(result.Execution.KernelRef) != "" {
		return result.Execution.KernelRef
	}
	return ""
}

func resolveSubmissionID(result execution.Result) string {
	if result.Submission != nil && strings.TrimSpace(result.Submission.SubmissionID) != "" {
		return result.Submission.SubmissionID
	}
	return ""
}

func formatIdentifier(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "", stateDryRun, stateDisabled, statePending, stateUnavailable, stateNotSubmitted, stateSubmissionMetadataUnavailable, "submitted":
		if value == "" {
			return stateUnavailable
		}
		return value
	default:
		return "`" + escapeMarkdown(value) + "`"
	}
}

func normalizeStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return value
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func escapeMarkdown(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}
