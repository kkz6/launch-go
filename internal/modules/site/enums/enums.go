package enums

import (
	"database/sql/driver"
	"fmt"
)

// SiteType represents the type of site (Laravel, WordPress, Static, Generic)
type SiteType string

const (
	SiteTypeLaravel   SiteType = "laravel"
	SiteTypeWordpress SiteType = "wordpress"
	SiteTypeStatic    SiteType = "static"
	SiteTypeGeneric   SiteType = "generic"
)

func (s SiteType) String() string {
	return string(s)
}

func (s SiteType) Label() string {
	labels := map[SiteType]string{
		SiteTypeLaravel:   "Laravel",
		SiteTypeWordpress: "Wordpress",
		SiteTypeStatic:    "Static",
		SiteTypeGeneric:   "Generic",
	}

	if label, ok := labels[s]; ok {
		return label
	}

	return string(s)
}

func (s SiteType) IsValid() bool {
	switch s {
	case SiteTypeLaravel, SiteTypeWordpress, SiteTypeStatic, SiteTypeGeneric:
		return true
	}

	return false
}

func (s SiteType) HasEnvironment() bool {
	switch s {
	case SiteTypeLaravel, SiteTypeWordpress:
		return true
	}

	return false
}

func (s SiteType) RequiresGitAccount() bool {
	return s != SiteTypeWordpress
}

// GetDefaultWebFolder returns the default web folder for the site type
func (s SiteType) GetDefaultWebFolder() string {
	if s == SiteTypeWordpress {
		return "/"
	}
	return "public"
}

func (s *SiteType) Scan(value interface{}) error {
	if value == nil {
		*s = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan SiteType: %v", value)
		}
		str = string(bytes)
	}

	*s = SiteType(str)

	return nil
}

func (s SiteType) Value() (driver.Value, error) {
	return string(s), nil
}

// GetDatabaseEnvVarNames returns environment variable names for database configuration
func (s SiteType) GetDatabaseEnvVarNames() map[string]string {
	if s == SiteTypeWordpress {
		return map[string]string{
			"database":   "DB_NAME",
			"username":   "DB_USER",
			"password":   "DB_PASSWORD",
			"host":       "DB_HOST",
			"port":       "",
			"connection": "",
		}
	}

	return map[string]string{
		"database":   "DB_DATABASE",
		"username":   "DB_USERNAME",
		"password":   "DB_PASSWORD",
		"host":       "DB_HOST",
		"port":       "DB_PORT",
		"connection": "DB_CONNECTION",
	}
}

// GetDefaultAttributes returns default configuration for the site type
func (s SiteType) GetDefaultAttributes(zeroDowntime bool) map[string]interface{} {
	switch s {
	case SiteTypeLaravel:
		return s.laravelDefaults(zeroDowntime)
	case SiteTypeStatic:
		return s.staticDefaults(zeroDowntime)
	case SiteTypeWordpress:
		return s.wordpressDefaults()
	default:
		return map[string]interface{}{}
	}
}

func (s SiteType) laravelDefaults(zeroDowntime bool) map[string]interface{} {
	installScript := `composer install --no-dev --no-interaction --prefer-dist --optimize-autoloader
#npm install --prefer-offline --no-audit
#npm run build
$PHP_BINARY artisan storage:link
$PHP_BINARY artisan config:cache
$PHP_BINARY artisan route:cache
$PHP_BINARY artisan view:cache
$PHP_BINARY artisan event:cache
# $PHP_BINARY artisan migrate --force`

	if !zeroDowntime {
		installScript += "\n$PHP_BINARY artisan up"
	}

	defaults := map[string]interface{}{
		"shared_directories": []string{"storage"},
		"shared_files":       []string{".env"},
		"writeable_directories": []string{
			"bootstrap/cache",
			"storage",
			"storage/app",
			"storage/app/public",
			"storage/framework",
			"storage/framework/cache",
			"storage/framework/sessions",
			"storage/framework/views",
			"storage/logs",
		},
	}

	if zeroDowntime {
		defaults["hook_before_updating_repository"] = ""
		defaults["hook_after_updating_repository"] = ""
		defaults["hook_before_making_current"] = installScript
		defaults["hook_after_making_current"] = ""
	} else {
		defaults["hook_before_updating_repository"] = "$PHP_BINARY artisan down"
		defaults["hook_after_updating_repository"] = installScript
		defaults["hook_before_making_current"] = ""
		defaults["hook_after_making_current"] = ""
	}

	return defaults
}

func (s SiteType) staticDefaults(zeroDowntime bool) map[string]interface{} {
	script := `# npm install --prefer-offline --no-audit
# npm run build`

	if zeroDowntime {
		return map[string]interface{}{
			"hook_before_making_current": script,
		}
	}

	return map[string]interface{}{
		"hook_after_updating_repository": script,
	}
}

