// Package types contains all type definitions for the notification module
package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// ChannelType
// =============================================================================

// ChannelType represents a notification channel type
type ChannelType string

const (
	ChannelTypeEmail    ChannelType = "email"
	ChannelTypeSlack    ChannelType = "slack"
	ChannelTypeDiscord  ChannelType = "discord"
	ChannelTypeTelegram ChannelType = "telegram"
)

var allChannelTypes = []ChannelType{
	ChannelTypeEmail,
	ChannelTypeSlack,
	ChannelTypeDiscord,
	ChannelTypeTelegram,
}

var channelTypeLabels = map[ChannelType]string{
	ChannelTypeEmail:    "Email",
	ChannelTypeSlack:    "Slack",
	ChannelTypeDiscord:  "Discord",
	ChannelTypeTelegram: "Telegram",
}

// AllChannelTypes returns all available channel types
func AllChannelTypes() []ChannelType {
	return allChannelTypes
}

// String returns the string value of the channel type
func (c ChannelType) String() string {
	return string(c)
}

// Label returns a human-readable label for the channel type
func (c ChannelType) Label() string {
	return enumtypes.Label(c, channelTypeLabels, string(c))
}

// IsValid checks if the channel type is valid
func (c ChannelType) IsValid() bool {
	return enumtypes.IsValid(c, allChannelTypes...)
}

// Value implements the driver.Valuer interface
func (c ChannelType) Value() (driver.Value, error) {
	return enumtypes.Value(c)
}

// Scan implements the sql.Scanner interface
func (c *ChannelType) Scan(value any) error {
	return enumtypes.Scan(c, value)
}

// ParseChannelType parses a string into a ChannelType
func ParseChannelType(s string) (ChannelType, error) {
	ct := ChannelType(s)
	if !ct.IsValid() {
		return "", fmt.Errorf("invalid channel type: %s", s)
	}

	return ct, nil
}

// =============================================================================
// NotificationType
// =============================================================================

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeServerProvisioned           NotificationType = "server_provisioned"
	NotificationTypeServerProvisioningFailed    NotificationType = "server_provisioning_failed"
	NotificationTypeServerConnectionLost        NotificationType = "server_connection_lost"
	NotificationTypeServerThresholdExceeded     NotificationType = "server_threshold_exceeded"
	NotificationTypeDeploymentFailed            NotificationType = "deployment_failed"
	NotificationTypeSiteInstallationFailed      NotificationType = "site_installation_failed"
	NotificationTypeJobOnServerFailed           NotificationType = "job_on_server_failed"
	NotificationTypePhpInstallationFailed       NotificationType = "php_installation_failed"
	NotificationTypePhpExtensionInstallFailed   NotificationType = "php_extension_install_failed"
	NotificationTypePhpExtensionUninstallFailed NotificationType = "php_extension_uninstall_failed"
	NotificationTypeVulnerabilityAuditCompleted NotificationType = "vulnerability_audit_completed"
	NotificationTypeFailedToDeleteServer        NotificationType = "failed_to_delete_server"
	NotificationTypeDatabaseBackupSucceeded     NotificationType = "database_backup_succeeded"
	NotificationTypeDatabaseBackupFailed        NotificationType = "database_backup_failed"
	NotificationTypeStaleProviderImages         NotificationType = "stale_provider_images"
	NotificationTypeGHAPermissionsMissing       NotificationType = "gha_permissions_missing"
	NotificationTypeQueueStopped                NotificationType = "queue_stopped"
)

var allNotificationTypes = []NotificationType{
	NotificationTypeServerProvisioned,
	NotificationTypeServerProvisioningFailed,
	NotificationTypeServerConnectionLost,
	NotificationTypeServerThresholdExceeded,
	NotificationTypeDeploymentFailed,
	NotificationTypeSiteInstallationFailed,
	NotificationTypeJobOnServerFailed,
	NotificationTypePhpInstallationFailed,
	NotificationTypePhpExtensionInstallFailed,
	NotificationTypePhpExtensionUninstallFailed,
	NotificationTypeVulnerabilityAuditCompleted,
	NotificationTypeFailedToDeleteServer,
	NotificationTypeDatabaseBackupSucceeded,
	NotificationTypeDatabaseBackupFailed,
	NotificationTypeStaleProviderImages,
	NotificationTypeGHAPermissionsMissing,
	NotificationTypeQueueStopped,
}

var notificationTypeLabels = map[NotificationType]string{
	NotificationTypeServerProvisioned:           "Server Provisioned",
	NotificationTypeServerProvisioningFailed:    "Server Provisioning Failed",
	NotificationTypeServerConnectionLost:        "Server Connection Lost",
	NotificationTypeServerThresholdExceeded:     "Server Threshold Exceeded",
	NotificationTypeDeploymentFailed:            "Deployment Failed",
	NotificationTypeSiteInstallationFailed:      "Site Installation Failed",
	NotificationTypeJobOnServerFailed:           "Job On Server Failed",
	NotificationTypePhpInstallationFailed:       "PHP Installation Failed",
	NotificationTypePhpExtensionInstallFailed:   "PHP Extension Install Failed",
	NotificationTypePhpExtensionUninstallFailed: "PHP Extension Uninstall Failed",
	NotificationTypeVulnerabilityAuditCompleted: "Vulnerability Audit Completed",
	NotificationTypeFailedToDeleteServer:        "Failed To Delete Server",
	NotificationTypeDatabaseBackupSucceeded:     "Database Backup Succeeded",
	NotificationTypeDatabaseBackupFailed:        "Database Backup Failed",
	NotificationTypeStaleProviderImages:         "Stale Provider Images",
	NotificationTypeGHAPermissionsMissing:       "GitHub App Missing Permissions",
	NotificationTypeQueueStopped:                "Queue Worker Stopped",
}

// AllNotificationTypes returns all valid notification types
func AllNotificationTypes() []NotificationType {
	return allNotificationTypes
}

// String returns the string value of the notification type
func (n NotificationType) String() string {
	return string(n)
}

// Label returns a human-readable label for the notification type
func (n NotificationType) Label() string {
	return enumtypes.Label(n, notificationTypeLabels, string(n))
}

// IsValid checks if the notification type is valid
func (n NotificationType) IsValid() bool {
	return enumtypes.IsValid(n, allNotificationTypes...)
}

// Value implements the driver.Valuer interface
func (n NotificationType) Value() (driver.Value, error) {
	return enumtypes.Value(n)
}

// Scan implements the sql.Scanner interface
func (n *NotificationType) Scan(value any) error {
	return enumtypes.Scan(n, value)
}
