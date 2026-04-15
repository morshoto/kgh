package kaggle

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const defaultKernelPollInterval = 5 * time.Second

// KernelStatusQuerier describes the status lookup needed by the poller.
type KernelStatusQuerier interface {
	KernelStatus(context.Context, KernelStatusRequest) (KernelStatusResponse, error)
}

// PollBackoffFunc returns the next sleep duration after the given attempt.
// Attempt numbers are 1-based and count successful status checks.
type PollBackoffFunc func(attempt int, previous time.Duration) time.Duration

// KernelPoller repeatedly queries kernel status until a terminal state is observed.
type KernelPoller struct {
	client KernelStatusQuerier
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
}

// KernelPollRequest configures a polling run.
type KernelPollRequest struct {
	KernelRef string
	Debug     bool
	Interval  time.Duration
	Timeout   time.Duration
	Backoff   PollBackoffFunc
}

// KernelPollResult captures the last observed kernel status and timing metadata.
type KernelPollResult struct {
	KernelStatusResponse
	Attempts   int
	StartedAt  time.Time
	FinishedAt time.Time
	Elapsed    time.Duration
	Terminal   KernelPollTerminalState
}

// KernelPollTerminalState classifies the final observed poll outcome.
type KernelPollTerminalState string

const (
	KernelPollTerminalStateUnknown   KernelPollTerminalState = ""
	KernelPollTerminalStateSucceeded KernelPollTerminalState = "succeeded"
	KernelPollTerminalStateFailed    KernelPollTerminalState = "failed"
	KernelPollTerminalStateCancelled KernelPollTerminalState = "cancelled"
)

// KernelPollTerminalError reports a terminal non-success run state with context.
type KernelPollTerminalError struct {
	KernelRef   string
	Terminal    KernelPollTerminalState
	Attempts    int
	LastStatus  string
	LastMessage string
	Err         error
}

func (e *KernelPollTerminalError) Error() string {
	if e == nil {
		return "kaggle kernel run terminated"
	}
	if e.KernelRef == "" {
		return fmt.Sprintf("kaggle kernel run terminated: %s", e.Terminal)
	}
	if e.LastStatus == "" {
		return fmt.Sprintf("kaggle kernel %s run terminated: %s", e.KernelRef, e.Terminal)
	}
	return fmt.Sprintf("kaggle kernel %s run terminated: %s (last status: %s)", e.KernelRef, e.Terminal, e.LastStatus)
}

func (e *KernelPollTerminalError) Unwrap() error { return e.Err }

// KernelPollTimeoutError reports that polling stopped because the timeout expired.
type KernelPollTimeoutError struct {
	KernelRef   string
	Timeout     time.Duration
	Attempts    int
	LastStatus  string
	LastMessage string
	Err         error
}

func (e *KernelPollTimeoutError) Error() string {
	if e == nil {
		return "kaggle kernel polling timed out"
	}
	if e.KernelRef == "" {
		return fmt.Sprintf("kaggle kernel polling timed out after %s", e.Timeout)
	}
	if e.LastStatus == "" {
		return fmt.Sprintf("kaggle kernel %s polling timed out after %s", e.KernelRef, e.Timeout)
	}
	return fmt.Sprintf("kaggle kernel %s polling timed out after %s (last status: %s)", e.KernelRef, e.Timeout, e.LastStatus)
}

func (e *KernelPollTimeoutError) Unwrap() error { return e.Err }

// NewKernelPoller constructs a poller using production timing primitives.
func NewKernelPoller(client KernelStatusQuerier) *KernelPoller {
	return NewKernelPollerWithDeps(client, time.Now, sleepContext)
}

// NewKernelPollerWithDeps constructs a poller with injected timing functions.
func NewKernelPollerWithDeps(client KernelStatusQuerier, now func() time.Time, sleep func(context.Context, time.Duration) error) *KernelPoller {
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = sleepContext
	}
	return &KernelPoller{
		client: client,
		now:    now,
		sleep:  sleep,
	}
}

