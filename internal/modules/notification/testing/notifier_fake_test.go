package testing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/slack"
)

// mockNotification embeds BaseNotification to implement all interface methods
type mockNotification struct {
	*models.BaseNotification
}

func newMockNotification(notifType enums.NotificationType) models.Notification {
	return &mockNotification{
		BaseNotification: models.NewBaseNotification(notifType, "test notification"),
	}
}

func TestNewNotifierFake(t *testing.T) {
	fake := NewNotifierFake()

	require.NotNil(t, fake)
	assert.Empty(t, fake.GetSentNotifications())
	assert.Empty(t, fake.GetSentAdminAlerts())
}

func TestNotifierFake_SendToTeam(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()
	notif := newMockNotification(enums.NotificationTypeServerProvisioned)

	err := fake.SendToTeam(ctx, "team-123", notif)

	assert.NoError(t, err)
	assert.Equal(t, 1, fake.SendToTeamCalledTimes())

	notifications := fake.GetSentNotifications()
	require.Len(t, notifications, 1)
	assert.Equal(t, "team-123", notifications[0].TeamID)
	assert.Equal(t, notif, notifications[0].Notification)
}

func TestNotifierFake_SendToChannel(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()
	notif := newMockNotification(enums.NotificationTypeServerProvisioned)

	err := fake.SendToChannel(ctx, "channel-456", notif)

	assert.NoError(t, err)
	assert.Equal(t, 1, fake.SendToChannelCalledTimes())

	notifications := fake.GetSentNotifications()
	require.Len(t, notifications, 1)
	assert.Equal(t, "channel-456", notifications[0].ChannelID)
}

func TestNotifierFake_AssertSent(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	// No notifications sent yet
	assert.False(t, fake.AssertSent(enums.NotificationTypeServerProvisioned))

	// Send a notification
	notif := newMockNotification(enums.NotificationTypeServerProvisioned)
	_ = fake.SendToTeam(ctx, "team-123", notif)

	// Now it should be asserted as sent
	assert.True(t, fake.AssertSent(enums.NotificationTypeServerProvisioned))
	assert.False(t, fake.AssertSent(enums.NotificationTypeDeploymentFailed))
}

func TestNotifierFake_AssertSentTo(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	notif := newMockNotification(enums.NotificationTypeServerProvisioned)
	_ = fake.SendToTeam(ctx, "team-123", notif)

	assert.True(t, fake.AssertSentTo("team-123", enums.NotificationTypeServerProvisioned))
	assert.False(t, fake.AssertSentTo("team-456", enums.NotificationTypeServerProvisioned))
	assert.False(t, fake.AssertSentTo("team-123", enums.NotificationTypeDeploymentFailed))
}

func TestNotifierFake_AssertNotSent(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	assert.True(t, fake.AssertNotSent(enums.NotificationTypeServerProvisioned))

	notif := newMockNotification(enums.NotificationTypeServerProvisioned)
	_ = fake.SendToTeam(ctx, "team-123", notif)

	assert.False(t, fake.AssertNotSent(enums.NotificationTypeServerProvisioned))
	assert.True(t, fake.AssertNotSent(enums.NotificationTypeDeploymentFailed))
}

func TestNotifierFake_AssertNothingSent(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	assert.True(t, fake.AssertNothingSent())

	notif := newMockNotification(enums.NotificationTypeServerProvisioned)
	_ = fake.SendToTeam(ctx, "team-123", notif)

	assert.False(t, fake.AssertNothingSent())
}

func TestNotifierFake_AssertSentTimes(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	assert.True(t, fake.AssertSentTimes(enums.NotificationTypeServerProvisioned, 0))

	notif := newMockNotification(enums.NotificationTypeServerProvisioned)
	_ = fake.SendToTeam(ctx, "team-1", notif)
	_ = fake.SendToTeam(ctx, "team-2", notif)

	assert.True(t, fake.AssertSentTimes(enums.NotificationTypeServerProvisioned, 2))
	assert.False(t, fake.AssertSentTimes(enums.NotificationTypeServerProvisioned, 1))
}

