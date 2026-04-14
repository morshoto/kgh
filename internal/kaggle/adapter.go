package kaggle

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shotomorisk/kgh/internal/kernelref"
)

// Adapter defines workflow-level Kaggle operations without exposing shell details.
type Adapter interface {
	PushKernel(context.Context, PushKernelRequest) (PushKernelResponse, error)
	KernelStatus(context.Context, KernelStatusRequest) (KernelStatusResponse, error)
	DownloadKernelOutput(context.Context, DownloadKernelOutputRequest) (DownloadKernelOutputResponse, error)
	SubmitCompetition(context.Context, CompetitionSubmitRequest) (CompetitionSubmitResponse, error)
	ListCompetitionSubmissions(context.Context, CompetitionSubmissionsRequest) (CompetitionSubmissionsResponse, error)
}

type PushKernelRequest struct {
	WorkDir string
	Debug   bool
}

type PushKernelResponse struct {
	KernelRef string
	Output    Result
}

type KernelStatusRequest struct {
	KernelRef string
	Debug     bool
}

type KernelStatusResponse struct {
	KernelRef string
	Status    string
	Message   string
	Raw       KernelStatusRawStatus
}

// KernelStatusRawStatus preserves the parsed CLI output alongside the normalized
// convenience fields exposed by KernelStatusResponse.
type KernelStatusRawStatus struct {
	Fields   map[string]string
	Stdout   string
	Stderr   string
	ExitCode int
}

type DownloadKernelOutputRequest struct {
	KernelRef string
	OutputDir string
	Debug     bool
}

type DownloadKernelOutputResponse struct {
	OutputDir string
}

type CompetitionSubmitRequest struct {
	Competition string
	FilePath    string
	Message     string
	Debug       bool
}

type CompetitionSubmitResponse struct {
	Competition string
	Submitted   bool
}

type CompetitionSubmissionsRequest struct {
	Competition string
	Limit       int
	Debug       bool
}

type CompetitionSubmissionsResponse struct {
	Submissions []CompetitionSubmission
}

type CompetitionSubmission struct {
	FileName    string
	Description string
	Status      string
	PublicScore string
	SubmittedAt time.Time
}

var ErrNotImplemented = errors.New("kaggle adapter operation not implemented")
var ErrUnsupportedRequest = errors.New("kaggle adapter request not supported")

type clientRunner interface {
	Run(context.Context, []string, RunOptions) (Result, error)
}

// CLIAdapter maps typed adapter operations to Kaggle CLI invocations.
type CLIAdapter struct {
	client clientRunner
}

// NewAdapter constructs a workflow-level adapter backed by the Kaggle CLI client.
func NewAdapter(client *Client) Adapter {
	return &CLIAdapter{client: client}
}

func (a *CLIAdapter) PushKernel(ctx context.Context, req PushKernelRequest) (PushKernelResponse, error) {
	args, err := buildPushKernelCommand(req)
	if err != nil {
		return PushKernelResponse{}, err
	}
	result, err := a.run(ctx, "push kernel", args, req.Debug)
	if err != nil {
		return PushKernelResponse{}, err
	}
	kernelRef, err := extractKernelRefFromPushResult(result)
	if err != nil {
		return PushKernelResponse{}, err
	}
	return PushKernelResponse{KernelRef: kernelRef, Output: result}, nil
}

func (a *CLIAdapter) KernelStatus(ctx context.Context, req KernelStatusRequest) (KernelStatusResponse, error) {
	args, err := buildKernelStatusCommand(req)
	if err != nil {
		return KernelStatusResponse{}, err
	}
	result, err := a.run(ctx, "kernel status", args, req.Debug)
	if err != nil {
		return KernelStatusResponse{}, err
	}
	resp, err := parseKernelStatusResult(req.KernelRef, result)
	if err != nil {
		return KernelStatusResponse{}, err
	}
	return resp, nil
}

func (a *CLIAdapter) DownloadKernelOutput(ctx context.Context, req DownloadKernelOutputRequest) (DownloadKernelOutputResponse, error) {
	args, err := buildDownloadKernelOutputCommand(req)
	if err != nil {
		return DownloadKernelOutputResponse{}, err
	}
	if _, err := a.run(ctx, "download kernel output", args, req.Debug); err != nil {
		return DownloadKernelOutputResponse{}, err
	}
	return DownloadKernelOutputResponse{OutputDir: req.OutputDir}, nil
}