func (s SiteType) wordpressDefaults() map[string]interface{} {
	return map[string]interface{}{
		"type":                            SiteTypeWordpress,
		"tls_setting":                     TlsSettingAuto,
		"web_folder":                      "/",
		"zero_downtime_deployment":        false,
		"source_control_repositories_id":  nil,
		"repository_branch":               "main",
		"deploy_notification_email":       nil,
		"shared_directories":              []string{},
		"shared_files":                    []string{},
		"writeable_directories":           []string{},
		"hook_before_updating_repository": "",
		"hook_after_updating_repository":  "",
		"hook_before_making_current":      "",
		"hook_after_making_current":       "",
	}
}

// ParseSiteType parses a string into a SiteType
func ParseSiteType(s string) (SiteType, error) {
	st := SiteType(s)
	if !st.IsValid() {
		return "", fmt.Errorf("invalid site type: %s", s)
	}

	return st, nil
}

// AllSiteTypes returns all valid site types
func AllSiteTypes() []SiteType {
	return []SiteType{
		SiteTypeLaravel,
		SiteTypeWordpress,
		SiteTypeStatic,
		SiteTypeGeneric,
	}
}

// SiteStatus represents the installation status of a site
type SiteStatus string

const (
	SiteStatusPending      SiteStatus = "pending"
	SiteStatusInstalling   SiteStatus = "installing"
	SiteStatusActive       SiteStatus = "active"
	SiteStatusFailed       SiteStatus = "failed"
	SiteStatusUninstalling SiteStatus = "uninstalling"
)

func (s SiteStatus) String() string {
	return string(s)
}

func (s SiteStatus) IsValid() bool {
	switch s {
	case SiteStatusPending, SiteStatusInstalling, SiteStatusActive, SiteStatusFailed, SiteStatusUninstalling:
		return true
	}

	return false
}

func (s *SiteStatus) Scan(value interface{}) error {
	if value == nil {
		*s = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan SiteStatus: %v", value)
		}
		str = string(bytes)
	}

	*s = SiteStatus(str)

	return nil
}

func (s SiteStatus) Value() (driver.Value, error) {
	return string(s), nil
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusQueued     DeploymentStatus = "queued"
	DeploymentStatusInstalling DeploymentStatus = "installing"
	DeploymentStatusFinished   DeploymentStatus = "finished"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusTimeout    DeploymentStatus = "timeout"
)

func (d DeploymentStatus) String() string {
	return string(d)
}

func (d DeploymentStatus) Label() string {
	labels := map[DeploymentStatus]string{
		DeploymentStatusPending:    "Pending",
		DeploymentStatusQueued:     "Queued",
		DeploymentStatusInstalling: "Installing",
		DeploymentStatusFinished:   "Finished",
		DeploymentStatusFailed:     "Failed",
		DeploymentStatusTimeout:    "Timeout",
	}

	if label, ok := labels[d]; ok {
		return label
	}

	return string(d)
}

func (d DeploymentStatus) IsValid() bool {
	switch d {
	case DeploymentStatusPending, DeploymentStatusQueued, DeploymentStatusInstalling,
		DeploymentStatusFinished, DeploymentStatusFailed, DeploymentStatusTimeout:
		return true
	}

	return false
}

func (d DeploymentStatus) IsActive() bool {
	return d == DeploymentStatusPending || d == DeploymentStatusInstalling
}

func (d DeploymentStatus) IsComplete() bool {
	return d == DeploymentStatusFinished || d == DeploymentStatusFailed || d == DeploymentStatusTimeout
}

func (d *DeploymentStatus) Scan(value interface{}) error {
	if value == nil {
		*d = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan DeploymentStatus: %v", value)
		}
		str = string(bytes)
	}

	*d = DeploymentStatus(str)

	return nil
}

func (d DeploymentStatus) Value() (driver.Value, error) {
	return string(d), nil
}

// AllDeploymentStatuses returns all valid deployment statuses
func AllDeploymentStatuses() []DeploymentStatus {
	return []DeploymentStatus{
		DeploymentStatusPending,
		DeploymentStatusQueued,
		DeploymentStatusInstalling,
		DeploymentStatusFinished,
		DeploymentStatusFailed,
		DeploymentStatusTimeout,
	}
}

// TlsSetting represents the TLS/SSL configuration for a site
type TlsSetting string

const (
	TlsSettingAuto     TlsSetting = "auto"
	TlsSettingCustom   TlsSetting = "custom"
	TlsSettingInternal TlsSetting = "internal"
	TlsSettingOff      TlsSetting = "off"
)

func (t TlsSetting) String() string {
	return string(t)
}

func (t TlsSetting) Label() string {
	labels := map[TlsSetting]string{
		TlsSettingAuto:     "Auto",
		TlsSettingCustom:   "Custom",
		TlsSettingInternal: "Internal",
		TlsSettingOff:      "Off",
	}

	if label, ok := labels[t]; ok {
		return label
	}

	return string(t)
}

