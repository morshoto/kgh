package kaggle

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestKernelPollerStopsOnTerminalStatus(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &pollerFakeClient{
		t: t,
		responses: []KernelStatusResponse{
			{KernelRef: "alice/exp142", Status: "running", Message: "queued"},
			{KernelRef: "alice/exp142", Status: "complete", Message: "finished"},
		},
	}
	poller := NewKernelPollerWithDeps(client, clock.Now, clock.Sleep)

	result, err := poller.Poll(context.Background(), KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected two attempts, got %d", result.Attempts)
	}
	if result.Status != "complete" || result.Message != "finished" {
		t.Fatalf("unexpected result %+v", result)
	}
	if result.Terminal != KernelPollTerminalStateSucceeded {
		t.Fatalf("unexpected terminal state %q", result.Terminal)
	}
	if result.StartedAt != time.Unix(0, 0) {
		t.Fatalf("unexpected start time %s", result.StartedAt)
	}
	if result.FinishedAt != time.Unix(0, 0).Add(2*time.Second) {
		t.Fatalf("unexpected finish time %s", result.FinishedAt)
	}
	if result.Elapsed != 2*time.Second {
		t.Fatalf("unexpected elapsed %s", result.Elapsed)
	}
	if !equalDurations(clock.sleeps, []time.Duration{2 * time.Second}) {
		t.Fatalf("unexpected sleep schedule %#v", clock.sleeps)
	}
}