func (a *CLIAdapter) SubmitCompetition(ctx context.Context, req CompetitionSubmitRequest) (CompetitionSubmitResponse, error) {
	args, err := buildCompetitionSubmitCommand(req)
	if err != nil {
		return CompetitionSubmitResponse{}, err
	}
	if _, err := a.run(ctx, "submit competition", args, req.Debug); err != nil {
		return CompetitionSubmitResponse{}, err
	}
	return CompetitionSubmitResponse{
		Competition: req.Competition,
		Submitted:   true,
	}, nil
}

func (a *CLIAdapter) ListCompetitionSubmissions(ctx context.Context, req CompetitionSubmissionsRequest) (CompetitionSubmissionsResponse, error) {
	args, err := buildCompetitionSubmissionsCommand(req)
	if err != nil {
		return CompetitionSubmissionsResponse{}, err
	}
	result, err := a.run(ctx, "list competition submissions", args, req.Debug)
	if err != nil {
		return CompetitionSubmissionsResponse{}, err
	}
	resp, err := parseCompetitionSubmissionsResult(result)
	if err != nil {
		return CompetitionSubmissionsResponse{}, err
	}
	return resp, nil
}

func (a *CLIAdapter) run(ctx context.Context, operation string, args []string, debug bool) (Result, error) {
	if a == nil || a.client == nil {
		return Result{}, fmt.Errorf("kaggle adapter client is nil")
	}
	result, err := a.client.Run(ctx, args, RunOptions{Debug: debug})
	if err != nil {
		return result, normalizeAdapterError(operation, err)
	}
	return result, nil
}

// StubAdapter is a compile-ready placeholder implementation for tests and wiring.
type StubAdapter struct{}

func (StubAdapter) PushKernel(context.Context, PushKernelRequest) (PushKernelResponse, error) {
	return PushKernelResponse{}, notImplemented("push kernel")
}

func (StubAdapter) KernelStatus(context.Context, KernelStatusRequest) (KernelStatusResponse, error) {
	return KernelStatusResponse{}, notImplemented("kernel status")
}

func (StubAdapter) DownloadKernelOutput(context.Context, DownloadKernelOutputRequest) (DownloadKernelOutputResponse, error) {
	return DownloadKernelOutputResponse{}, notImplemented("download kernel output")
}

func (StubAdapter) SubmitCompetition(context.Context, CompetitionSubmitRequest) (CompetitionSubmitResponse, error) {
	return CompetitionSubmitResponse{}, notImplemented("submit competition")
}

func (StubAdapter) ListCompetitionSubmissions(context.Context, CompetitionSubmissionsRequest) (CompetitionSubmissionsResponse, error) {
	return CompetitionSubmissionsResponse{}, notImplemented("list competition submissions")
}

func notImplemented(operation string) error {
	return fmt.Errorf("%w: %s", ErrNotImplemented, operation)
}

func buildPushKernelCommand(req PushKernelRequest) ([]string, error) {
	workDir, err := requiredValue("work dir", req.WorkDir)
	if err != nil {
		return nil, err
	}
	return []string{"kernels", "push", "-p", workDir}, nil
}

func buildKernelStatusCommand(req KernelStatusRequest) ([]string, error) {
	kernelRef, err := requiredValue("kernel ref", req.KernelRef)
	if err != nil {
		return nil, err
	}
	return []string{"kernels", "status", "-p", kernelRef}, nil
}

func buildDownloadKernelOutputCommand(req DownloadKernelOutputRequest) ([]string, error) {
	kernelRef, err := requiredValue("kernel ref", req.KernelRef)
	if err != nil {
		return nil, err
	}
	outputDir, err := requiredValue("output dir", req.OutputDir)
	if err != nil {
		return nil, err
	}
	return []string{"kernels", "output", kernelRef, "-p", outputDir}, nil
}

func buildCompetitionSubmitCommand(req CompetitionSubmitRequest) ([]string, error) {
	competition, err := requiredValue("competition", req.Competition)
	if err != nil {
		return nil, err
	}
	filePath, err := requiredValue("file path", req.FilePath)
	if err != nil {
		return nil, err
	}
	message, err := requiredValue("message", req.Message)
	if err != nil {
		return nil, err
	}
	return []string{"competitions", "submit", "-c", competition, "-f", filePath, "-m", message}, nil
}

func buildCompetitionSubmissionsCommand(req CompetitionSubmissionsRequest) ([]string, error) {
	competition, err := requiredValue("competition", req.Competition)
	if err != nil {
		return nil, err
	}
	if req.Limit != 0 {
		return nil, unsupportedRequest("competition submissions limit")
	}
	return []string{"competitions", "submissions", "-c", competition}, nil
}

func requiredValue(field string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return trimmed, nil
}

func unsupportedRequest(detail string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedRequest, detail)
}

