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
		"tls_setting":                     TLSSettingAuto,
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

// IsLaravel returns true if this is a Laravel site type
func (s SiteType) IsLaravel() bool {
	return s == SiteTypeLaravel
}

// IsPHP returns true if this is a PHP-based site type
func (s SiteType) IsPHP() bool {
	return s.IsLaravel() || s == SiteTypeWordpress
}

// IsStatic returns true if this is a static site type
func (s SiteType) IsStatic() bool {
	return s == SiteTypeStatic
}

// SupportsZeroDowntime returns true if this site type supports zero-downtime deployments
func (s SiteType) SupportsZeroDowntime() bool {
	return s.IsLaravel() || s == SiteTypeStatic
}

// SupportsQueue returns true if this site type supports queue workers
func (s SiteType) SupportsQueue() bool {
	return s.IsLaravel()
}

// SupportsScheduler returns true if this site type supports the task scheduler
func (s SiteType) SupportsScheduler() bool {
	return s.IsLaravel()
}

// SupportsHorizon returns true if this site type supports Laravel Horizon
func (s SiteType) SupportsHorizon() bool {
	return s.IsLaravel()
}

// SupportsMigrations returns true if this site type supports database migrations
func (s SiteType) SupportsMigrations() bool {
	return s.IsLaravel()
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

// TLSSetting represents the TLS/SSL configuration for a site
type TLSSetting string

const (
	TLSSettingAuto     TLSSetting = "auto"
	TLSSettingCustom   TLSSetting = "custom"
	TLSSettingInternal TLSSetting = "internal"
	TLSSettingOff      TLSSetting = "off"
)

func (t TLSSetting) String() string {
	return string(t)
}

func (t TLSSetting) Label() string {
	labels := map[TLSSetting]string{
		TLSSettingAuto:     "Auto",
		TLSSettingCustom:   "Custom",
		TLSSettingInternal: "Internal",
		TLSSettingOff:      "Off",
	}

	if label, ok := labels[t]; ok {
		return label
	}

	return string(t)
}

func (t TLSSetting) IsValid() bool {
	switch t {
	case TLSSettingAuto, TLSSettingCustom, TLSSettingInternal, TLSSettingOff:
		return true
	}

	return false
}

func (t TLSSetting) IsEnabled() bool {
	return t != TLSSettingOff
}

func (t TLSSetting) GetPort() int {
	if t == TLSSettingOff {
		return 80
	}

	return 443
}

func (t TLSSetting) GetProtocol() string {
	if t == TLSSettingOff {
		return "http"
	}

	return "https"
}

func (t *TLSSetting) Scan(value interface{}) error {
	if value == nil {
		*t = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan TLSSetting: %v", value)
		}
		str = string(bytes)
	}

	*t = TLSSetting(str)

	return nil
}

func (t TLSSetting) Value() (driver.Value, error) {
	return string(t), nil
}