func TestKernelPollerAppliesBackoffStrategy(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &pollerFakeClient{
		t: t,
		responses: []KernelStatusResponse{
			{KernelRef: "alice/exp142", Status: "running"},
			{KernelRef: "alice/exp142", Status: "running"},
			{KernelRef: "alice/exp142", Status: "complete"},
		},
	}
	poller := NewKernelPollerWithDeps(client, clock.Now, clock.Sleep)

	_, err := poller.Poll(context.Background(), KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  time.Second,
		Backoff: func(attempt int, previous time.Duration) time.Duration {
			return time.Duration(attempt) * previous
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !equalDurations(clock.sleeps, []time.Duration{1 * time.Second, 2 * time.Second}) {
		t.Fatalf("unexpected sleep schedule %#v", clock.sleeps)
	}
}

func TestKernelPollerReturnsTimeoutError(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &pollerFakeClient{
		t: t,
		responses: []KernelStatusResponse{
			{KernelRef: "alice/exp142", Status: "running", Message: "queued"},
			{KernelRef: "alice/exp142", Status: "running", Message: "still queued"},
			{KernelRef: "alice/exp142", Status: "running", Message: "still queued"},
		},
	}
	poller := NewKernelPollerWithDeps(client, clock.Now, clock.Sleep)

	result, err := poller.Poll(context.Background(), KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  2 * time.Second,
		Timeout:   5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	var timeoutErr *KernelPollTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected KernelPollTimeoutError, got %T", err)
	}
	if timeoutErr.KernelRef != "alice/exp142" {
		t.Fatalf("unexpected kernel ref %q", timeoutErr.KernelRef)
	}
	if timeoutErr.Timeout != 5*time.Second {
		t.Fatalf("unexpected timeout %s", timeoutErr.Timeout)
	}
	if timeoutErr.Attempts != 3 {
		t.Fatalf("unexpected attempts %d", timeoutErr.Attempts)
	}
	if timeoutErr.LastStatus != "running" {
		t.Fatalf("unexpected last status %q", timeoutErr.LastStatus)
	}
	if result.Attempts != 3 {
		t.Fatalf("unexpected result attempts %d", result.Attempts)
	}
	if result.Status != "running" || result.Message != "still queued" {
		t.Fatalf("unexpected partial result %+v", result)
	}
	if result.Terminal != KernelPollTerminalStateUnknown {
		t.Fatalf("unexpected terminal state %q", result.Terminal)
	}
	if result.Elapsed != 5*time.Second {
		t.Fatalf("unexpected elapsed %s", result.Elapsed)
	}
	if !equalDurations(clock.sleeps, []time.Duration{2 * time.Second, 2 * time.Second, 1 * time.Second}) {
		t.Fatalf("unexpected sleep schedule %#v", clock.sleeps)
	}
}

func TestKernelPollerHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &pollerFakeClient{
		t: t,
		responses: []KernelStatusResponse{
			{KernelRef: "alice/exp142", Status: "running"},
		},
	}
	poller := NewKernelPollerWithDeps(client, clock.Now, func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	})

	_, err := poller.Poll(ctx, KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  time.Second,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestKernelPollerReturnsTerminalErrorForFailedStatus(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &pollerFakeClient{
		t: t,
		responses: []KernelStatusResponse{
			{KernelRef: "alice/exp142", Status: "running", Message: "queued"},
			{KernelRef: "alice/exp142", Status: "failed", Message: "kernel crashed"},
		},
	}
	poller := NewKernelPollerWithDeps(client, clock.Now, clock.Sleep)

	result, err := poller.Poll(context.Background(), KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  2 * time.Second,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	var terminalErr *KernelPollTerminalError
	if !errors.As(err, &terminalErr) {
		t.Fatalf("expected KernelPollTerminalError, got %T", err)
	}
	if terminalErr.Terminal != KernelPollTerminalStateFailed {
		t.Fatalf("unexpected terminal state %q", terminalErr.Terminal)
	}
	if terminalErr.LastStatus != "failed" || terminalErr.LastMessage != "kernel crashed" {
		t.Fatalf("unexpected terminal error %+v", terminalErr)
	}
	if result.Terminal != KernelPollTerminalStateFailed {
		t.Fatalf("unexpected result terminal state %q", result.Terminal)
	}
	if result.Attempts != 2 {
		t.Fatalf("unexpected attempts %d", result.Attempts)
	}
}

func TestKernelPollerReturnsTerminalErrorForCancelledStatus(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &pollerFakeClient{
		t: t,
		responses: []KernelStatusResponse{
			{KernelRef: "alice/exp142", Status: "running", Message: "queued"},
			{KernelRef: "alice/exp142", Status: "cancelled", Message: "user cancelled"},
		},
	}
	poller := NewKernelPollerWithDeps(client, clock.Now, clock.Sleep)

	result, err := poller.Poll(context.Background(), KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  2 * time.Second,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	var terminalErr *KernelPollTerminalError
	if !errors.As(err, &terminalErr) {
		t.Fatalf("expected KernelPollTerminalError, got %T", err)
	}
	if terminalErr.Terminal != KernelPollTerminalStateCancelled {
		t.Fatalf("unexpected terminal state %q", terminalErr.Terminal)
	}
	if terminalErr.LastStatus != "cancelled" || terminalErr.LastMessage != "user cancelled" {
		t.Fatalf("unexpected terminal error %+v", terminalErr)
	}
	if result.Terminal != KernelPollTerminalStateCancelled {
		t.Fatalf("unexpected result terminal state %q", result.Terminal)
	}
	if result.Attempts != 2 {
		t.Fatalf("unexpected attempts %d", result.Attempts)
	}
}

func TestIsTerminalKernelStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   bool
	}{
		{status: "complete", want: true},
		{status: "completed", want: true},
		{status: "failed", want: true},
		{status: "error", want: true},
		{status: "cancelled", want: true},
		{status: "running", want: false},
		{status: "queued", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()

			if got := isTerminalKernelStatus(tt.status); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestClassifyTerminalKernelStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   KernelPollTerminalState
		ok     bool
	}{
		{status: "complete", want: KernelPollTerminalStateSucceeded, ok: true},
		{status: "failed", want: KernelPollTerminalStateFailed, ok: true},
		{status: "error", want: KernelPollTerminalStateFailed, ok: true},
		{status: "cancelled", want: KernelPollTerminalStateCancelled, ok: true},
		{status: "aborted", want: KernelPollTerminalStateCancelled, ok: true},
		{status: "running", want: KernelPollTerminalStateUnknown, ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()

			got, ok := classifyTerminalKernelStatus(tt.status)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("unexpected classification got=(%q,%v) want=(%q,%v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

type pollerFakeClient struct {
	t         *testing.T
	responses []KernelStatusResponse
	errors    []error
	calls     int
}

func (f *pollerFakeClient) KernelStatus(context.Context, KernelStatusRequest) (KernelStatusResponse, error) {
	if f.t == nil {
		panic("pollerFakeClient requires a test handle")
	}
	if f.calls >= len(f.responses) {
		f.t.Fatalf("unexpected KernelStatus call %d", f.calls+1)
	}
	resp := f.responses[f.calls]
	var err error
	if f.calls < len(f.errors) {
		err = f.errors[f.calls]
	}
	f.calls++
	return resp, err
}

type pollerClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *pollerClock) Now() time.Time {
	return c.now
}

func (c *pollerClock) Sleep(_ context.Context, d time.Duration) error {
	if d < 0 {
		return fmt.Errorf("negative sleep %s", d)
	}
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	return nil
}

func equalDurations(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
