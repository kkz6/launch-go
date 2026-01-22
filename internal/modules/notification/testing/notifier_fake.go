package testing

import (
	"context"
	"sync"

	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/slack"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// withLockReturn is a generic helper that executes a function while holding a lock
// and returns the result. This reduces boilerplate in test fakes that need thread-safe
// access to internal state.
func withLockReturn[T any](mu *sync.Mutex, fn func() T) T {
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// SentNotification represents a notification that was sent
type SentNotification struct {
	TeamID       string
	ChannelID    string
	Notification models.Notification
}

// SentAdminAlert represents an admin alert that was sent
type SentAdminAlert struct {
	Type         string
	Server       slack.ServerInfo
	User         *slack.UserInfo
	Output       string
	ErrorMessage string
	ExtraData    map[string]string
}

// NotifierFake is a fake notifier for testing
type NotifierFake struct {
	mu                  sync.Mutex
	sentNotifications   []SentNotification
	sentAdminAlerts     []SentAdminAlert
	sendToTeamCalled    int
	sendToChannelCalled int
}

// NewNotifierFake creates a new fake notifier
func NewNotifierFake() *NotifierFake {
	return &NotifierFake{
		sentNotifications: []SentNotification{},
		sentAdminAlerts:   []SentAdminAlert{},
	}
}

// withLock executes a function while holding the mutex lock.
// This reduces lock/unlock boilerplate in methods that don't return values.
func (n *NotifierFake) withLock(fn func()) {
	n.mu.Lock()
	defer n.mu.Unlock()
	fn()
}

// SendToTeam records a notification sent to a team
func (n *NotifierFake) SendToTeam(ctx context.Context, teamID string, notification models.Notification) error {
	n.withLock(func() {
		n.sendToTeamCalled++
		n.sentNotifications = append(n.sentNotifications, SentNotification{
			TeamID:       teamID,
			Notification: notification,
		})
	})
	return nil
}

// SendToChannel records a notification sent to a channel
func (n *NotifierFake) SendToChannel(ctx context.Context, channelID string, notification models.Notification) error {
	n.withLock(func() {
		n.sendToChannelCalled++
		n.sentNotifications = append(n.sentNotifications, SentNotification{
			ChannelID:    channelID,
			Notification: notification,
		})
	})
	return nil
}

// SendServerProvisioningFailedAdminAlert records an admin alert
func (n *NotifierFake) SendServerProvisioningFailedAdminAlert(ctx context.Context, server slack.ServerInfo, user *slack.UserInfo, output, errorMessage string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.sentAdminAlerts = append(n.sentAdminAlerts, SentAdminAlert{
		Type:         "server_provisioning_failed",
		Server:       server,
		User:         user,
		Output:       output,
		ErrorMessage: errorMessage,
	})
	return nil
}

// SendPhpInstallationFailedAdminAlert records an admin alert
func (n *NotifierFake) SendPhpInstallationFailedAdminAlert(ctx context.Context, server slack.ServerInfo, phpVersion string, user *slack.UserInfo, output, errorMessage string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.sentAdminAlerts = append(n.sentAdminAlerts, SentAdminAlert{
		Type:         "php_installation_failed",
		Server:       server,
		User:         user,
		Output:       output,
		ErrorMessage: errorMessage,
		ExtraData:    map[string]string{"php_version": phpVersion},
	})
	return nil
}

// SendPhpExtensionInstallFailedAdminAlert records an admin alert
func (n *NotifierFake) SendPhpExtensionInstallFailedAdminAlert(ctx context.Context, server slack.ServerInfo, extensionName, phpVersion string, user *slack.UserInfo, output, errorMessage string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.sentAdminAlerts = append(n.sentAdminAlerts, SentAdminAlert{
		Type:         "php_extension_install_failed",
		Server:       server,
		User:         user,
		Output:       output,
		ErrorMessage: errorMessage,
		ExtraData:    map[string]string{"php_version": phpVersion, "extension_name": extensionName},
	})
	return nil
}

// SendPhpExtensionUninstallFailedAdminAlert records an admin alert
func (n *NotifierFake) SendPhpExtensionUninstallFailedAdminAlert(ctx context.Context, server slack.ServerInfo, extensionName, phpVersion string, user *slack.UserInfo, output, errorMessage string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.sentAdminAlerts = append(n.sentAdminAlerts, SentAdminAlert{
		Type:         "php_extension_uninstall_failed",
		Server:       server,
		User:         user,
		Output:       output,
		ErrorMessage: errorMessage,
		ExtraData:    map[string]string{"php_version": phpVersion, "extension_name": extensionName},
	})
	return nil
}

// AssertSent asserts that a notification of the given type was sent
func (n *NotifierFake) AssertSent(notificationType notificationtypes.NotificationType) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, sent := range n.sentNotifications {
		if sent.Notification.Type() == notificationType {
			return true
		}
	}
	return false
}

// AssertSentTo asserts that a notification of the given type was sent to a specific team
func (n *NotifierFake) AssertSentTo(teamID string, notificationType notificationtypes.NotificationType) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, sent := range n.sentNotifications {
		if sent.TeamID == teamID && sent.Notification.Type() == notificationType {
			return true
		}
	}
	return false
}

// AssertNotSent asserts that a notification of the given type was NOT sent
func (n *NotifierFake) AssertNotSent(notificationType notificationtypes.NotificationType) bool {
	return !n.AssertSent(notificationType)
}

// AssertNothingSent asserts that no notifications were sent
func (n *NotifierFake) AssertNothingSent() bool {
	return withLockReturn(&n.mu, func() bool {
		return len(n.sentNotifications) == 0
	})
}

// AssertSentTimes asserts that a notification of the given type was sent a specific number of times
func (n *NotifierFake) AssertSentTimes(notificationType notificationtypes.NotificationType, times int) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	count := 0
	for _, sent := range n.sentNotifications {
		if sent.Notification.Type() == notificationType {
			count++
		}
	}
	return count == times
}

// AssertAdminAlertSent asserts that an admin alert of the given type was sent
func (n *NotifierFake) AssertAdminAlertSent(alertType string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, sent := range n.sentAdminAlerts {
		if sent.Type == alertType {
			return true
		}
	}
	return false
}

// GetSentNotifications returns all sent notifications
func (n *NotifierFake) GetSentNotifications() []SentNotification {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make([]SentNotification, len(n.sentNotifications))
	copy(result, n.sentNotifications)
	return result
}

// GetSentAdminAlerts returns all sent admin alerts
func (n *NotifierFake) GetSentAdminAlerts() []SentAdminAlert {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make([]SentAdminAlert, len(n.sentAdminAlerts))
	copy(result, n.sentAdminAlerts)
	return result
}

// Reset clears all sent notifications and alerts
func (n *NotifierFake) Reset() {
	n.withLock(func() {
		n.sentNotifications = []SentNotification{}
		n.sentAdminAlerts = []SentAdminAlert{}
		n.sendToTeamCalled = 0
		n.sendToChannelCalled = 0
	})
}

// SendToTeamCalledTimes returns how many times SendToTeam was called
func (n *NotifierFake) SendToTeamCalledTimes() int {
	return withLockReturn(&n.mu, func() int {
		return n.sendToTeamCalled
	})
}

// SendToChannelCalledTimes returns how many times SendToChannel was called
func (n *NotifierFake) SendToChannelCalledTimes() int {
	return withLockReturn(&n.mu, func() int {
		return n.sendToChannelCalled
	})
}
