package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shotomorisk/kgh/internal/config"
	"github.com/shotomorisk/kgh/internal/kaggle"
	"github.com/shotomorisk/kgh/internal/parser"
	"github.com/shotomorisk/kgh/internal/planner"
	"github.com/shotomorisk/kgh/internal/spec"
)

const DefaultConfigPath = config.DefaultPath

const (
	ModeDryRun = "dry-run"
	ModeLive   = "live"
)

type FailureStage string

const (
	FailureStageExecution               FailureStage = "execution"
	FailureStageConfig                  FailureStage = "config"
	FailureStageTargetResolution        FailureStage = "target-resolution"
	FailureStageGitHubTriggerResolution FailureStage = "github-trigger-resolution"
	FailureStageBundleStaging           FailureStage = "bundle-staging"
	FailureStagePush                    FailureStage = "push"
	FailureStagePoll                    FailureStage = "poll"
	FailureStageOutputDir               FailureStage = "output-dir"
	FailureStageDownloadOutput          FailureStage = "download-output"
	FailureStageOutputValidation        FailureStage = "output-validation"
	FailureStageSubmit                  FailureStage = "submit"
)

type Request struct {
	Target       string
	DryRun       bool
	GPU          *bool
	Internet     *bool
	ConfigPath   string
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type Result struct {
	Mode         string             `json:"mode"`
	DryRun       bool               `json:"dry_run"`
	ConfigPath   string             `json:"config_path"`
	PollInterval Duration           `json:"poll_interval"`
	PollTimeout  Duration           `json:"poll_timeout"`
	Execution    spec.ExecutionSpec `json:"execution"`
	Bundle       *BundleResult      `json:"bundle,omitempty"`
	Push         *PushResult        `json:"push,omitempty"`
	Poll         *PollResult        `json:"poll,omitempty"`
	Outputs      *OutputsResult     `json:"outputs,omitempty"`
	Submission   *SubmissionResult  `json:"submission,omitempty"`
	Score        *ScoreResult       `json:"score,omitempty"`
}

type BundleResult struct {
	WorkDir      string `json:"work_dir"`
	NotebookPath string `json:"notebook_path"`
	MetadataPath string `json:"metadata_path"`
}

type PushResult struct {
	KernelRef string `json:"kernel_ref"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
}

type PollResult struct {
	KernelRef  string                         `json:"kernel_ref"`
	Status     string                         `json:"status"`
	Message    string                         `json:"message,omitempty"`
	Attempts   int                            `json:"attempts"`
	Terminal   kaggle.KernelPollTerminalState `json:"terminal,omitempty"`
	StartedAt  time.Time                      `json:"started_at"`
	FinishedAt time.Time                      `json:"finished_at"`
	Elapsed    Duration                       `json:"elapsed"`
	Raw        kaggle.KernelStatusRawStatus   `json:"raw,omitempty"`
}

// OutputsResult is the stable handoff contract for downstream submit and
// reporting steps after kernel outputs have been downloaded and validated.
type OutputsResult struct {
	OutputDir      string                 `json:"output_dir"`
	SubmissionPath string                 `json:"submission_path"`
	MetricsPath    string                 `json:"metrics_path"`
	Submission     OutputFileResult       `json:"submission"`
	Metrics        OutputFileResult       `json:"metrics"`
	Validation     OutputValidationResult `json:"validation"`
}

type OutputValidationResult struct {
	Valid           bool     `json:"valid"`
	MissingRequired []string `json:"missing_required"`
	MissingOptional []string `json:"missing_optional"`
}

// OutputFileResult records the configured and resolved state of one expected
// output file so downstream consumers do not need to re-scan the output dir.
type OutputFileResult struct {
	ConfiguredPath string `json:"configured_path"`
	ExpectedPath   string `json:"expected_path"`
	Path           string `json:"path"`
	Present        bool   `json:"present"`
	Required       bool   `json:"required"`
	Error          string `json:"error"`
}

type SubmissionResult struct {
	Attempted    bool      `json:"attempted"`
	Submitted    bool      `json:"submitted"`
	Competition  string    `json:"competition"`
	FilePath     string    `json:"file_path"`
	FileName     string    `json:"file_name"`
	Message      string    `json:"message"`
	AttemptedAt  time.Time `json:"attempted_at"`
	SubmissionID string    `json:"submission_id,omitempty"`
	Status       string    `json:"status,omitempty"`
	SubmittedAt  time.Time `json:"submitted_at,omitempty"`
}

type ScoreResult struct {
	State        string    `json:"state"`
	Competition  string    `json:"competition"`
	FileName     string    `json:"file_name"`
	Message      string    `json:"message"`
	SubmissionID string    `json:"submission_id,omitempty"`
	Status       string    `json:"status,omitempty"`
	PublicScore  string    `json:"public_score,omitempty"`
	SubmittedAt  time.Time `json:"submitted_at,omitempty"`
}

const (
	ScoreStateReady    = "ready"
	ScoreStatePending  = "pending"
	ScoreStateNotFound = "not_found"
)

type Duration time.Duration

type FailureSummary struct {
	Stage FailureStage
	Error string
}

type ErrorWithResult struct {
	Result Result
	Stage  FailureStage
	Err    error
}

func (e *ErrorWithResult) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ErrorWithResult) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ResultFromError(err error) (Result, bool) {
	var reportErr *ErrorWithResult
	if !errors.As(err, &reportErr) || reportErr == nil {
		return Result{}, false
	}
	return reportErr.Result, true
}

func FailureSummaryFromError(err error) (*FailureSummary, bool) {
	var reportErr *ErrorWithResult
	if !errors.As(err, &reportErr) || reportErr == nil {
		return nil, false
	}
	if reportErr.Stage == "" || reportErr.Err == nil {
		return nil, false
	}
	return &FailureSummary{
		Stage: reportErr.Stage,
		Error: reportErr.Err.Error(),
	}, true
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

type Adapter interface {
	PushKernel(context.Context, kaggle.PushKernelRequest) (kaggle.PushKernelResponse, error)
	PollKernelStatus(context.Context, kaggle.KernelPollRequest) (kaggle.KernelPollResult, error)
	DownloadKernelOutput(context.Context, kaggle.DownloadKernelOutputRequest) (kaggle.DownloadKernelOutputResponse, error)
	SubmitCompetition(context.Context, kaggle.CompetitionSubmitRequest) (kaggle.CompetitionSubmitResponse, error)
	ListCompetitionSubmissions(context.Context, kaggle.CompetitionSubmissionsRequest) (kaggle.CompetitionSubmissionsResponse, error)
}

type Runner struct {
	loadConfig   func(string) (config.Config, error)
	adapter      Adapter
	pollTimeout  time.Duration
	pollInterval time.Duration
	now          func() time.Time
	sleep        func(context.Context, time.Duration) error
}

func NewRunner(adapter Adapter) *Runner {
	if adapter == nil {
		adapter = kaggle.NewAdapter(kaggle.NewClient()).(Adapter)
	}
	return &Runner{
		loadConfig:   config.Load,
		adapter:      adapter,
		pollTimeout:  30 * time.Minute,
		pollInterval: 5 * time.Second,
		now:          time.Now,
		sleep:        sleepContext,
	}
}

func (r *Runner) Execute(ctx context.Context, req Request) (Result, error) {
	if r == nil {
		return Result{}, fmt.Errorf("execution runner is nil")
	}
	if r.loadConfig == nil {
		r.loadConfig = config.Load
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.sleep == nil {
		r.sleep = sleepContext
	}
	if req.ConfigPath == "" {
		req.ConfigPath = DefaultConfigPath
	}

	report := baseResultFromRequest(req, r.pollInterval, r.pollTimeout)

	cfg, err := r.loadConfig(req.ConfigPath)
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageConfig, err, "load config %q: %w", req.ConfigPath)
	}

	trigger := parser.Trigger{
		Target:   req.Target,
		GPU:      req.GPU,
		Internet: req.Internet,
	}

	execSpec, err := planner.Resolve(cfg, trigger)
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageTargetResolution, err, "%w")
	}

	report.Execution = execSpec
	if !req.DryRun {
		return r.executeLive(ctx, execSpec, report)
	}

	return report, nil
}

func baseResultFromRequest(req Request, defaultPollInterval, defaultPollTimeout time.Duration) Result {
	return Result{
		Mode:         ModeDryRun,
		DryRun:       true,
		ConfigPath:   req.ConfigPath,
		PollInterval: Duration(effectivePollInterval(req.PollInterval, defaultPollInterval)),
		PollTimeout:  Duration(effectivePollTimeout(req.PollTimeout, defaultPollTimeout)),
		Execution: spec.ExecutionSpec{
			TargetName: req.Target,
		},
	}
}

func wrapErrorWithResult(report Result, stage FailureStage, err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	args = append(args, err)
	return &ErrorWithResult{
		Result: report,
		Stage:  stage,
		Err:    fmt.Errorf(format, args...),
	}
}

func (r *Runner) executeLive(ctx context.Context, execSpec spec.ExecutionSpec, report Result) (Result, error) {
	if r.adapter == nil {
		return Result{}, fmt.Errorf("live execution requires a Kaggle adapter")
	}

	report.Mode = ModeLive
	report.DryRun = false

	bundle, err := kaggle.StageKernelBundle(execSpec)
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageBundleStaging, err, "stage kaggle bundle: %w")
	}
	defer func() {
		if bundle.Cleanup != nil {
			_ = bundle.Cleanup()
		}
	}()

	report.Bundle = &BundleResult{
		WorkDir:      bundle.WorkDir,
		NotebookPath: bundle.NotebookPath,
		MetadataPath: bundle.MetadataPath,
	}

	pushResp, err := r.adapter.PushKernel(ctx, kaggle.PushKernelRequest{
		WorkDir: bundle.WorkDir,
	})
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStagePush, err, "push kaggle kernel: %w")
	}
	report.Push = &PushResult{
		KernelRef: pushResp.KernelRef,
		ExitCode:  pushResp.Output.ExitCode,
		Stdout:    pushResp.Output.Stdout,
		Stderr:    pushResp.Output.Stderr,
	}

	pollResp, err := r.adapter.PollKernelStatus(ctx, kaggle.KernelPollRequest{
		KernelRef: pushResp.KernelRef,
		Interval:  time.Duration(report.PollInterval),
		Timeout:   time.Duration(report.PollTimeout),
	})
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStagePoll, err, "poll kaggle kernel: %w")
	}
	report.Poll = &PollResult{
		KernelRef:  pollResp.KernelRef,
		Status:     pollResp.Status,
		Message:    pollResp.Message,
		Attempts:   pollResp.Attempts,
		Terminal:   pollResp.Terminal,
		StartedAt:  pollResp.StartedAt,
		FinishedAt: pollResp.FinishedAt,
		Elapsed:    Duration(pollResp.Elapsed),
		Raw:        pollResp.KernelStatusResponse.Raw,
	}

	outputDir, err := createOutputDir()
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageOutputDir, err, "create output dir: %w")
	}

	downloadResp, err := r.adapter.DownloadKernelOutput(ctx, kaggle.DownloadKernelOutputRequest{
		KernelRef: pushResp.KernelRef,
		OutputDir: outputDir,
	})
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageDownloadOutput, err, "download kaggle output: %w")
	}

	outputs, err := buildOutputsResult(execSpec, downloadResp.OutputDir)
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageDownloadOutput, err, "build output handoff: %w")
	}
	report.Outputs = &outputs

	if !execSpec.Submit {
		return report, nil
	}
	if !outputs.Submission.Present {
		return report, wrapErrorWithResult(report, FailureStageOutputValidation, errors.New(outputs.Submission.Error), "submit enabled but submission artifact is missing: %w")
	}

	submissionAttemptedAt := r.now().UTC()
	submitMessage := buildCompetitionSubmitMessage(execSpec, pushResp.KernelRef)
	submitResp, err := r.adapter.SubmitCompetition(ctx, kaggle.CompetitionSubmitRequest{
		Competition: execSpec.Competition,
		FilePath:    outputs.Submission.Path,
		Message:     submitMessage,
	})
	if err != nil {
		return report, wrapErrorWithResult(report, FailureStageSubmit, err, "submit kaggle competition: %w")
	}
	report.Submission = &SubmissionResult{
		Attempted:   true,
		Submitted:   submitResp.Submitted,
		Competition: submitResp.Competition,
		FilePath:    outputs.Submission.Path,
		FileName:    filepath.Base(outputs.Submission.Path),
		Message:     submitMessage,
		AttemptedAt: submissionAttemptedAt,
	}

	match, err := waitForRelevantSubmission(
		ctx,
		r.adapter,
		execSpec.Competition,
		*report.Submission,
		time.Duration(report.PollInterval),
		time.Duration(report.PollTimeout),
		r.now,
		r.sleep,
	)
	if err != nil {
		report.Score = unavailableScoreResult(execSpec.Competition, report.Submission)
		return report, nil
	}
	if err := applySubmissionMetadata(report.Submission, match); err != nil {
		report.Score = unavailableScoreResult(execSpec.Competition, report.Submission)
		return report, nil
	}
	report.Score = resolveScoreResult(execSpec.Competition, report.Submission, match)

	return report, nil
}

func buildCompetitionSubmitMessage(execSpec spec.ExecutionSpec, kernelRef string) string {
	return fmt.Sprintf("kgh submit target=%s kernel=%s", execSpec.TargetName, kernelRef)
}

func waitForRelevantSubmission(
	ctx context.Context,
	adapter Adapter,
	competition string,
	submission SubmissionResult,
	interval, timeout time.Duration,
	now func() time.Time,
	sleep func(context.Context, time.Duration) error,
) (kaggle.CompetitionSubmission, error) {
	if adapter == nil {
		return kaggle.CompetitionSubmission{}, fmt.Errorf("list kaggle competition submissions: adapter is nil")
	}
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = sleepContext
	}

	startedAt := now().UTC()
	deadline := time.Time{}
	if timeout > 0 {
		deadline = startedAt.Add(timeout)
	}

	for {
		if err := ctx.Err(); err != nil {
			return kaggle.CompetitionSubmission{}, err
		}
		if !deadline.IsZero() && !now().Before(deadline) {
			return kaggle.CompetitionSubmission{}, fmt.Errorf(
				"submission metadata unavailable: timed out waiting for Kaggle submission row after %s",
				timeout,
			)
		}

		submissionsResp, err := adapter.ListCompetitionSubmissions(ctx, kaggle.CompetitionSubmissionsRequest{
			Competition: competition,
		})
		if err != nil {
			return kaggle.CompetitionSubmission{}, fmt.Errorf("list kaggle competition submissions: %w", err)
		}
		match, ok := findRelevantSubmission(submission, submissionsResp.Submissions)
		if ok {
			return match, nil
		}

		delay := interval
		if delay < 0 {
			delay = 0
		}
		if !deadline.IsZero() {
			remaining := deadline.Sub(now())
			if remaining < delay {
				delay = remaining
			}
			if delay <= 0 {
				return kaggle.CompetitionSubmission{}, fmt.Errorf(
					"submission metadata unavailable: timed out waiting for Kaggle submission row after %s",
					timeout,
				)
			}
		}
		if err := sleep(ctx, delay); err != nil {
			return kaggle.CompetitionSubmission{}, err
		}
	}
}

func resolveScoreResult(competition string, submission *SubmissionResult, match kaggle.CompetitionSubmission) *ScoreResult {
	if submission == nil {
		return nil
	}

	score := &ScoreResult{
		State:        ScoreStateNotFound,
		Competition:  competition,
		FileName:     submission.FileName,
		Message:      submission.Message,
		SubmissionID: submission.SubmissionID,
	}
	score.Status = strings.TrimSpace(match.Status)
	score.PublicScore = strings.TrimSpace(match.PublicScore)
	score.SubmittedAt = match.SubmittedAt
	if score.PublicScore != "" {
		score.State = ScoreStateReady
		return score
	}

	score.State = ScoreStatePending
	return score
}

func unavailableScoreResult(competition string, submission *SubmissionResult) *ScoreResult {
	if submission == nil {
		return nil
	}

	return &ScoreResult{
		State:        ScoreStateNotFound,
		Competition:  competition,
		FileName:     submission.FileName,
		Message:      submission.Message,
		SubmissionID: submission.SubmissionID,
	}
}

func findRelevantSubmission(submission SubmissionResult, rows []kaggle.CompetitionSubmission) (kaggle.CompetitionSubmission, bool) {
	var (
		best  kaggle.CompetitionSubmission
		found bool
	)

	for _, row := range rows {
		if strings.TrimSpace(row.Description) != submission.Message {
			continue
		}
		if strings.TrimSpace(row.FileName) != submission.FileName {
			continue
		}
		if !row.SubmittedAt.IsZero() && row.SubmittedAt.Before(submission.AttemptedAt) {
			continue
		}
		if !found {
			best = row
			found = true
			continue
		}
		if best.SubmittedAt.IsZero() || row.SubmittedAt.After(best.SubmittedAt) {
			best = row
			found = true
		}
	}

	return best, found
}

func applySubmissionMetadata(submission *SubmissionResult, match kaggle.CompetitionSubmission) error {
	if submission == nil {
		return fmt.Errorf("submission result is nil")
	}

	submissionID := strings.TrimSpace(match.Ref)
	if submissionID == "" {
		return fmt.Errorf("matched Kaggle submission is missing submission ID")
	}
	status := strings.TrimSpace(match.Status)
	if status == "" {
		return fmt.Errorf("matched Kaggle submission is missing status")
	}
	if match.SubmittedAt.IsZero() {
		return fmt.Errorf("matched Kaggle submission is missing timestamp")
	}

	submission.SubmissionID = submissionID
	submission.Status = status
	submission.SubmittedAt = match.SubmittedAt
	return nil
}

func effectivePollInterval(requested, fallback time.Duration) time.Duration {
	if requested > 0 {
		return requested
	}
	if fallback > 0 {
		return fallback
	}
	return 5 * time.Second
}

func effectivePollTimeout(requested, fallback time.Duration) time.Duration {
	if requested > 0 {
		return requested
	}
	if fallback > 0 {
		return fallback
	}
	return 30 * time.Minute
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

const outputTempPrefix = "kgh-kernel-output-*"

func createOutputDir() (string, error) {
	outputDir, err := os.MkdirTemp("", outputTempPrefix)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(outputDir, 0o700); err != nil {
		_ = os.RemoveAll(outputDir)
		return "", err
	}
	return outputDir, nil
}

func buildOutputsResult(execSpec spec.ExecutionSpec, outputDir string) (OutputsResult, error) {
	resolvedDir, err := filepath.Abs(filepath.Clean(outputDir))
	if err != nil {
		return OutputsResult{}, err
	}

	info, err := os.Stat(resolvedDir)
	if err != nil {
		return OutputsResult{}, err
	}
	if !info.IsDir() {
		return OutputsResult{}, fmt.Errorf("output dir %q is not a directory", resolvedDir)
	}

	submission := resolveOutputFile(resolvedDir, execSpec.Outputs.Submission, "submission", execSpec.Submit)
	metrics := resolveOutputFile(resolvedDir, execSpec.Outputs.Metrics, "metrics", false)

	result := OutputsResult{
		OutputDir:      resolvedDir,
		SubmissionPath: submission.Path,
		MetricsPath:    metrics.Path,
		Submission:     submission,
		Metrics:        metrics,
		Validation: OutputValidationResult{
			Valid: true,
		},
	}

	applyOutputValidation(&result.Validation, "submission", submission)
	applyOutputValidation(&result.Validation, "metrics", metrics)

	return result, nil
}

func resolveOutputFile(outputDir, configuredPath, label string, required bool) OutputFileResult {
	result := OutputFileResult{
		ConfiguredPath: configuredPath,
		Required:       required,
	}
	if configuredPath == "" {
		result.Error = fmt.Sprintf("%s output is not configured", label)
		return result
	}

	expectedPath, err := filepath.Abs(filepath.Join(outputDir, filepath.Clean(configuredPath)))
	if err != nil {
		result.Error = fmt.Sprintf("resolve %s output path %q: %v", label, configuredPath, err)
		return result
	}
	result.ExpectedPath = expectedPath

	if !pathWithinBase(outputDir, expectedPath) {
		result.Error = fmt.Sprintf("%s output %q resolves outside output dir", label, configuredPath)
		return result
	}

	info, err := os.Stat(expectedPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Error = fmt.Sprintf("%s output is missing: %s", label, expectedPath)
			return result
		}
		result.Error = fmt.Sprintf("stat %s output %q: %v", label, expectedPath, err)
		return result
	}
	if info.IsDir() {
		result.Error = fmt.Sprintf("%s output is a directory: %s", label, expectedPath)
		return result
	}

	result.Path = expectedPath
	result.Present = true
	return result
}

func applyOutputValidation(validation *OutputValidationResult, name string, result OutputFileResult) {
	if validation == nil || result.Present {
		return
	}

	if result.Required {
		validation.Valid = false
		validation.MissingRequired = append(validation.MissingRequired, name)
		return
	}
	validation.MissingOptional = append(validation.MissingOptional, name)
}

func pathWithinBase(baseDir, target string) bool {
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return false
	}
	return rel != ".." && rel != "." && filepath.Dir(rel) != ".." && rel != ""
}
