package providers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkflowRunJobs_ParsesJobsAndSteps(t *testing.T) {
	f := newFakeGitHub(t)
	defer f.Close()
	p := testProvider(t, f.srv.URL)

	jobs, err := p.ListWorkflowRunJobs(context.Background(), "inst-1", "o", "r", "5")
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "build-and-deploy", jobs[0].Name)
	assert.Equal(t, "in_progress", jobs[0].Status)
	require.Len(t, jobs[0].Steps, 2)
	assert.Equal(t, "Set up job", jobs[0].Steps[0].Name)
	assert.Equal(t, "completed", jobs[0].Steps[0].Status)
	assert.Equal(t, "success", jobs[0].Steps[0].Conclusion)
	assert.Equal(t, 1, jobs[0].Steps[0].Number)
	assert.Equal(t, "in_progress", jobs[0].Steps[1].Status)
	assert.Empty(t, jobs[0].Steps[1].Conclusion)
}

func TestListWorkflowRunJobs_RequiresRunID(t *testing.T) {
	p := testProvider(t, "http://example.invalid")
	_, err := p.ListWorkflowRunJobs(context.Background(), "inst-1", "o", "r", "")
	require.Error(t, err)
}

func TestFindLatestWorkflowRun_PicksRunAtOrAfterSince(t *testing.T) {
	f := newFakeGitHub(t)
	defer f.Close()
	p := testProvider(t, f.srv.URL)

	// The fake's run is created 2026-06-09T00:00:00Z. A dispatch the day
	// before is comfortably before it → the run is matched.
	since := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	run, err := p.FindLatestWorkflowRun(context.Background(), "inst-1", "o", "r", "launch-deploy-x.yml", "main", since)
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, int64(12345), run.ID)
	assert.Equal(t, "in_progress", run.Status)
	assert.Equal(t, "workflow_dispatch", run.Event)
}

func TestFindLatestWorkflowRun_IgnoresRunsBeforeSince(t *testing.T) {
	f := newFakeGitHub(t)
	defer f.Close()
	p := testProvider(t, f.srv.URL)

	// A dispatch well after the fake's run (even past the 30s skew slack)
	// → nothing matches.
	since := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	run, err := p.FindLatestWorkflowRun(context.Background(), "inst-1", "o", "r", "launch-deploy-x.yml", "main", since)
	require.NoError(t, err)
	assert.Nil(t, run)
}
