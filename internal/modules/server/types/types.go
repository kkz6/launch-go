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

var allServerStatuses = []ServerStatus{
	ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
	ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
	ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed,
}

var serverStatusLabels = map[ServerStatus]string{
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

func (s ServerStatus) String() string {
	return string(s)
}

func (s ServerStatus) Label() string {
	return enumtypes.Label(s, serverStatusLabels, "Unknown")
}

func (s ServerStatus) IsValid() bool {
	return enumtypes.IsValid(s, allServerStatuses...)
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
	return allServerStatuses
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

var allServerProviders = []ServerProvider{
	ProviderDigitalOcean, ProviderHetzner, ProviderLinode,
	ProviderVultr, ProviderAWS, ProviderCustom,
}

var serverProviderLabels = map[ServerProvider]string{
	ProviderDigitalOcean: "DigitalOcean",
	ProviderHetzner:      "Hetzner Cloud",
	ProviderLinode:       "Linode",
	ProviderVultr:        "Vultr",
	ProviderAWS:          "AWS",
	ProviderCustom:       "Custom",
}

func (p ServerProvider) String() string {
	return string(p)
}

func (p ServerProvider) Label() string {
	return enumtypes.Label(p, serverProviderLabels, "Unknown")
}

func (p ServerProvider) IsValid() bool {
	return enumtypes.IsValid(p, allServerProviders...)
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
	return enumtypes.ScanStringWithDefault(p, value, ProviderCustom)
}

func (p ServerProvider) Value() (driver.Value, error) {
	return enumtypes.ValueString(p)
}

func ParseServerProvider(s string) (ServerProvider, error) {
	provider := ServerProvider(s)
	if !provider.IsValid() {
		return ProviderCustom, fmt.Errorf("invalid server provider: %s", s)
	}

	return provider, nil
}

func AllServerProviders() []ServerProvider {
	return allServerProviders
}

// =============================================================================
// ServerType
// =============================================================================

// ServerType represents the type of server
type ServerType string

const (
	ServerTypePhp          ServerType = "php"
	ServerTypeDatabase     ServerType = "database"
	ServerTypeLoadBalancer ServerType = "loadbalancer"
)

var allServerTypes = []ServerType{ServerTypePhp, ServerTypeDatabase, ServerTypeLoadBalancer}

var serverTypeLabels = map[ServerType]string{
	ServerTypePhp:          "PHP Application Server",
	ServerTypeDatabase:     "Database Server",
	ServerTypeLoadBalancer: "Load Balancer",
}

func (t ServerType) String() string {
	return string(t)
}

func (t ServerType) Label() string {
	return enumtypes.Label(t, serverTypeLabels, "Unknown")
}

func (t ServerType) IsValid() bool {
	return enumtypes.IsValid(t, allServerTypes...)
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
	case ServerTypeLoadBalancer:
		return []ServerFeature{
			ServerFeatureLoadBalancing,
			ServerFeatureSSLCertificates,
			ServerFeatureServices,
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
	case ServerTypeDatabase, ServerTypeLoadBalancer:
		return ProcessManagerNone
	}

	return ProcessManagerNone
}

func (t *ServerType) Scan(value interface{}) error {
	return enumtypes.ScanStringWithDefault(t, value, ServerTypePhp)
}

func (t ServerType) Value() (driver.Value, error) {
	return enumtypes.ValueString(t)
}

func ParseServerType(s string) (ServerType, error) {
	t := ServerType(s)
	if !t.IsValid() {
		return ServerTypePhp, fmt.Errorf("invalid server type: %s", s)
	}

	return t, nil
}

func AllServerTypes() []ServerType {
	return allServerTypes
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
	ServerFeatureLoadBalancing      ServerFeature = "load_balancing"
)

var allServerFeatures = []ServerFeature{
	ServerFeatureSites, ServerFeatureSSLCertificates, ServerFeaturePhpManagement,
	ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
	ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
	ServerFeatureServices, ServerFeatureLoadBalancing,
}

var serverFeatureLabels = map[ServerFeature]string{
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
	ServerFeatureLoadBalancing:      "Load Balancing",
}

func (f ServerFeature) String() string {
	return string(f)
}

func (f ServerFeature) Label() string {
	return enumtypes.Label(f, serverFeatureLabels, "Unknown")
}

func (f ServerFeature) IsValid() bool {
	return enumtypes.IsValid(f, allServerFeatures...)
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
		ServerFeatureLoadBalancing:      "upstreams",
	}
	if key, ok := keys[f]; ok {
		return key
	}

	return ""
}

func AllServerFeatures() []ServerFeature {
	return allServerFeatures
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

var allServiceTypes = []ServiceType{
	ServiceTypePhp, ServiceTypeMySQL, ServiceTypePostgreSQL,
	ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
	ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent,
}

var serviceTypeLabels = map[ServiceType]string{
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

func (s ServiceType) String() string {
	return string(s)
}

func (s ServiceType) Label() string {
	return enumtypes.Label(s, serviceTypeLabels, "Unknown")
}

func (s ServiceType) IsValid() bool {
	return enumtypes.IsValid(s, allServiceTypes...)
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
	return enumtypes.ScanString(s, value)
}

func (s ServiceType) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

func ParseServiceType(str string) (ServiceType, error) {
	s := ServiceType(str)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid service type: %s", str)
	}

	return s, nil
}

func AllServiceTypes() []ServiceType {
	return allServiceTypes
}

// =============================================================================
// ServiceStatus
// =============================================================================

// ServiceStatus represents the status of a service
type ServiceStatus string

const (
	ServiceStatusPending      ServiceStatus = "pending"
	ServiceStatusInstalling   ServiceStatus = "installing"
	ServiceStatusUninstalling ServiceStatus = "uninstalling"
	ServiceStatusFailed       ServiceStatus = "failed"
	ServiceStatusInstalled    ServiceStatus = "installed"
	ServiceStatusStopped      ServiceStatus = "stopped"
	ServiceStatusRunning      ServiceStatus = "running"
)

var allServiceStatuses = []ServiceStatus{
	ServiceStatusPending, ServiceStatusInstalling, ServiceStatusUninstalling,
	ServiceStatusFailed, ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning,
}

var serviceStatusLabels = map[ServiceStatus]string{
	ServiceStatusPending:      "Pending",
	ServiceStatusInstalling:   "Installing",
	ServiceStatusUninstalling: "Uninstalling",
	ServiceStatusFailed:       "Failed",
	ServiceStatusInstalled:    "Installed",
	ServiceStatusStopped:      "Stopped",
	ServiceStatusRunning:      "Running",
}

func (s ServiceStatus) String() string {
	return string(s)
}

func (s ServiceStatus) Label() string {
	return enumtypes.Label(s, serviceStatusLabels, "Unknown")
}

func (s ServiceStatus) IsValid() bool {
	return enumtypes.IsValid(s, allServiceStatuses...)
}

func (s ServiceStatus) IsActive() bool {
	return s == ServiceStatusRunning || s == ServiceStatusInstalled
}

func (s *ServiceStatus) Scan(value interface{}) error {
	return enumtypes.ScanStringWithDefault(s, value, ServiceStatusPending)
}

func (s ServiceStatus) Value() (driver.Value, error) {
	return enumtypes.ValueString(s)
}

func ParseServiceStatus(str string) (ServiceStatus, error) {
	s := ServiceStatus(str)
	if !s.IsValid() {
		return ServiceStatusPending, fmt.Errorf("invalid service status: %s", str)
	}

	return s, nil
}

func AllServiceStatuses() []ServiceStatus {
	return allServiceStatuses
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

var allServiceOptions = []ServiceOption{
	ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
	ServiceOptionRemove, ServiceOptionStatus,
}

var serviceOptionLabels = map[ServiceOption]string{
	ServiceOptionStart:   "Start",
	ServiceOptionStop:    "Stop",
	ServiceOptionRestart: "Restart",
	ServiceOptionRemove:  "Remove",
	ServiceOptionStatus:  "Status",
}

func (o ServiceOption) String() string {
	return string(o)
}

func (o ServiceOption) Label() string {
	return enumtypes.Label(o, serviceOptionLabels, "Unknown")
}

func (o ServiceOption) IsValid() bool {
	return enumtypes.IsValid(o, allServiceOptions...)
}

func ParseServiceOption(s string) (ServiceOption, error) {
	option := ServiceOption(s)
	if !option.IsValid() {
		return "", fmt.Errorf("invalid service option: %s", s)
	}

	return option, nil
}

func AllServiceOptions() []ServiceOption {
	return allServiceOptions
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

var allProcessManagers = []ProcessManager{ProcessManagerSupervisor, ProcessManagerNone}

var processManagerLabels = map[ProcessManager]string{
	ProcessManagerSupervisor: "Supervisor",
	ProcessManagerNone:       "None",
}

func (p ProcessManager) String() string {
	return string(p)
}

func (p ProcessManager) Label() string {
	return enumtypes.Label(p, processManagerLabels, "Unknown")
}

func (p ProcessManager) IsValid() bool {
	return enumtypes.IsValid(p, allProcessManagers...)
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
	return allProcessManagers
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

var allOperatingSystems = []OperatingSystem{OSUbuntu20, OSUbuntu22, OSUbuntu24}

var operatingSystemLabels = map[OperatingSystem]string{
	OSUbuntu20: "Ubuntu 20.04",
	OSUbuntu22: "Ubuntu 22.04",
	OSUbuntu24: "Ubuntu 24.04",
}

func (o OperatingSystem) String() string {
	return string(o)
}

func (o OperatingSystem) Label() string {
	return enumtypes.Label(o, operatingSystemLabels, "Unknown")
}

func (o OperatingSystem) IsValid() bool {
	return enumtypes.IsValid(o, allOperatingSystems...)
}

func (o *OperatingSystem) Scan(value interface{}) error {
	return enumtypes.ScanStringWithDefault(o, value, OSUbuntu24)
}

func (o OperatingSystem) Value() (driver.Value, error) {
	return enumtypes.ValueString(o)
}

func ParseOperatingSystem(s string) (OperatingSystem, error) {
	os := OperatingSystem(s)
	if !os.IsValid() {
		return OSUbuntu24, fmt.Errorf("invalid operating system: %s", s)
	}

	return os, nil
}

func AllOperatingSystems() []OperatingSystem {
	return allOperatingSystems
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

var allRuleActions = []RuleAction{RuleActionAllow, RuleActionDeny, RuleActionReject}

var ruleActionLabels = map[RuleAction]string{
	RuleActionAllow:  "Allow",
	RuleActionDeny:   "Deny",
	RuleActionReject: "Reject",
}

func (r RuleAction) String() string {
	return string(r)
}

func (r RuleAction) Label() string {
	return enumtypes.Label(r, ruleActionLabels, "Unknown")
}

func (r RuleAction) IsValid() bool {
	return enumtypes.IsValid(r, allRuleActions...)
}

func (r *RuleAction) Scan(value interface{}) error {
	return enumtypes.ScanStringWithDefault(r, value, RuleActionAllow)
}

func (r RuleAction) Value() (driver.Value, error) {
	return enumtypes.ValueString(r)
}

func ParseRuleAction(s string) (RuleAction, error) {
	action := RuleAction(s)
	if !action.IsValid() {
		return RuleActionAllow, fmt.Errorf("invalid rule action: %s", s)
	}

	return action, nil
}

func AllRuleActions() []RuleAction {
	return allRuleActions
}

// =============================================================================
// TaskStatus
// =============================================================================

// TaskStatus represents the status of a task execution
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusFinished TaskStatus = "finished"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusTimeout  TaskStatus = "timeout"
)

func (s TaskStatus) String() string {
	return string(s)
}

func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusFinished || s == TaskStatusFailed || s == TaskStatusTimeout
}
