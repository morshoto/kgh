package kaggle

import (
	"context"
	"errors"
	"fmt"
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
