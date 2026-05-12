package github

import (
	"os"

	"github.com/shotomorisk/kgh/internal/execution"
	"github.com/shotomorisk/kgh/internal/reporting"
)

// SummaryWriter appends execution summaries to the GitHub step summary file.
type SummaryWriter struct {
	Getenv     func(string) string
	AppendFile func(path string, body []byte) error
}

func NewSummaryWriter() SummaryWriter {
	return SummaryWriter{
		Getenv:     os.Getenv,
		AppendFile: appendFile,
	}
}

func (w SummaryWriter) Write(result execution.Result, failure *execution.FailureSummary) error {
	if w.Getenv == nil {
		w.Getenv = os.Getenv
	}
	if w.AppendFile == nil {
		w.AppendFile = appendFile
	}

	path := w.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return nil
	}
	return w.AppendFile(path, []byte(reporting.RenderGitHubSummary(result, reporting.GitHubSummaryOptions{
		Failure: failure,
	})))
}

func (w SummaryWriter) WriteExecutionSummary(result execution.Result, failure *execution.FailureSummary) error {
	return w.Write(result, failure)
}

func appendFile(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(body)
	return err
}
