// Package remediation is a durable infrastructure-remediation workflow on Temporal.
// Properties it demonstrates: crash-safe progress, idempotent activities (keyed by
// workflow ID), bounded retries with backoff, human approval before disruptive
// actions, saga-style rollback, and duplicate-incident prevention (workflow ID).
package remediation

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue      = "remediation"
	ApprovalSignal = "approval"
)

type Incident struct {
	Service    string
	Disruptive bool // true => requires human approval before acting
}

func WorkflowID(service string) string { return "remediate-" + service }

func RemediationWorkflow(ctx workflow.Context, in Incident) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: time.Minute, MaximumAttempts: 5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var a *Activities

	var diagnosis string
	if err := workflow.ExecuteActivity(ctx, a.Diagnose, in).Get(ctx, &diagnosis); err != nil {
		return err
	}

	if in.Disruptive {
		approved := false
		sig := workflow.GetSignalChannel(ctx, ApprovalSignal)
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(sig, func(c workflow.ReceiveChannel, _ bool) { c.Receive(ctx, &approved) })
		sel.AddFuture(workflow.NewTimer(timerCtx, 15*time.Minute), func(workflow.Future) {})
		sel.Select(ctx)
		cancelTimer()
		if !approved {
			return temporal.NewNonRetryableApplicationError("approval not granted in time", "ApprovalTimeout", nil)
		}
	}

	key := workflow.GetInfo(ctx).WorkflowExecution.ID // stable across retries and worker crashes
	if err := workflow.ExecuteActivity(ctx, a.Restart, in.Service, key).Get(ctx, nil); err != nil {
		// Compensate on a disconnected context so rollback still runs if the workflow is cancelled.
		dctx, cancel := workflow.NewDisconnectedContext(ctx)
		defer cancel()
		dctx = workflow.WithActivityOptions(dctx, ao)
		_ = workflow.ExecuteActivity(dctx, a.Rollback, in.Service, key).Get(dctx, nil)
		return err
	}
	return workflow.ExecuteActivity(ctx, a.Verify, in.Service).Get(ctx, nil)
}
