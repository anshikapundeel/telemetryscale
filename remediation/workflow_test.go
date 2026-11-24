package remediation

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestRetriesTransientFailureThenSucceeds(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	a := &Activities{}
	env.RegisterActivity(a)
	env.OnActivity(a.Diagnose, mock.Anything, mock.Anything).Return("ok", nil)
	env.OnActivity(a.Restart, mock.Anything, "svc", mock.Anything).Return(errors.New("transient")).Twice()
	env.OnActivity(a.Restart, mock.Anything, "svc", mock.Anything).Return(nil)
	env.OnActivity(a.Verify, mock.Anything, "svc").Return(nil)
	env.ExecuteWorkflow(RemediationWorkflow, Incident{Service: "svc"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestRollsBackWhenRestartExhaustsRetries(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	a := &Activities{}
	env.RegisterActivity(a)
	env.OnActivity(a.Diagnose, mock.Anything, mock.Anything).Return("ok", nil)
	env.OnActivity(a.Restart, mock.Anything, "svc", mock.Anything).Return(errors.New("permanent"))
	env.OnActivity(a.Rollback, mock.Anything, "svc", mock.Anything).Return(nil).Once()
	env.ExecuteWorkflow(RemediationWorkflow, Incident{Service: "svc"})
	require.Error(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestDisruptiveActionWaitsForApproval(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	a := &Activities{}
	env.RegisterActivity(a)
	env.OnActivity(a.Diagnose, mock.Anything, mock.Anything).Return("ok", nil)
	env.OnActivity(a.Restart, mock.Anything, "svc", mock.Anything).Return(nil).Once()
	env.OnActivity(a.Verify, mock.Anything, "svc").Return(nil)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(ApprovalSignal, true) }, 2*time.Minute)
	env.ExecuteWorkflow(RemediationWorkflow, Incident{Service: "svc", Disruptive: true})
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestNoApprovalMeansNoAction(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	a := &Activities{}
	env.RegisterActivity(a)
	env.OnActivity(a.Diagnose, mock.Anything, mock.Anything).Return("ok", nil)
	env.ExecuteWorkflow(RemediationWorkflow, Incident{Service: "svc", Disruptive: true})
	require.Error(t, env.GetWorkflowError()) // timed out; Restart was never mocked, so never called
}

func TestMemRemediatorIsIdempotent(t *testing.T) {
	m := NewMemRemediator()
	require.NoError(t, m.Restart("svc", "k"))
	require.NoError(t, m.Restart("svc", "k"))
	require.Equal(t, 1, m.Applied["k"])
}
