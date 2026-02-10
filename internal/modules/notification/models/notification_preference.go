package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// NotificationPreference stores per-team email notification toggles
type NotificationPreference struct {
	basemodels.BaseModel
	TeamID                 string `gorm:"column:team_id;type:char(26);not null;uniqueIndex" json:"team_id"`
	EmailServerCreated     bool   `gorm:"column:email_server_created;default:true" json:"email_server_created"`
	EmailServerDeleted     bool   `gorm:"column:email_server_deleted;default:true" json:"email_server_deleted"`
	EmailDeploymentSuccess bool   `gorm:"column:email_deployment_success;default:false" json:"email_deployment_success"`
	EmailDeploymentFailed  bool   `gorm:"column:email_deployment_failed;default:true" json:"email_deployment_failed"`
	EmailBackupSuccess     bool   `gorm:"column:email_backup_success;default:false" json:"email_backup_success"`
	EmailBackupFailed      bool   `gorm:"column:email_backup_failed;default:true" json:"email_backup_failed"`
}

// TableName returns the table name for GORM
func (NotificationPreference) TableName() string {
	return "notification_preferences"
}

// ShouldSend returns whether a notification of the given type should be sent
// based on this team's preferences. Types without a preference toggle are always sent.
func (p *NotificationPreference) ShouldSend(notifType notificationtypes.NotificationType) bool {
	switch notifType {
	case notificationtypes.NotificationTypeServerProvisioned:
		return p.EmailServerCreated
	case notificationtypes.NotificationTypeFailedToDeleteServer:
		return p.EmailServerDeleted
	case notificationtypes.NotificationTypeDeploymentFailed,
		notificationtypes.NotificationTypeSiteInstallationFailed:
		return p.EmailDeploymentFailed
	default:
		// Types without a preference toggle are always sent:
		// ServerProvisioningFailed, ServerConnectionLost, ServerThresholdExceeded,
		// VulnerabilityAuditCompleted, PhpInstallationFailed, PhpExtensionInstallFailed,
		// PhpExtensionUninstallFailed, JobOnServerFailed
		return true
	}
}
