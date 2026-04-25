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
	switch c {
	case ChannelTypeEmail:
		return "Email"
	case ChannelTypeSlack:
		return "Slack"
	case ChannelTypeDiscord:
		return "Discord"
	case ChannelTypeTelegram:
		return "Telegram"
	default:
		return string(c)
	}
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
	switch n {
	case NotificationTypeServerProvisioned:
		return "Server Provisioned"
	case NotificationTypeServerProvisioningFailed:
		return "Server Provisioning Failed"
	case NotificationTypeServerConnectionLost:
		return "Server Connection Lost"
	case NotificationTypeServerThresholdExceeded:
		return "Server Threshold Exceeded"
	case NotificationTypeDeploymentFailed:
		return "Deployment Failed"
	case NotificationTypeSiteInstallationFailed:
		return "Site Installation Failed"
	case NotificationTypeJobOnServerFailed:
		return "Job On Server Failed"
	case NotificationTypePhpInstallationFailed:
		return "PHP Installation Failed"
	case NotificationTypePhpExtensionInstallFailed:
		return "PHP Extension Install Failed"
	case NotificationTypePhpExtensionUninstallFailed:
		return "PHP Extension Uninstall Failed"
	case NotificationTypeVulnerabilityAuditCompleted:
		return "Vulnerability Audit Completed"
	case NotificationTypeFailedToDeleteServer:
		return "Failed To Delete Server"
	default:
		return string(n)
	}
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
