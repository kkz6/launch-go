package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

func TestToNotificationPreferencesResponse(t *testing.T) {
	pref := &models.NotificationPreference{
		EmailServerCreated:     true,
		EmailServerDeleted:     false,
		EmailDeploymentSuccess: true,
		EmailDeploymentFailed:  false,
		EmailBackupSuccess:     true,
		EmailBackupFailed:      false,
	}

	resp := ToNotificationPreferencesResponse(pref)

	assert.True(t, resp.EmailServerCreated)
	assert.False(t, resp.EmailServerDeleted)
	assert.True(t, resp.EmailDeploymentSuccess)
	assert.False(t, resp.EmailDeploymentFailed)
	assert.True(t, resp.EmailBackupSuccess)
	assert.False(t, resp.EmailBackupFailed)
}

func TestToNotificationPreferencesResponse_AllDefaults(t *testing.T) {
	pref := &models.NotificationPreference{
		EmailServerCreated:     true,
		EmailServerDeleted:     true,
		EmailDeploymentSuccess: false,
		EmailDeploymentFailed:  true,
		EmailBackupSuccess:     false,
		EmailBackupFailed:      true,
	}

	resp := ToNotificationPreferencesResponse(pref)

	assert.True(t, resp.EmailServerCreated)
	assert.True(t, resp.EmailServerDeleted)
	assert.False(t, resp.EmailDeploymentSuccess)
	assert.True(t, resp.EmailDeploymentFailed)
	assert.False(t, resp.EmailBackupSuccess)
	assert.True(t, resp.EmailBackupFailed)
}