func (t TlsSetting) IsValid() bool {
	switch t {
	case TlsSettingAuto, TlsSettingCustom, TlsSettingInternal, TlsSettingOff:
		return true
	}

	return false
}

func (t TlsSetting) IsEnabled() bool {
	return t != TlsSettingOff
}

func (t TlsSetting) GetPort() int {
	if t == TlsSettingOff {
		return 80
	}

	return 443
}

func (t TlsSetting) GetProtocol() string {
	if t == TlsSettingOff {
		return "http"
	}

	return "https"
}

func (t *TlsSetting) Scan(value interface{}) error {
	if value == nil {
		*t = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan TlsSetting: %v", value)
		}
		str = string(bytes)
	}

	*t = TlsSetting(str)

	return nil
}

func (t TlsSetting) Value() (driver.Value, error) {
	return string(t), nil
}

// AllTlsSettings returns all valid TLS settings
func AllTlsSettings() []TlsSetting {
	return []TlsSetting{
		TlsSettingAuto,
		TlsSettingCustom,
		TlsSettingInternal,
		TlsSettingOff,
	}
}

// RedirectMode represents the type of HTTP redirect
type RedirectMode int

const (
	RedirectModePermanent RedirectMode = 1
	RedirectModeTemporary RedirectMode = 2
)

func (r RedirectMode) StatusCode() int {
	switch r {
	case RedirectModePermanent:
		return 301
	case RedirectModeTemporary:
		return 302
	default:
		return 302
	}
}

func (r RedirectMode) Label() string {
	switch r {
	case RedirectModePermanent:
		return "Permanent"
	case RedirectModeTemporary:
		return "Temporary"
	default:
		return "Unknown"
	}
}

func (r RedirectMode) IsValid() bool {
	return r == RedirectModePermanent || r == RedirectModeTemporary
}

func (r *RedirectMode) Scan(value interface{}) error {
	if value == nil {
		*r = 0
		return nil
	}

	switch v := value.(type) {
	case int64:
		*r = RedirectMode(v)
	case int:
		*r = RedirectMode(v)
	default:
		return fmt.Errorf("failed to scan RedirectMode: %v", value)
	}

	return nil
}

func (r RedirectMode) Value() (driver.Value, error) {
	return int64(r), nil
}

// CommandStatus represents the status of a command execution
type CommandStatus string

const (
	CommandStatusPending  CommandStatus = "pending"
	CommandStatusRunning  CommandStatus = "running"
	CommandStatusFinished CommandStatus = "finished"
	CommandStatusFailed   CommandStatus = "failed"
	CommandStatusTimeout  CommandStatus = "timeout"
)

func (c CommandStatus) String() string {
	return string(c)
}

func (c CommandStatus) IsValid() bool {
	switch c {
	case CommandStatusPending, CommandStatusRunning, CommandStatusFinished, CommandStatusFailed, CommandStatusTimeout:
		return true
	}

	return false
}

func (c *CommandStatus) Scan(value interface{}) error {
	if value == nil {
		*c = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan CommandStatus: %v", value)
		}
		str = string(bytes)
	}

	*c = CommandStatus(str)

	return nil
}

func (c CommandStatus) Value() (driver.Value, error) {
	return string(c), nil
}

// QueueStatus represents the installation status of a queue worker
type QueueStatus string

const (
	QueueStatusPending      QueueStatus = "pending"
	QueueStatusInstalling   QueueStatus = "installing"
	QueueStatusActive       QueueStatus = "active"
	QueueStatusFailed       QueueStatus = "failed"
	QueueStatusUninstalling QueueStatus = "uninstalling"
)

func (q QueueStatus) String() string {
	return string(q)
}

func (q QueueStatus) IsValid() bool {
	switch q {
	case QueueStatusPending, QueueStatusInstalling, QueueStatusActive, QueueStatusFailed, QueueStatusUninstalling:
		return true
	}

	return false
}

func (q *QueueStatus) Scan(value interface{}) error {
	if value == nil {
		*q = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan QueueStatus: %v", value)
		}
		str = string(bytes)
	}

	*q = QueueStatus(str)

	return nil
}

func (q QueueStatus) Value() (driver.Value, error) {
	return string(q), nil
}

// CertificateType represents the type of SSL certificate
type CertificateType string

const (
	CertificateTypeAuto   CertificateType = "auto"
	CertificateTypeCustom CertificateType = "custom"
)

func (c CertificateType) String() string {
	return string(c)
}

func (c CertificateType) IsValid() bool {
	return c == CertificateTypeAuto || c == CertificateTypeCustom
}

func (c *CertificateType) Scan(value interface{}) error {
	if value == nil {
		*c = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan CertificateType: %v", value)
		}
		str = string(bytes)
	}

	*c = CertificateType(str)

	return nil
}

func (c CertificateType) Value() (driver.Value, error) {
	return string(c), nil
}