func TestNotifierFake_SendServerProvisioningFailedAdminAlert(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	server := slack.ServerInfo{ID: "server-123", Name: "my-server", PublicIPv4: "192.168.1.1"}
	user := &slack.UserInfo{Name: "John", Email: "john@example.com"}

	err := fake.SendServerProvisioningFailedAdminAlert(ctx, server, user, "output", "error")

	assert.NoError(t, err)
	assert.True(t, fake.AssertAdminAlertSent("server_provisioning_failed"))

	alerts := fake.GetSentAdminAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "server_provisioning_failed", alerts[0].Type)
	assert.Equal(t, server, alerts[0].Server)
	assert.Equal(t, user, alerts[0].User)
	assert.Equal(t, "output", alerts[0].Output)
	assert.Equal(t, "error", alerts[0].ErrorMessage)
}

func TestNotifierFake_SendPhpInstallationFailedAdminAlert(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	server := slack.ServerInfo{ID: "server-123", Name: "my-server"}
	err := fake.SendPhpInstallationFailedAdminAlert(ctx, server, "8.2", nil, "output", "error")

	assert.NoError(t, err)
	assert.True(t, fake.AssertAdminAlertSent("php_installation_failed"))

	alerts := fake.GetSentAdminAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "8.2", alerts[0].ExtraData["php_version"])
}

func TestNotifierFake_SendPhpExtensionInstallFailedAdminAlert(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	server := slack.ServerInfo{ID: "server-123", Name: "my-server"}
	err := fake.SendPhpExtensionInstallFailedAdminAlert(ctx, server, "redis", "8.2", nil, "output", "error")

	assert.NoError(t, err)
	assert.True(t, fake.AssertAdminAlertSent("php_extension_install_failed"))

	alerts := fake.GetSentAdminAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "8.2", alerts[0].ExtraData["php_version"])
	assert.Equal(t, "redis", alerts[0].ExtraData["extension_name"])
}

func TestNotifierFake_SendPhpExtensionUninstallFailedAdminAlert(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	server := slack.ServerInfo{ID: "server-123", Name: "my-server"}
	err := fake.SendPhpExtensionUninstallFailedAdminAlert(ctx, server, "redis", "8.2", nil, "output", "error")

	assert.NoError(t, err)
	assert.True(t, fake.AssertAdminAlertSent("php_extension_uninstall_failed"))

	alerts := fake.GetSentAdminAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "8.2", alerts[0].ExtraData["php_version"])
	assert.Equal(t, "redis", alerts[0].ExtraData["extension_name"])
}

func TestNotifierFake_AssertAdminAlertSent(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	assert.False(t, fake.AssertAdminAlertSent("server_provisioning_failed"))

	server := slack.ServerInfo{ID: "server-123", Name: "my-server"}
	_ = fake.SendServerProvisioningFailedAdminAlert(ctx, server, nil, "", "")

	assert.True(t, fake.AssertAdminAlertSent("server_provisioning_failed"))
	assert.False(t, fake.AssertAdminAlertSent("php_installation_failed"))
}

func TestNotifierFake_Reset(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	notif := newMockNotification(enums.NotificationTypeServerProvisioned)
	_ = fake.SendToTeam(ctx, "team-123", notif)

	server := slack.ServerInfo{ID: "server-123", Name: "my-server"}
	_ = fake.SendServerProvisioningFailedAdminAlert(ctx, server, nil, "", "")

	assert.False(t, fake.AssertNothingSent())
	assert.Equal(t, 1, fake.SendToTeamCalledTimes())

	fake.Reset()

	assert.True(t, fake.AssertNothingSent())
	assert.Empty(t, fake.GetSentAdminAlerts())
	assert.Equal(t, 0, fake.SendToTeamCalledTimes())
	assert.Equal(t, 0, fake.SendToChannelCalledTimes())
}

func TestNotifierFake_ThreadSafety(t *testing.T) {
	fake := NewNotifierFake()
	ctx := context.Background()

	// Send multiple notifications concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			notif := newMockNotification(enums.NotificationTypeServerProvisioned)
			_ = fake.SendToTeam(ctx, "team-123", notif)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, fake.SendToTeamCalledTimes())
	assert.Len(t, fake.GetSentNotifications(), 10)
}