func parseKernelStatusResult(kernelRef string, result Result) (KernelStatusResponse, error) {
	fields := parseKeyValueOutput(result.Stdout)
	status := firstNonEmpty(fields["status"], inferStatus(result.Stdout))
	message := firstNonEmpty(fields["message"], fields["error"])
	if strings.TrimSpace(status) == "" {
		return KernelStatusResponse{}, unexpectedOutputError("kernel status", result, "missing status field")
	}
	return KernelStatusResponse{
		KernelRef: kernelRef,
		Status:    strings.TrimSpace(status),
		Message:   strings.TrimSpace(message),
		Raw: KernelStatusRawStatus{
			Fields:   cloneStringMap(fields),
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
			ExitCode: result.ExitCode,
		},
	}, nil
}

func parseCompetitionSubmissionsResult(result Result) (CompetitionSubmissionsResponse, error) {
	output := strings.TrimSpace(result.Stdout)
	if output == "" {
		return CompetitionSubmissionsResponse{}, nil
	}
	rows, err := readDelimitedRows(output)
	if err != nil {
		return CompetitionSubmissionsResponse{}, unexpectedOutputError("list competition submissions", result, err.Error())
	}
	if len(rows) == 0 {
		return CompetitionSubmissionsResponse{}, nil
	}
	header := normalizeHeader(rows[0])
	required := []string{"file", "description", "date", "status", "publicscore"}
	for _, name := range required {
		if _, ok := header[name]; !ok {
			return CompetitionSubmissionsResponse{}, unexpectedOutputError("list competition submissions", result, "missing submissions header")
		}
	}

	submissions := make([]CompetitionSubmission, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		submission := CompetitionSubmission{
			FileName:    valueAt(row, header, "file"),
			Description: valueAt(row, header, "description"),
			Status:      valueAt(row, header, "status"),
			PublicScore: valueAt(row, header, "publicscore"),
		}
		date := valueAt(row, header, "date")
		if strings.TrimSpace(date) != "" {
			parsed, err := parseSubmissionTime(date)
			if err != nil {
				return CompetitionSubmissionsResponse{}, unexpectedOutputError("list competition submissions", result, "invalid submission date")
			}
			submission.SubmittedAt = parsed
		}
		submissions = append(submissions, submission)
	}
	return CompetitionSubmissionsResponse{Submissions: submissions}, nil
}

func parseKeyValueOutput(output string) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), " ", ""))
		values[key] = strings.TrimSpace(value)
	}
	return values
}

func inferStatus(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		index := strings.Index(lower, "status")
		if index == -1 {
			continue
		}
		value := strings.TrimSpace(line[index+len("status"):])
		value = strings.TrimLeft(value, " :=-\"'")
		value = strings.TrimRight(value, "\"'")
		if value != "" {
			return value
		}
	}
	return ""
}

func readDelimitedRows(output string) ([][]string, error) {
	delimiter := detectDelimiter(output)
	reader := csv.NewReader(strings.NewReader(output))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.Comma = delimiter

	rows := make([][]string, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		trimmed := trimRow(record)
		if len(trimmed) == 0 {
			continue
		}
		rows = append(rows, trimmed)
	}
}

func detectDelimiter(output string) rune {
	firstLine := output
	if index := strings.IndexByte(output, '\n'); index >= 0 {
		firstLine = output[:index]
	}
	switch {
	case strings.Contains(firstLine, "\t"):
		return '\t'
	case strings.Contains(firstLine, "|"):
		return '|'
	default:
		return ','
	}
}

func trimRow(row []string) []string {
	trimmed := make([]string, 0, len(row))
	for _, value := range row {
		value = strings.TrimSpace(strings.Trim(value, `"`))
		trimmed = append(trimmed, value)
	}
	if len(trimmed) == 1 && trimmed[0] == "" {
		return nil
	}
	return trimmed
}

func normalizeHeader(row []string) map[string]int {
	header := make(map[string]int, len(row))
	for i, value := range row {
		key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
		header[key] = i
	}
	return header
}

func valueAt(row []string, header map[string]int, key string) string {
	index, ok := header[key]
	if !ok || index >= len(row) {
		return ""
	}
	return row[index]
}

func parseSubmissionTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("parse submission time")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func extractKernelRefFromPushResult(result Result) (string, error) {
	output := strings.TrimSpace(strings.Join([]string{result.Stdout, result.Stderr}, "\n"))
	kernelRef, err := kernelref.ExtractFromText(output)
	if err != nil {
		return "", unexpectedOutputError("push kernel", result, err.Error())
	}
	return kernelRef, nil
}
