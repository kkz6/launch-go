package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
)

func TestJobsAllCompleted(t *testing.T) {
	assert.False(t, jobsAllCompleted(nil), "no jobs is not completed")
	assert.False(t, jobsAllCompleted([]gitproviders.WorkflowJob{
		{Status: "completed"}, {Status: "in_progress"},
	}), "any non-completed job means not done")
	assert.True(t, jobsAllCompleted([]gitproviders.WorkflowJob{
		{Status: "completed"}, {Status: "completed"},
	}), "all completed means done")
}

func TestOverallRunStatus(t *testing.T) {
	assert.Equal(t, "queued", overallRunStatus(nil))
	assert.Equal(t, "queued", overallRunStatus([]gitproviders.WorkflowJob{
		{Status: "queued"},
	}))
	assert.Equal(t, "in_progress", overallRunStatus([]gitproviders.WorkflowJob{
		{Status: "completed"}, {Status: "in_progress"},
	}))
	assert.Equal(t, "completed", overallRunStatus([]gitproviders.WorkflowJob{
		{Status: "completed"}, {Status: "completed"},
	}))
}

func TestGHAJobsToPayload_FlattensSteps(t *testing.T) {
	out := ghaJobsToPayload([]gitproviders.WorkflowJob{
		{
			Name:   "build-and-deploy",
			Status: "in_progress",
			Steps: []gitproviders.WorkflowStep{
				{Name: "Checkout", Status: "completed", Conclusion: "success", Number: 1},
			},
		},
	})
	assert.Len(t, out, 1)
	assert.Equal(t, "build-and-deploy", out[0]["name"])
	steps, ok := out[0]["steps"].([]map[string]any)
	assert.True(t, ok)
	assert.Len(t, steps, 1)
	assert.Equal(t, "Checkout", steps[0]["name"])
	assert.Equal(t, "success", steps[0]["conclusion"])
}
