// Package types contains all type definitions for the server module
package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// ServerStatus
// =============================================================================

// ServerStatus represents the current state of a server
type ServerStatus string

const (
	ServerStatusNew          ServerStatus = "new"
	ServerStatusStarting     ServerStatus = "starting"
	ServerStatusProvisioning ServerStatus = "provisioning"
	ServerStatusRunning      ServerStatus = "running"
	ServerStatusPaused       ServerStatus = "paused"
	ServerStatusStopped      ServerStatus = "stopped"
	ServerStatusDeleting     ServerStatus = "deleting"
	ServerStatusArchived     ServerStatus = "archived"
	ServerStatusUnknown      ServerStatus = "unknown"
	ServerStatusFailed       ServerStatus = "failed"
)

func (s ServerStatus) String() string {
	return string(s)
}

func (s ServerStatus) Label() string {
	labels := map[ServerStatus]string{
		ServerStatusNew:          "Connecting",
		ServerStatusStarting:     "Starting",
		ServerStatusProvisioning: "Provisioning",
		ServerStatusRunning:      "Running",
		ServerStatusPaused:       "Paused",
		ServerStatusStopped:      "Stopped",
		ServerStatusDeleting:     "Deleting",
		ServerStatusArchived:     "Archived",
		ServerStatusUnknown:      "Unknown",
		ServerStatusFailed:       "Failed",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s ServerStatus) IsValid() bool {
	switch s {
	case ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
		ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
		ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed:
		return true
	}

	return false
}

func (s ServerStatus) IsActive() bool {
	return s == ServerStatusRunning || s == ServerStatusProvisioning || s == ServerStatusStarting
}

func (s ServerStatus) IsTerminal() bool {
	return s == ServerStatusFailed || s == ServerStatusArchived || s == ServerStatusDeleting
}

func (s *ServerStatus) Scan(value interface{}) error {
	return enumtypes.ScanString(s, value)
}

func (s ServerStatus) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

func ParseServerStatus(s string) (ServerStatus, error) {
	status := ServerStatus(s)
	if !status.IsValid() {
		return ServerStatusUnknown, fmt.Errorf("invalid server status: %s", s)
	}

	return status, nil
}

func AllServerStatuses() []ServerStatus {
	return []ServerStatus{
		ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
		ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
		ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed,
	}
}

// =============================================================================
// ServerProvider
// =============================================================================

// ServerProvider represents cloud providers
type ServerProvider string

const (
	ProviderDigitalOcean ServerProvider = "digitalocean"
	ProviderHetzner      ServerProvider = "hetzner"
	ProviderLinode       ServerProvider = "linode"
	ProviderVultr        ServerProvider = "vultr"
	ProviderAWS          ServerProvider = "aws"
	ProviderCustom       ServerProvider = "custom_server"
)

func (p ServerProvider) String() string {
	return string(p)
}

func (p ServerProvider) Label() string {
	labels := map[ServerProvider]string{
		ProviderDigitalOcean: "DigitalOcean",
		ProviderHetzner:      "Hetzner Cloud",
		ProviderLinode:       "Linode",
		ProviderVultr:        "Vultr",
		ProviderAWS:          "AWS",
		ProviderCustom:       "Custom",
	}
	if label, ok := labels[p]; ok {
		return label
	}

	return "Unknown"
}

func (p ServerProvider) IsValid() bool {
	switch p {
	case ProviderDigitalOcean, ProviderHetzner, ProviderLinode,
		ProviderVultr, ProviderAWS, ProviderCustom:
		return true
	}

	return false
}

func (p ServerProvider) IsCloud() bool {
	return p != ProviderCustom
}

func (p ServerProvider) GetDefaultUsername(os OperatingSystem) string {
	switch p {
	case ProviderAWS:
		return "ubuntu"
	default:
		return "root"
	}
}

func (p *ServerProvider) Scan(value interface{}) error {
	if value == nil {
		*p = ProviderCustom
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*p = ServerProvider(v)
	case string:
		*p = ServerProvider(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServerProvider", value)
	}

	return nil
}

func (p ServerProvider) Value() (driver.Value, error) {
	return string(p), nil
}

func ParseServerProvider(s string) (ServerProvider, error) {
	provider := ServerProvider(s)
	if !provider.IsValid() {
		return ProviderCustom, fmt.Errorf("invalid server provider: %s", s)
	}

	return provider, nil
}

func AllServerProviders() []ServerProvider {
	return []ServerProvider{
		ProviderDigitalOcean, ProviderHetzner, ProviderLinode,
		ProviderVultr, ProviderAWS, ProviderCustom,
	}
}

// =============================================================================
// ServerType
// =============================================================================

// ServerType represents the type of server
type ServerType string

const (
	ServerTypePhp      ServerType = "php"
	ServerTypeDatabase ServerType = "database"
)

func (t ServerType) String() string {
	return string(t)
}

func (t ServerType) Label() string {
	labels := map[ServerType]string{
		ServerTypePhp:      "PHP Application Server",
		ServerTypeDatabase: "Database Server",
	}
	if label, ok := labels[t]; ok {
		return label
	}

	return "Unknown"
}

func (t ServerType) IsValid() bool {
	switch t {
	case ServerTypePhp, ServerTypeDatabase:
		return true
	}

	return false
}

func (t ServerType) GetFeatures() []ServerFeature {
	switch t {
	case ServerTypePhp:
		return []ServerFeature{
			ServerFeatureSites,
			ServerFeaturePhpManagement,
			ServerFeatureComposer,
			ServerFeatureDatabaseManagement,
			ServerFeatureQueueWorkers,
			ServerFeatureDaemons,
			ServerFeatureScheduler,
			ServerFeatureSSLCertificates,
			ServerFeatureRedis,
			ServerFeatureBackups,
			ServerFeatureServices,
		}
	case ServerTypeDatabase:
		return []ServerFeature{
			ServerFeatureDatabaseManagement,
			ServerFeatureBackups,
		}
	}

	return nil
}

func (t ServerType) HasFeature(feature ServerFeature) bool {
	features := t.GetFeatures()
	for _, f := range features {
		if f == feature {
			return true
		}
	}

	return false
}

func (t ServerType) GetProcessManager() ProcessManager {
	switch t {
	case ServerTypePhp:
		return ProcessManagerSupervisor
	case ServerTypeDatabase:
		return ProcessManagerNone
	}

	return ProcessManagerNone
}

func (t *ServerType) Scan(value interface{}) error {
	if value == nil {
		*t = ServerTypePhp
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*t = ServerType(v)
	case string:
		*t = ServerType(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServerType", value)
	}

	return nil
}

func (t ServerType) Value() (driver.Value, error) {
	return string(t), nil
}

func ParseServerType(s string) (ServerType, error) {
	t := ServerType(s)
	if !t.IsValid() {
		return ServerTypePhp, fmt.Errorf("invalid server type: %s", s)
	}

	return t, nil
}

func AllServerTypes() []ServerType {
	return []ServerType{ServerTypePhp, ServerTypeDatabase}
}

// =============================================================================
// ServerFeature
// =============================================================================

// ServerFeature represents available server features
type ServerFeature string

const (
	ServerFeatureSites              ServerFeature = "sites"
	ServerFeatureSSLCertificates    ServerFeature = "ssl_certificates"
	ServerFeaturePhpManagement      ServerFeature = "php_management"
	ServerFeatureComposer           ServerFeature = "composer"
	ServerFeatureDatabaseManagement ServerFeature = "database_management"
	ServerFeatureQueueWorkers       ServerFeature = "queue_workers"
	ServerFeatureDaemons            ServerFeature = "daemons"
	ServerFeatureScheduler          ServerFeature = "scheduler"
	ServerFeatureRedis              ServerFeature = "redis"
	ServerFeatureBackups            ServerFeature = "backups"
	ServerFeatureServices           ServerFeature = "services"
)

func (f ServerFeature) String() string {
	return string(f)
}

func (f ServerFeature) Label() string {
	labels := map[ServerFeature]string{
		ServerFeatureSites:              "Sites",
		ServerFeatureSSLCertificates:    "SSL Certificates",
		ServerFeaturePhpManagement:      "PHP Management",
		ServerFeatureComposer:           "Composer",
		ServerFeatureDatabaseManagement: "Database Management",
		ServerFeatureQueueWorkers:       "Queue Workers",
		ServerFeatureDaemons:            "Daemons",
		ServerFeatureScheduler:          "Scheduler",
		ServerFeatureRedis:              "Redis",
		ServerFeatureBackups:            "Backups",
		ServerFeatureServices:           "Services",
	}
	if label, ok := labels[f]; ok {
		return label
	}

	return "Unknown"
}

func (f ServerFeature) IsValid() bool {
	switch f {
	case ServerFeatureSites, ServerFeatureSSLCertificates, ServerFeaturePhpManagement,
		ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
		ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
		ServerFeatureServices:
		return true
	}

	return false
}

func (f ServerFeature) NavigationKey() string {
	keys := map[ServerFeature]string{
		ServerFeatureSites:              "sites",
		ServerFeatureDatabaseManagement: "databases",
		ServerFeaturePhpManagement:      "php",
		ServerFeatureQueueWorkers:       "queues",
		ServerFeatureDaemons:            "daemons",
		ServerFeatureScheduler:          "scheduler",
		ServerFeatureSSLCertificates:    "ssl",
		ServerFeatureBackups:            "backups",
		ServerFeatureServices:           "advanced",
	}
	if key, ok := keys[f]; ok {
		return key
	}

	return ""
}

func AllServerFeatures() []ServerFeature {
	return []ServerFeature{
		ServerFeatureSites, ServerFeatureSSLCertificates, ServerFeaturePhpManagement,
		ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
		ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
		ServerFeatureServices,
	}
}

// =============================================================================
// ServiceType
// =============================================================================

// ServiceType represents types of services
type ServiceType string

const (
	ServiceTypePhp         ServiceType = "php"
	ServiceTypeMySQL       ServiceType = "mysql"
	ServiceTypePostgreSQL  ServiceType = "postgresql"
	ServiceTypeSupervisor  ServiceType = "process_manager"
	ServiceTypeRedis       ServiceType = "memory_database"
	ServiceTypeCaddy       ServiceType = "webserver"
	ServiceTypeComposer    ServiceType = "package_manager"
	ServiceTypeNode        ServiceType = "node"
	ServiceTypeBun         ServiceType = "bun"
	ServiceTypeLaunchAgent ServiceType = "launch_agent"
)

func (s ServiceType) String() string {
	return string(s)
}

func (s ServiceType) Label() string {
	labels := map[ServiceType]string{
		ServiceTypePhp:         "PHP",
		ServiceTypeMySQL:       "MySQL",
		ServiceTypePostgreSQL:  "PostgreSQL",
		ServiceTypeSupervisor:  "Supervisor",
		ServiceTypeRedis:       "Redis",
		ServiceTypeCaddy:       "Caddy",
		ServiceTypeComposer:    "Composer",
		ServiceTypeNode:        "Node.js",
		ServiceTypeBun:         "Bun",
		ServiceTypeLaunchAgent: "Launch Agent",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s ServiceType) IsValid() bool {
	switch s {
	case ServiceTypePhp, ServiceTypeMySQL, ServiceTypePostgreSQL,
		ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
		ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent:
		return true
	}

	return false
}

func (s ServiceType) IsDatabase() bool {
	return s == ServiceTypeMySQL || s == ServiceTypePostgreSQL
}

func (s ServiceType) GetDatabasePort() int {
	switch s {
	case ServiceTypeMySQL:
		return 3306
	case ServiceTypePostgreSQL:
		return 5432
	}

	return 0
}

func (s ServiceType) GetDatabaseConnection() string {
	switch s {
	case ServiceTypeMySQL:
		return "mysql"
	case ServiceTypePostgreSQL:
		return "pgsql"
	}

	return ""
}

func (s *ServiceType) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*s = ServiceType(v)
	case string:
		*s = ServiceType(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServiceType", value)
	}

	return nil
}

func (s ServiceType) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseServiceType(str string) (ServiceType, error) {
	s := ServiceType(str)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid service type: %s", str)
	}

	return s, nil
}

func AllServiceTypes() []ServiceType {
	return []ServiceType{
		ServiceTypePhp, ServiceTypeMySQL, ServiceTypePostgreSQL,
		ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
		ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent,
	}
}

// =============================================================================
// ServiceStatus
// =============================================================================

// ServiceStatus represents the status of a service
type ServiceStatus string

const (
	ServiceStatusPending    ServiceStatus = "pending"
	ServiceStatusInstalling ServiceStatus = "installing"
	ServiceStatusFailed     ServiceStatus = "failed"
	ServiceStatusInstalled  ServiceStatus = "installed"
	ServiceStatusStopped    ServiceStatus = "stopped"
	ServiceStatusRunning    ServiceStatus = "running"
)

func (s ServiceStatus) String() string {
	return string(s)
}

func (s ServiceStatus) Label() string {
	labels := map[ServiceStatus]string{
		ServiceStatusPending:    "Pending",
		ServiceStatusInstalling: "Installing",
		ServiceStatusFailed:     "Failed",
		ServiceStatusInstalled:  "Installed",
		ServiceStatusStopped:    "Stopped",
		ServiceStatusRunning:    "Running",
	}
	if label, ok := labels[s]; ok {
		return label
	}

	return "Unknown"
}

func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusPending, ServiceStatusInstalling, ServiceStatusFailed,
		ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning:
		return true
	}

	return false
}