// AllTLSSettings returns all valid TLS settings
func AllTLSSettings() []TLSSetting {
	return []TLSSetting{
		TLSSettingAuto,
		TLSSettingCustom,
		TLSSettingInternal,
		TLSSettingOff,
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

// LaravelFeature represents a Laravel feature that can be enabled/disabled
type LaravelFeature string

const (
	LaravelFeatureScheduler LaravelFeature = "scheduler"
	LaravelFeatureQueue     LaravelFeature = "queue"
	LaravelFeatureHorizon   LaravelFeature = "horizon"
	LaravelFeatureInertia   LaravelFeature = "inertia"
	LaravelFeatureOctane    LaravelFeature = "octane"
	LaravelFeatureReverb    LaravelFeature = "reverb"
)

func (f LaravelFeature) String() string {
	return string(f)
}

func (f LaravelFeature) Label() string {
	labels := map[LaravelFeature]string{
		LaravelFeatureScheduler: "Task Scheduler",
		LaravelFeatureQueue:     "Queue Workers",
		LaravelFeatureHorizon:   "Horizon",
		LaravelFeatureInertia:   "Inertia SSR",
		LaravelFeatureOctane:    "Octane",
		LaravelFeatureReverb:    "Reverb",
	}
	if label, ok := labels[f]; ok {
		return label
	}
	return string(f)
}

func (f LaravelFeature) Description() string {
	descriptions := map[LaravelFeature]string{
		LaravelFeatureScheduler: "Run scheduled tasks using Laravel's task scheduler",
		LaravelFeatureQueue:     "Process queued jobs in the background",
		LaravelFeatureHorizon:   "Monitor and manage Laravel queues with Horizon",
		LaravelFeatureInertia:   "Enable server-side rendering for Inertia.js",
		LaravelFeatureOctane:    "Supercharge your application with Octane",
		LaravelFeatureReverb:    "Real-time WebSocket broadcasting with Reverb",
	}
	if desc, ok := descriptions[f]; ok {
		return desc
	}
	return ""
}

func (f LaravelFeature) IsValid() bool {
	switch f {
	case LaravelFeatureScheduler, LaravelFeatureQueue, LaravelFeatureHorizon,
		LaravelFeatureInertia, LaravelFeatureOctane, LaravelFeatureReverb:
		return true
	}
	return false
}

// ConflictsWith returns features that conflict with this feature
func (f LaravelFeature) ConflictsWith() []LaravelFeature {
	switch f {
	case LaravelFeatureQueue:
		return []LaravelFeature{LaravelFeatureHorizon}
	case LaravelFeatureHorizon:
		return []LaravelFeature{LaravelFeatureQueue}
	default:
		return nil
	}
}

// AllLaravelFeatures returns all available Laravel features
func AllLaravelFeatures() []LaravelFeature {
	return []LaravelFeature{
		LaravelFeatureScheduler,
		LaravelFeatureQueue,
		LaravelFeatureHorizon,
		LaravelFeatureInertia,
		LaravelFeatureOctane,
		LaravelFeatureReverb,
	}
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

// PhpVersion represents a PHP version for a site
type PhpVersion string

const (
	PhpVersion56 PhpVersion = "php56"
	PhpVersion70 PhpVersion = "php70"
	PhpVersion71 PhpVersion = "php71"
	PhpVersion72 PhpVersion = "php72"
	PhpVersion73 PhpVersion = "php73"
	PhpVersion74 PhpVersion = "php74"
	PhpVersion80 PhpVersion = "php80"
	PhpVersion81 PhpVersion = "php81"
	PhpVersion82 PhpVersion = "php82"
	PhpVersion83 PhpVersion = "php83"
	PhpVersion84 PhpVersion = "php84"
)

func (p PhpVersion) String() string {
	return string(p)
}

func (p PhpVersion) Label() string {
	labels := map[PhpVersion]string{
		PhpVersion56: "PHP 5.6",
		PhpVersion70: "PHP 7.0",
		PhpVersion71: "PHP 7.1",
		PhpVersion72: "PHP 7.2",
		PhpVersion73: "PHP 7.3",
		PhpVersion74: "PHP 7.4",
		PhpVersion80: "PHP 8.0",
		PhpVersion81: "PHP 8.1",
		PhpVersion82: "PHP 8.2",
		PhpVersion83: "PHP 8.3",
		PhpVersion84: "PHP 8.4",
	}
	if label, ok := labels[p]; ok {
		return label
	}
	return string(p)
}

func (p PhpVersion) IsValid() bool {
	switch p {
	case PhpVersion56, PhpVersion70, PhpVersion71, PhpVersion72, PhpVersion73,
		PhpVersion74, PhpVersion80, PhpVersion81, PhpVersion82, PhpVersion83, PhpVersion84:
		return true
	}
	return false
}

// GetVersion returns the version string (e.g., "8.3")
func (p PhpVersion) GetVersion() string {
	versions := map[PhpVersion]string{
		PhpVersion56: "5.6",
		PhpVersion70: "7.0",
		PhpVersion71: "7.1",
		PhpVersion72: "7.2",
		PhpVersion73: "7.3",
		PhpVersion74: "7.4",
		PhpVersion80: "8.0",
		PhpVersion81: "8.1",
		PhpVersion82: "8.2",
		PhpVersion83: "8.3",
		PhpVersion84: "8.4",
	}
	if v, ok := versions[p]; ok {
		return v
	}
	return ""
}

// BinaryPath returns the PHP binary path (e.g., "php8.3")
func (p PhpVersion) BinaryPath() string {
	if !p.IsValid() {
		return "php"
	}
	return "php" + p.GetVersion()
}

// FpmServiceName returns the PHP-FPM service name (e.g., "php8.3-fpm")
func (p PhpVersion) FpmServiceName() string {
	if !p.IsValid() {
		return ""
	}
	return "php" + p.GetVersion() + "-fpm"
}

// SocketPath returns the PHP-FPM socket path (e.g., "/run/php/php8.3-fpm.sock")
func (p PhpVersion) SocketPath() string {
	if !p.IsValid() {
		return ""
	}
	return "/run/php/php" + p.GetVersion() + "-fpm.sock"
}

func (p *PhpVersion) Scan(value interface{}) error {
	if value == nil {
		*p = ""
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan PhpVersion: %v", value)
		}
		str = string(bytes)
	}

	*p = PhpVersion(str)
	return nil
}

func (p PhpVersion) Value() (driver.Value, error) {
	return string(p), nil
}

// AllPhpVersions returns all supported PHP versions (newest first)
func AllPhpVersions() []PhpVersion {
	return []PhpVersion{
		PhpVersion84, PhpVersion83, PhpVersion82, PhpVersion81, PhpVersion80,
		PhpVersion74, PhpVersion73, PhpVersion72, PhpVersion71, PhpVersion70, PhpVersion56,
	}
}

// ParsePhpVersion parses a string into a PhpVersion
func ParsePhpVersion(s string) (PhpVersion, error) {
	p := PhpVersion(s)
	if !p.IsValid() {
		return "", fmt.Errorf("invalid PHP version: %s", s)
	}
	return p, nil
}
