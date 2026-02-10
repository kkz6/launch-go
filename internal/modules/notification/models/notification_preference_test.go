package models

import (
	"testing"

	"github.com/stretchr/testify/assert"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNotificationPreference_TableName(t *testing.T) {
	pref := NotificationPreference{}
	assert.Equal(t, "notification_preferences", pref.TableName())
}

func TestNotificationPreference_ShouldSend_WithToggledPreferences(t *testing.T) {
	tests := []struct {
		name     string
		pref     NotificationPreference
		notifTyp notificationtypes.NotificationType
		expected bool
	}{
		{
			name:     "server created enabled",
			pref:     NotificationPreference{EmailServerCreated: true},
			notifTyp: notificationtypes.NotificationTypeServerProvisioned,
			expected: true,
		},
		{
			name:     "server created disabled",
			pref:     NotificationPreference{EmailServerCreated: false},
			notifTyp: notificationtypes.NotificationTypeServerProvisioned,
			expected: false,
		},
		{
			name:     "server deleted enabled",
			pref:     NotificationPreference{EmailServerDeleted: true},
			notifTyp: notificationtypes.NotificationTypeFailedToDeleteServer,
			expected: true,
		},
		{
			name:     "server deleted disabled",
			pref:     NotificationPreference{EmailServerDeleted: false},
			notifTyp: notificationtypes.NotificationTypeFailedToDeleteServer,
			expected: false,
		},
		{
			name:     "deployment failed enabled",
			pref:     NotificationPreference{EmailDeploymentFailed: true},
			notifTyp: notificationtypes.NotificationTypeDeploymentFailed,
			expected: true,
		},
		{
			name:     "deployment failed disabled",
			pref:     NotificationPreference{EmailDeploymentFailed: false},
			notifTyp: notificationtypes.NotificationTypeDeploymentFailed,
			expected: false,
		},
		{
			name:     "site installation failed maps to deployment failed toggle",
			pref:     NotificationPreference{EmailDeploymentFailed: true},
			notifTyp: notificationtypes.NotificationTypeSiteInstallationFailed,
			expected: true,
		},
		{
			name:     "site installation failed disabled",
			pref:     NotificationPreference{EmailDeploymentFailed: false},
			notifTyp: notificationtypes.NotificationTypeSiteInstallationFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pref.ShouldSend(tt.notifTyp)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNotificationPreference_ShouldSend_AlwaysSentTypes(t *testing.T) {
	// These types have no preference toggle and should always be sent,
	// regardless of preference settings
	alwaysSentTypes := []notificationtypes.NotificationType{
		notificationtypes.NotificationTypeServerProvisioningFailed,
		notificationtypes.NotificationTypeServerConnectionLost,
		notificationtypes.NotificationTypeServerThresholdExceeded,
		notificationtypes.NotificationTypeVulnerabilityAuditCompleted,
		notificationtypes.NotificationTypePhpInstallationFailed,
		notificationtypes.NotificationTypePhpExtensionInstallFailed,
		notificationtypes.NotificationTypePhpExtensionUninstallFailed,
		notificationtypes.NotificationTypeJobOnServerFailed,
	}

	// All toggles disabled
	pref := NotificationPreference{
		EmailServerCreated:     false,
		EmailServerDeleted:     false,
		EmailDeploymentSuccess: false,
		EmailDeploymentFailed:  false,
		EmailBackupSuccess:     false,
		EmailBackupFailed:      false,
	}

	for _, notifType := range alwaysSentTypes {
		t.Run(notifType.String(), func(t *testing.T) {
			assert.True(t, pref.ShouldSend(notifType),
				"expected %s to always be sent regardless of preferences", notifType)
		})
	}
}

func TestNotificationPreference_ShouldSend_UnknownType(t *testing.T) {
	pref := NotificationPreference{}
	assert.True(t, pref.ShouldSend(notificationtypes.NotificationType("unknown_type")))
}

func TestNotificationPreference_Defaults(t *testing.T) {
	pref := NotificationPreference{
		EmailServerCreated:     true,
		EmailServerDeleted:     true,
		EmailDeploymentSuccess: false,
		EmailDeploymentFailed:  true,
		EmailBackupSuccess:     false,
		EmailBackupFailed:      true,
	}

	assert.True(t, pref.ShouldSend(notificationtypes.NotificationTypeServerProvisioned))
	assert.True(t, pref.ShouldSend(notificationtypes.NotificationTypeFailedToDeleteServer))
	assert.True(t, pref.ShouldSend(notificationtypes.NotificationTypeDeploymentFailed))
	assert.True(t, pref.ShouldSend(notificationtypes.NotificationTypeVulnerabilityAuditCompleted))
}
