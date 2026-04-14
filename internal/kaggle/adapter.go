package kaggle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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
}

type PushKernelResponse struct {
	KernelRef string
}

type KernelStatusRequest struct {
	KernelRef string
}

type KernelStatusResponse struct {
	KernelRef string
	Status    string
	Message   string
}

type DownloadKernelOutputRequest struct {
	KernelRef string
	OutputDir string
}

type DownloadKernelOutputResponse struct {
	OutputDir string
}

type CompetitionSubmitRequest struct {
	Competition string
	FilePath    string
	Message     string
}

type CompetitionSubmitResponse struct {
	Competition string
	Submitted   bool
}

type CompetitionSubmissionsRequest struct {
	Competition string
	Limit       int
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
	if _, err := a.run(ctx, args); err != nil {
		return PushKernelResponse{}, err
	}
	return PushKernelResponse{}, nil
}

func (a *CLIAdapter) KernelStatus(ctx context.Context, req KernelStatusRequest) (KernelStatusResponse, error) {
	args, err := buildKernelStatusCommand(req)
	if err != nil {
		return KernelStatusResponse{}, err
	}
	if _, err := a.run(ctx, args); err != nil {
		return KernelStatusResponse{}, err
	}
	return KernelStatusResponse{KernelRef: req.KernelRef}, nil
}

func (a *CLIAdapter) DownloadKernelOutput(ctx context.Context, req DownloadKernelOutputRequest) (DownloadKernelOutputResponse, error) {
	args, err := buildDownloadKernelOutputCommand(req)
	if err != nil {
		return DownloadKernelOutputResponse{}, err
	}
	if _, err := a.run(ctx, args); err != nil {
		return DownloadKernelOutputResponse{}, err
	}
	return DownloadKernelOutputResponse{OutputDir: req.OutputDir}, nil
}

func (a *CLIAdapter) SubmitCompetition(ctx context.Context, req CompetitionSubmitRequest) (CompetitionSubmitResponse, error) {
	args, err := buildCompetitionSubmitCommand(req)
	if err != nil {
		return CompetitionSubmitResponse{}, err
	}
	if _, err := a.run(ctx, args); err != nil {
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
	if _, err := a.run(ctx, args); err != nil {
		return CompetitionSubmissionsResponse{}, err
	}
	return CompetitionSubmissionsResponse{}, nil
}

func (a *CLIAdapter) run(ctx context.Context, args []string) (Result, error) {
	if a == nil || a.client == nil {
		return Result{}, fmt.Errorf("kaggle adapter client is nil")
	}
	return a.client.Run(ctx, args, RunOptions{})
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
