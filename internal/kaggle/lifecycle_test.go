package kaggle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestKernelLifecyclePushStatusAndPollSuccess(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &lifecycleScriptedClient{
		t: t,
		steps: []lifecycleScriptedStep{
			{
				args: []string{"kernels", "push", "-p", "/tmp/work tree"},
				result: Result{
					Stdout:   "Kernel URL: https://www.kaggle.com/code/alice/exp142\nKernel pushed successfully\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "status: running\nmessage: queued for execution\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "status: running\nmessage: still queued\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "status: complete\nmessage: finished\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
		},
	}

	adapter := &CLIAdapter{
		client: client,
		newPoller: func(querier KernelStatusQuerier) *KernelPoller {
			return NewKernelPollerWithDeps(querier, clock.Now, clock.Sleep)
		},
	}

	pushResp, err := adapter.PushKernel(context.Background(), PushKernelRequest{
		WorkDir: "/tmp/work tree",
	})
	if err != nil {
		t.Fatalf("expected push to succeed, got %v", err)
	}
	if pushResp.KernelRef != "alice/exp142" {
		t.Fatalf("unexpected kernel ref %q", pushResp.KernelRef)
	}

	statusResp, err := adapter.KernelStatus(context.Background(), KernelStatusRequest{
		KernelRef: pushResp.KernelRef,
	})
	if err != nil {
		t.Fatalf("expected status lookup to succeed, got %v", err)
	}
	if statusResp.Status != "running" {
		t.Fatalf("unexpected status %q", statusResp.Status)
	}
	if statusResp.Message != "queued for execution" {
		t.Fatalf("unexpected message %q", statusResp.Message)
	}

	pollResp, err := adapter.PollKernelStatus(context.Background(), KernelPollRequest{
		KernelRef: pushResp.KernelRef,
		Interval:  time.Second,
	})
	if err != nil {
		t.Fatalf("expected poll to succeed, got %v", err)
	}
	if pollResp.Terminal != KernelPollTerminalStateSucceeded {
		t.Fatalf("unexpected terminal state %q", pollResp.Terminal)
	}
	if pollResp.Status != "complete" || pollResp.Message != "finished" {
		t.Fatalf("unexpected poll result %+v", pollResp)
	}
	if pollResp.Attempts != 2 {
		t.Fatalf("unexpected attempts %d", pollResp.Attempts)
	}
	if !equalDurations(clock.sleeps, []time.Duration{time.Second}) {
		t.Fatalf("unexpected sleep schedule %#v", clock.sleeps)
	}
}

func TestKernelLifecyclePollTimeout(t *testing.T) {
	t.Parallel()

	clock := &pollerClock{now: time.Unix(0, 0)}
	client := &lifecycleScriptedClient{
		t: t,
		steps: []lifecycleScriptedStep{
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "status: running\nmessage: queued\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "status: running\nmessage: still queued\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "status: running\nmessage: still queued\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
		},
	}

	adapter := &CLIAdapter{
		client: client,
		newPoller: func(querier KernelStatusQuerier) *KernelPoller {
			return NewKernelPollerWithDeps(querier, clock.Now, clock.Sleep)
		},
	}

	_, err := adapter.PollKernelStatus(context.Background(), KernelPollRequest{
		KernelRef: "alice/exp142",
		Interval:  2 * time.Second,
		Timeout:   5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected polling to time out")
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
}

func TestKernelLifecycleReportsMalformedStatusOutput(t *testing.T) {
	t.Parallel()

	client := &lifecycleScriptedClient{
		t: t,
		steps: []lifecycleScriptedStep{
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				result: Result{
					Stdout:   "message: queued\n",
					Stderr:   "",
					ExitCode: 0,
				},
			},
		},
	}

	_, err := (&CLIAdapter{client: client}).KernelStatus(context.Background(), KernelStatusRequest{
		KernelRef: "alice/exp142",
	})
	if err == nil {
		t.Fatal("expected status parsing to fail")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryUnexpectedOutput {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

func TestKernelLifecycleReportsFailingStatusCommand(t *testing.T) {
	t.Parallel()

	client := &lifecycleScriptedClient{
		t: t,
		steps: []lifecycleScriptedStep{
			{
				args: []string{"kernels", "status", "-p", "alice/exp142"},
				err: &CommandError{
					ExitCode: 1,
					Stderr:   "401 Unauthorized: invalid credentials",
					Err:      errors.New("exit status 1"),
				},
			},
		},
	}

	_, err := (&CLIAdapter{client: client}).KernelStatus(context.Background(), KernelStatusRequest{
		KernelRef: "alice/exp142",
	})
	if err == nil {
		t.Fatal("expected status command failure")
	}

	var adapterErr *AdapterError
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if adapterErr.Category != ErrorCategoryInvalidCredentials {
		t.Fatalf("unexpected category %q", adapterErr.Category)
	}
}

type lifecycleScriptedStep struct {
	args   []string
	result Result
	err    error
}

type lifecycleScriptedClient struct {
	t     *testing.T
	steps []lifecycleScriptedStep
	calls int
}

func (c *lifecycleScriptedClient) Run(_ context.Context, args []string, opts RunOptions) (Result, error) {
	if c.calls >= len(c.steps) {
		c.t.Fatalf("unexpected call %d with args %#v", c.calls+1, args)
	}

	step := c.steps[c.calls]
	c.calls++

	if !equalStrings(args, step.args) {
		c.t.Fatalf("unexpected args %#v", args)
	}
	assertZeroRunOptions(c.t, opts)

	return step.result, step.err
}
