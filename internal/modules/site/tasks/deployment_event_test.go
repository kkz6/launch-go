package tasks

import (
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	pkgevents "github.com/kkz6/launch-go/internal/pkg/events"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type callbackQueueStub struct {
	tasks []*asynq.Task
}

func (q *callbackQueueStub) Enqueue(
	task *asynq.Task,
	_ ...asynq.Option,
) (*asynq.TaskInfo, error) {
	q.tasks = append(q.tasks, task)
	return &asynq.TaskInfo{}, nil
}

func TestDeployTaskDispatchesTypedDeploymentSucceededEvent(t *testing.T) {
	queue := &callbackQueueStub{}
	task := &deploySiteTask{callback: callbackData{
		DeploymentID: "deployment-1",
		SiteID:       "site-1",
		ServerID:     "server-1",
		TeamID:       "team-1",
	}}

	err := task.dispatchDeploymentSucceeded(&taskrunner.CallbackContext{Queue: queue}, "task-1")
	require.NoError(t, err)
	require.Len(t, queue.tasks, 1)
	require.Equal(t, siteevents.TypeProcessEvent, queue.tasks[0].Type())

	event, err := pkgjobs.UnmarshalPayload[pkgevents.Event](queue.tasks[0])
	require.NoError(t, err)
	require.Equal(t, "deployment-1", event.ID)
	require.Equal(t, siteevents.DeploymentSucceededName, event.Name)

	payload, err := pkgevents.Decode[siteevents.DeploymentSucceeded](event)
	require.NoError(t, err)
	require.Equal(t, "site-1", payload.SiteID)
	require.Equal(t, "task-1", payload.TaskID)
}

func TestDeploymentEventTaskIDIsUniquePerDeployment(t *testing.T) {
	first := deploymentEventTaskID(siteevents.DeploymentSucceededName, "deployment-1")
	second := deploymentEventTaskID(siteevents.DeploymentSucceededName, "deployment-2")

	require.NotEqual(t, first, second)
	require.Contains(t, first, "deployment-1")
	require.Contains(t, second, "deployment-2")
}

func TestRestartAllQueuesScriptFailsWhenAnyQueueCannotStart(t *testing.T) {
	script := RestartAllQueues([]string{"queue-1", "queue-2"}).Script()

	require.Contains(t, script, `supervisorctl start "queue-1":*`)
	require.Contains(t, script, `supervisorctl start "queue-2":*`)
	require.Contains(t, script, "restart_failed=1")
	require.Contains(t, script, `if [ "$restart_failed" -ne 0 ]; then`)
	require.Contains(t, script, "exit 1")
}
