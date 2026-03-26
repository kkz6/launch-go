package jobs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnableLaravelReverbTask(t *testing.T) {
	siteID := "site_01JTEST00000000000000001"
	serverID := "srv_01JTEST00000000000000001"
	userID := "usr_01JTEST00000000000000001"

	task, err := NewEnableLaravelReverbTask(siteID, serverID, &userID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeEnableLaravelReverb, task.Type())

	var payload EnableLaravelReverbPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	require.NotNil(t, payload.UserID)
	assert.Equal(t, userID, *payload.UserID)
	assert.False(t, payload.ConfigureEnv)
}

func TestNewEnableLaravelReverbTask_NilUserID(t *testing.T) {
	siteID := "site_01JTEST00000000000000002"
	serverID := "srv_01JTEST00000000000000002"

	task, err := NewEnableLaravelReverbTask(siteID, serverID, nil)
	require.NoError(t, err)
	require.NotNil(t, task)

	var payload EnableLaravelReverbPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Nil(t, payload.UserID)
}

func TestNewEnableLaravelReverbTaskWithOptions(t *testing.T) {
	siteID := "site_01JTEST00000000000000003"
	serverID := "srv_01JTEST00000000000000003"
	userID := "usr_01JTEST00000000000000003"

	task, err := NewEnableLaravelReverbTaskWithOptions(siteID, serverID, &userID, true)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeEnableLaravelReverb, task.Type())

	var payload EnableLaravelReverbPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	require.NotNil(t, payload.UserID)
	assert.Equal(t, userID, *payload.UserID)
	assert.True(t, payload.ConfigureEnv)
}

func TestNewEnableLaravelReverbTaskWithOptions_ConfigureEnvFalse(t *testing.T) {
	siteID := "site_01JTEST00000000000000004"
	serverID := "srv_01JTEST00000000000000004"

	task, err := NewEnableLaravelReverbTaskWithOptions(siteID, serverID, nil, false)
	require.NoError(t, err)
	require.NotNil(t, task)

	var payload EnableLaravelReverbPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.False(t, payload.ConfigureEnv)
	assert.Nil(t, payload.UserID)
}

func TestNewDisableLaravelReverbTask(t *testing.T) {
	siteID := "site_01JTEST00000000000000005"
	serverID := "srv_01JTEST00000000000000005"
	userID := "usr_01JTEST00000000000000005"

	task, err := NewDisableLaravelReverbTask(siteID, serverID, &userID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeDisableLaravelReverb, task.Type())

	var payload DisableLaravelReverbPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	require.NotNil(t, payload.UserID)
	assert.Equal(t, userID, *payload.UserID)
}

func TestNewDisableLaravelReverbTask_NilUserID(t *testing.T) {
	siteID := "site_01JTEST00000000000000006"
	serverID := "srv_01JTEST00000000000000006"

	task, err := NewDisableLaravelReverbTask(siteID, serverID, nil)
	require.NoError(t, err)
	require.NotNil(t, task)

	var payload DisableLaravelReverbPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Nil(t, payload.UserID)
}

func TestReverbTaskTypeConstants(t *testing.T) {
	assert.Equal(t, "site:enable_laravel_reverb", TypeEnableLaravelReverb)
	assert.Equal(t, "site:disable_laravel_reverb", TypeDisableLaravelReverb)
}

func TestFeatureReverbConstant(t *testing.T) {
	assert.Equal(t, "reverb", FeatureReverb)
}

func TestEnableLaravelReverbPayload_JSON(t *testing.T) {
	userID := "user-123"
	payload := EnableLaravelReverbPayload{
		SiteID:       "site-123",
		ServerID:     "server-123",
		UserID:       &userID,
		ConfigureEnv: true,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded EnableLaravelReverbPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.SiteID, decoded.SiteID)
	assert.Equal(t, payload.ServerID, decoded.ServerID)
	require.NotNil(t, decoded.UserID)
	assert.Equal(t, userID, *decoded.UserID)
	assert.True(t, decoded.ConfigureEnv)
}

func TestDisableLaravelReverbPayload_JSON(t *testing.T) {
	payload := DisableLaravelReverbPayload{
		SiteID:   "site-456",
		ServerID: "server-456",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded DisableLaravelReverbPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.SiteID, decoded.SiteID)
	assert.Equal(t, payload.ServerID, decoded.ServerID)
	assert.Nil(t, decoded.UserID)
}

func TestEnableLaravelReverbPayload_ConfigureEnv_OmittedWhenFalse(t *testing.T) {
	payload := EnableLaravelReverbPayload{
		SiteID:       "site-789",
		ServerID:     "server-789",
		ConfigureEnv: false,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "configure_env")
}
