package tasks

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgevents "github.com/kkz6/launch-go/internal/pkg/events"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type callbackQueueStub struct {
	tasks        []*asynq.Task
	err          error
	errorsByType map[string]error
}

func (q *callbackQueueStub) Enqueue(
	task *asynq.Task,
	_ ...asynq.Option,
) (*asynq.TaskInfo, error) {
	if err := q.errorsByType[task.Type()]; err != nil {
		return nil, err
	}
	if q.err != nil {
		return nil, q.err
	}
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

func TestDeployTaskRequiresDeploymentIDForSucceededEvent(t *testing.T) {
	task := &deploySiteTask{callback: callbackData{SiteID: "site-1"}}

	err := task.dispatchDeploymentSucceeded(
		&taskrunner.CallbackContext{Queue: &callbackQueueStub{}},
		"task-1",
	)

	require.ErrorContains(t, err, "event id is required")
}

func TestDeployTaskRequiresQueueForSucceededEvent(t *testing.T) {
	task := deploymentEventTask()

	err := task.dispatchDeploymentSucceeded(&taskrunner.CallbackContext{}, "task-1")

	require.ErrorContains(t, err, "queue not available")
}

func TestDeployTaskReturnsSucceededEventQueueError(t *testing.T) {
	expectedErr := errors.New("redis unavailable")
	task := deploymentEventTask()

	err := task.dispatchDeploymentSucceeded(
		&taskrunner.CallbackContext{Queue: &callbackQueueStub{err: expectedErr}},
		"task-1",
	)

	require.ErrorIs(t, err, expectedErr)
}

func TestDeployTaskTreatsDuplicateSucceededEventAsPublished(t *testing.T) {
	for _, queueErr := range []error{asynq.ErrTaskIDConflict, asynq.ErrDuplicateTask} {
		task := deploymentEventTask()

		err := task.dispatchDeploymentSucceeded(
			&taskrunner.CallbackContext{Queue: &callbackQueueStub{err: queueErr}},
			"task-1",
		)

		require.NoError(t, err)
	}
}

func TestDeploySuccessPublishesSucceededEvent(t *testing.T) {
	db := deploymentCallbackDB(t)
	queue := &callbackQueueStub{}
	task := deploymentEventTask()

	err := task.OnSuccess(
		context.Background(),
		&taskrunner.CallbackContext{DB: db, Queue: queue},
		"task-1",
	)

	require.NoError(t, err)
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment, "id = ?", "deployment-1").Error)
	require.Equal(t, sitetypes.DeploymentStatusFinished, deployment.Status)
	require.Len(t, queue.tasks, 2)
	require.Equal(t, "site:update_provider_deployment_status", queue.tasks[0].Type())
	require.Equal(t, siteevents.TypeProcessEvent, queue.tasks[1].Type())
}

func TestDeploySuccessLogsSucceededEventPublishingFailure(t *testing.T) {
	db := deploymentCallbackDB(t)
	var output bytes.Buffer
	logger := zerolog.New(&output)
	task := deploymentEventTask()
	queue := &callbackQueueStub{errorsByType: map[string]error{
		siteevents.TypeProcessEvent: errors.New("redis unavailable"),
	}}

	err := task.OnSuccess(
		context.Background(),
		&taskrunner.CallbackContext{DB: db, Queue: queue, Logger: &logger},
		"task-1",
	)

	require.NoError(t, err)
	require.Contains(t, output.String(), "failed to publish deployment succeeded event")
	require.Contains(t, output.String(), "redis unavailable")
}

func TestRestartAllQueuesScriptFailsWhenAnyQueueCannotStart(t *testing.T) {
	script := RestartAllQueues([]string{"queue-1", "queue-2"}).Script()

	require.Contains(t, script, `supervisorctl start "queue-1":*`)
	require.Contains(t, script, `supervisorctl start "queue-2":*`)
	require.Contains(t, script, "restart_failed=1")
	require.Contains(t, script, `if [ "$restart_failed" -ne 0 ]; then`)
	require.Contains(t, script, "exit 1")
}

func deploymentEventTask() *deploySiteTask {
	return &deploySiteTask{callback: callbackData{
		DeploymentID: "deployment-1",
		SiteID:       "site-1",
		ServerID:     "server-1",
		TeamID:       "team-1",
	}}
}

func deploymentCallbackDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE deployments (
			id TEXT PRIMARY KEY,
			site_id TEXT,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO deployments (id, site_id, status, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)",
		"deployment-1",
		"site-1",
		sitetypes.DeploymentStatusPending,
	).Error)
	return db
}
