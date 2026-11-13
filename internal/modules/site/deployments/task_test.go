package deployments_test

import (
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/deployments"
)

func TestNewTaskPreservesQueueContract(t *testing.T) {
	redis := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	for _, tc := range []struct {
		name, taskType, taskID string
		zeroDowntime           bool
	}{
		{"standard", "site:deploy", "deploy:deployment-1", false},
		{"zero downtime", "site:deploy_zero_downtime", "deploy_zd:deployment-1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task, err := deployments.NewTask("site-1", "deployment-1", "user-1", map[string]string{"DEPLOY_MODE": "preview"}, tc.zeroDowntime)
			require.NoError(t, err)
			require.Equal(t, tc.taskType, task.Type())
			var payload map[string]any
			require.NoError(t, json.Unmarshal(task.Payload(), &payload))
			require.Equal(t, "site-1", payload["site_id"])
			require.Equal(t, "deployment-1", payload["deployment_id"])
			require.Equal(t, "user-1", payload["user_id"])
			require.Equal(t, map[string]any{"DEPLOY_MODE": "preview"}, payload["env_variables"])
			if tc.zeroDowntime {
				require.Equal(t, true, payload["zero_downtime"])
			} else {
				require.NotContains(t, payload, "zero_downtime")
			}
			info, err := client.Enqueue(task)
			require.NoError(t, err)
			require.Equal(t, tc.taskID, info.ID)
			require.Zero(t, info.MaxRetry)
			_, err = client.Enqueue(task)
			require.ErrorIs(t, err, asynq.ErrTaskIDConflict)
		})
	}
}

func TestNewTaskOmitsAbsentTransientValues(t *testing.T) {
	task, err := deployments.NewTask("site-1", "deployment-1", "", nil, false)
	require.NoError(t, err)
	require.JSONEq(t, `{"site_id":"site-1","deployment_id":"deployment-1"}`, string(task.Payload()))
}