func (s ServiceStatus) IsActive() bool {
	return s == ServiceStatusRunning || s == ServiceStatusInstalled
}

func (s *ServiceStatus) Scan(value interface{}) error {
	if value == nil {
		*s = ServiceStatusPending
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*s = ServiceStatus(v)
	case string:
		*s = ServiceStatus(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServiceStatus", value)
	}

	return nil
}

func (s ServiceStatus) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseServiceStatus(str string) (ServiceStatus, error) {
	s := ServiceStatus(str)
	if !s.IsValid() {
		return ServiceStatusPending, fmt.Errorf("invalid service status: %s", str)
	}

	return s, nil
}

func AllServiceStatuses() []ServiceStatus {
	return []ServiceStatus{
		ServiceStatusPending, ServiceStatusInstalling, ServiceStatusFailed,
		ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning,
	}
}

// =============================================================================
// ServiceOption
// =============================================================================

// ServiceOption represents actions that can be performed on a service
type ServiceOption string

const (
	ServiceOptionStart   ServiceOption = "start"
	ServiceOptionStop    ServiceOption = "stop"
	ServiceOptionRestart ServiceOption = "restart"
	ServiceOptionRemove  ServiceOption = "remove"
	ServiceOptionStatus  ServiceOption = "status"
)

func (o ServiceOption) String() string {
	return string(o)
}

func (o ServiceOption) Label() string {
	labels := map[ServiceOption]string{
		ServiceOptionStart:   "Start",
		ServiceOptionStop:    "Stop",
		ServiceOptionRestart: "Restart",
		ServiceOptionRemove:  "Remove",
		ServiceOptionStatus:  "Status",
	}
	if label, ok := labels[o]; ok {
		return label
	}

	return "Unknown"
}

func (o ServiceOption) IsValid() bool {
	switch o {
	case ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
		ServiceOptionRemove, ServiceOptionStatus:
		return true
	}

	return false
}

func ParseServiceOption(s string) (ServiceOption, error) {
	option := ServiceOption(s)
	if !option.IsValid() {
		return "", fmt.Errorf("invalid service option: %s", s)
	}

	return option, nil
}

func AllServiceOptions() []ServiceOption {
	return []ServiceOption{
		ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
		ServiceOptionRemove, ServiceOptionStatus,
	}
}

// =============================================================================
// ProcessManager
// =============================================================================

// ProcessManager represents available process managers
type ProcessManager string

const (
	ProcessManagerSupervisor ProcessManager = "supervisor"
	ProcessManagerNone       ProcessManager = "none"
)

func (p ProcessManager) String() string {
	return string(p)
}

func (p ProcessManager) Label() string {
	labels := map[ProcessManager]string{
		ProcessManagerSupervisor: "Supervisor",
		ProcessManagerNone:       "None",
	}
	if label, ok := labels[p]; ok {
		return label
	}

	return "Unknown"
}

func (p ProcessManager) IsValid() bool {
	switch p {
	case ProcessManagerSupervisor, ProcessManagerNone:
		return true
	}

	return false
}

func (p ProcessManager) ServiceName() string {
	switch p {
	case ProcessManagerSupervisor:
		return "supervisor"
	case ProcessManagerNone:
		return ""
	}

	return ""
}

func AllProcessManagers() []ProcessManager {
	return []ProcessManager{ProcessManagerSupervisor, ProcessManagerNone}
}

// =============================================================================
// OperatingSystem
// =============================================================================

// OperatingSystem represents supported operating systems
type OperatingSystem string

const (
	OSUbuntu20 OperatingSystem = "ubuntu_20"
	OSUbuntu22 OperatingSystem = "ubuntu_22"
	OSUbuntu24 OperatingSystem = "ubuntu_24"
)

func (o OperatingSystem) String() string {
	return string(o)
}

func (o OperatingSystem) Label() string {
	labels := map[OperatingSystem]string{
		OSUbuntu20: "Ubuntu 20.04",
		OSUbuntu22: "Ubuntu 22.04",
		OSUbuntu24: "Ubuntu 24.04",
	}
	if label, ok := labels[o]; ok {
		return label
	}

	return "Unknown"
}

func (o OperatingSystem) IsValid() bool {
	switch o {
	case OSUbuntu20, OSUbuntu22, OSUbuntu24:
		return true
	}

	return false
}

func (o *OperatingSystem) Scan(value interface{}) error {
	if value == nil {
		*o = OSUbuntu24
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*o = OperatingSystem(v)
	case string:
		*o = OperatingSystem(v)
	default:
		return fmt.Errorf("cannot scan type %T into OperatingSystem", value)
	}

	return nil
}

func (o OperatingSystem) Value() (driver.Value, error) {
	return string(o), nil
}

func ParseOperatingSystem(s string) (OperatingSystem, error) {
	os := OperatingSystem(s)
	if !os.IsValid() {
		return OSUbuntu24, fmt.Errorf("invalid operating system: %s", s)
	}

	return os, nil
}

func AllOperatingSystems() []OperatingSystem {
	return []OperatingSystem{OSUbuntu20, OSUbuntu22, OSUbuntu24}
}

// =============================================================================
// RuleAction
// =============================================================================

// RuleAction represents firewall rule actions
type RuleAction string

const (
	RuleActionAllow  RuleAction = "allow"
	RuleActionDeny   RuleAction = "deny"
	RuleActionReject RuleAction = "reject"
)

func (r RuleAction) String() string {
	return string(r)
}

func (r RuleAction) Label() string {
	labels := map[RuleAction]string{
		RuleActionAllow:  "Allow",
		RuleActionDeny:   "Deny",
		RuleActionReject: "Reject",
	}
	if label, ok := labels[r]; ok {
		return label
	}

	return "Unknown"
}

func (r RuleAction) IsValid() bool {
	switch r {
	case RuleActionAllow, RuleActionDeny, RuleActionReject:
		return true
	}

	return false
}

func (r *RuleAction) Scan(value interface{}) error {
	if value == nil {
		*r = RuleActionAllow
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*r = RuleAction(v)
	case string:
		*r = RuleAction(v)
	default:
		return fmt.Errorf("cannot scan type %T into RuleAction", value)
	}

	return nil
}

func (r RuleAction) Value() (driver.Value, error) {
	return string(r), nil
}

func ParseRuleAction(s string) (RuleAction, error) {
	action := RuleAction(s)
	if !action.IsValid() {
		return RuleActionAllow, fmt.Errorf("invalid rule action: %s", s)
	}

	return action, nil
}

func AllRuleActions() []RuleAction {
	return []RuleAction{RuleActionAllow, RuleActionDeny, RuleActionReject}
}
