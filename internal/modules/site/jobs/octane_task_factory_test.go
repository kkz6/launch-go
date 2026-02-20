package jobs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnableLaravelOctaneTask(t *testing.T) {
	siteID := "site_01JTEST00000000000000001"
	serverID := "srv_01JTEST00000000000000001"
	userID := "usr_01JTEST00000000000000001"

	task, err := NewEnableLaravelOctaneTask(siteID, serverID, &userID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeEnableLaravelOctane, task.Type())

	var payload EnableLaravelOctanePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	require.NotNil(t, payload.UserID)
	assert.Equal(t, userID, *payload.UserID)
}

func TestNewEnableLaravelOctaneTask_NilUserID(t *testing.T) {
	siteID := "site_01JTEST00000000000000002"
	serverID := "srv_01JTEST00000000000000002"

	task, err := NewEnableLaravelOctaneTask(siteID, serverID, nil)
	require.NoError(t, err)
	require.NotNil(t, task)

	var payload EnableLaravelOctanePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Nil(t, payload.UserID)
}

func TestNewDisableLaravelOctaneTask(t *testing.T) {
	siteID := "site_01JTEST00000000000000003"
	serverID := "srv_01JTEST00000000000000003"
	userID := "usr_01JTEST00000000000000003"

	task, err := NewDisableLaravelOctaneTask(siteID, serverID, &userID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeDisableLaravelOctane, task.Type())

	var payload DisableLaravelOctanePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	require.NotNil(t, payload.UserID)
	assert.Equal(t, userID, *payload.UserID)
}

func TestNewDisableLaravelOctaneTask_NilUserID(t *testing.T) {
	siteID := "site_01JTEST00000000000000004"
	serverID := "srv_01JTEST00000000000000004"

	task, err := NewDisableLaravelOctaneTask(siteID, serverID, nil)
	require.NoError(t, err)
	require.NotNil(t, task)

	var payload DisableLaravelOctanePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, siteID, payload.SiteID)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Nil(t, payload.UserID)
}

func TestOctaneTaskTypeConstants(t *testing.T) {
	assert.Equal(t, "site:enable_laravel_octane", TypeEnableLaravelOctane)
	assert.Equal(t, "site:disable_laravel_octane", TypeDisableLaravelOctane)
}

func TestEnableLaravelOctanePayload_JSON(t *testing.T) {
	userID := "user-123"
	payload := EnableLaravelOctanePayload{
		SiteID:   "site-123",
		ServerID: "server-123",
		UserID:   &userID,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded EnableLaravelOctanePayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.SiteID, decoded.SiteID)
	assert.Equal(t, payload.ServerID, decoded.ServerID)
	require.NotNil(t, decoded.UserID)
	assert.Equal(t, userID, *decoded.UserID)
}

func TestDisableLaravelOctanePayload_JSON(t *testing.T) {
	payload := DisableLaravelOctanePayload{
		SiteID:   "site-456",
		ServerID: "server-456",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded DisableLaravelOctanePayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.SiteID, decoded.SiteID)
	assert.Equal(t, payload.ServerID, decoded.ServerID)
	assert.Nil(t, decoded.UserID)
}

func TestFeatureOctaneConstant(t *testing.T) {
	assert.Equal(t, "octane", FeatureOctane)
}