// Poll queries kernel status until a terminal state is reached or the timeout expires.
func (p *KernelPoller) Poll(ctx context.Context, req KernelPollRequest) (KernelPollResult, error) {
	if p == nil || p.client == nil {
		return KernelPollResult{}, fmt.Errorf("kaggle kernel poller client is nil")
	}

	kernelRef, err := requiredValue("kernel ref", req.KernelRef)
	if err != nil {
		return KernelPollResult{}, err
	}

	interval := req.Interval
	if interval <= 0 {
		interval = defaultKernelPollInterval
	}

	startedAt := p.now()
	deadline := time.Time{}
	if req.Timeout > 0 {
		deadline = startedAt.Add(req.Timeout)
	}

	result := KernelPollResult{
		StartedAt: startedAt,
	}

	attempts := 0
	for {
		if err := ctx.Err(); err != nil {
			result.FinishedAt = p.now()
			result.Elapsed = result.FinishedAt.Sub(startedAt)
			return result, err
		}
		if !deadline.IsZero() && !p.now().Before(deadline) {
			result.FinishedAt = p.now()
			result.Elapsed = result.FinishedAt.Sub(startedAt)
			return result, &KernelPollTimeoutError{
				KernelRef:   kernelRef,
				Timeout:     req.Timeout,
				Attempts:    attempts,
				LastStatus:  result.Status,
				LastMessage: result.Message,
			}
		}

		status, err := p.client.KernelStatus(ctx, KernelStatusRequest{
			KernelRef: kernelRef,
			Debug:     req.Debug,
		})
		attempts++
		if err != nil {
			result.FinishedAt = p.now()
			result.Elapsed = result.FinishedAt.Sub(startedAt)
			return result, err
		}

		result.KernelStatusResponse = status
		result.Attempts = attempts
		terminalState, terminal := classifyTerminalKernelStatus(status.Status)
		if terminal {
			result.FinishedAt = p.now()
			result.Elapsed = result.FinishedAt.Sub(startedAt)
			result.Terminal = terminalState
			if terminalState != KernelPollTerminalStateSucceeded {
				return result, &KernelPollTerminalError{
					KernelRef:   kernelRef,
					Terminal:    terminalState,
					Attempts:    attempts,
					LastStatus:  result.Status,
					LastMessage: result.Message,
				}
			}
			return result, nil
		}

		delay := interval
		if req.Backoff != nil {
			delay = req.Backoff(attempts, delay)
		}
		if delay < 0 {
			delay = 0
		}
		if !deadline.IsZero() {
			remaining := deadline.Sub(p.now())
			if remaining < delay {
				delay = remaining
			}
			if delay <= 0 {
				result.FinishedAt = p.now()
				result.Elapsed = result.FinishedAt.Sub(startedAt)
				return result, &KernelPollTimeoutError{
					KernelRef:   kernelRef,
					Timeout:     req.Timeout,
					Attempts:    attempts,
					LastStatus:  result.Status,
					LastMessage: result.Message,
				}
			}
		}

		if err := p.sleep(ctx, delay); err != nil {
			result.FinishedAt = p.now()
			result.Elapsed = result.FinishedAt.Sub(startedAt)
			return result, err
		}
	}
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

func isTerminalKernelStatus(status string) bool {
	_, terminal := classifyTerminalKernelStatus(status)
	return terminal
}

func classifyTerminalKernelStatus(status string) (KernelPollTerminalState, bool) {
	switch normalizeKernelStatus(status) {
	case "COMPLETE", "COMPLETED":
		return KernelPollTerminalStateSucceeded, true
	case "FAILED", "FAILURE", "ERROR":
		return KernelPollTerminalStateFailed, true
	case "CANCELLED", "CANCELED", "ABORTED", "TERMINATED":
		return KernelPollTerminalStateCancelled, true
	default:
		return KernelPollTerminalStateUnknown, false
	}
}

func normalizeKernelStatus(status string) string {
	status = strings.TrimSpace(status)
	status = strings.Trim(status, `"'`)
	if index := strings.LastIndex(status, "."); index >= 0 {
		status = status[index+1:]
	}
	return strings.ToUpper(status)
}
