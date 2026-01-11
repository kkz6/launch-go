package enums

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

// String returns the string value of the notification type
func (n NotificationType) String() string {
	return string(n)
}

// IsValid checks if the notification type is valid
func (n NotificationType) IsValid() bool {
	switch n {
	case NotificationTypeServerProvisioned,
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
		NotificationTypeFailedToDeleteServer:
		return true
	}

	return false
}
