package server

import (
	"database/sql/driver"
	"fmt"
)

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
	if value == nil {
		*s = ServerStatusUnknown
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*s = ServerStatus(v)
	case string:
		*s = ServerStatus(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServerStatus", value)
	}
	return nil
}

func (s ServerStatus) Value() (driver.Value, error) {
	return string(s), nil
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
			ServerFeatureSslCertificates,
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

// ServiceType represents types of services
type ServiceType string

const (
	ServiceTypePhp            ServiceType = "php"
	ServiceTypeMySql          ServiceType = "mysql"
	ServiceTypePostgreSql     ServiceType = "postgresql"
	ServiceTypeSupervisor     ServiceType = "process_manager"
	ServiceTypeRedis          ServiceType = "memory_database"
	ServiceTypeCaddy          ServiceType = "webserver"
	ServiceTypeComposer       ServiceType = "package_manager"
	ServiceTypeNode           ServiceType = "node"
	ServiceTypeBun            ServiceType = "bun"
	ServiceTypeLaunchAgent    ServiceType = "launch_agent"
)

func (s ServiceType) String() string {
	return string(s)
}

func (s ServiceType) Label() string {
	labels := map[ServiceType]string{
		ServiceTypePhp:         "PHP",
		ServiceTypeMySql:       "MySQL",
		ServiceTypePostgreSql:  "PostgreSQL",
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
	case ServiceTypePhp, ServiceTypeMySql, ServiceTypePostgreSql,
		ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
		ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent:
		return true
	}
	return false
}

func (s ServiceType) IsDatabase() bool {
	return s == ServiceTypeMySql || s == ServiceTypePostgreSql
}

func (s ServiceType) GetDatabasePort() int {
	switch s {
	case ServiceTypeMySql:
		return 3306
	case ServiceTypePostgreSql:
		return 5432
	}
	return 0
}

func (s ServiceType) GetDatabaseConnection() string {
	switch s {
	case ServiceTypeMySql:
		return "mysql"
	case ServiceTypePostgreSql:
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
		ServiceTypePhp, ServiceTypeMySql, ServiceTypePostgreSql,
		ServiceTypeSupervisor, ServiceTypeRedis, ServiceTypeCaddy,
		ServiceTypeComposer, ServiceTypeNode, ServiceTypeBun, ServiceTypeLaunchAgent,
	}
}

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

// Software represents installable software
type Software string

const (
	SoftwareCaddy2       Software = "caddy2"
	SoftwareComposer2    Software = "composer2"
	SoftwareMySql80      Software = "mysql80"
	SoftwarePostgreSql16 Software = "postgresql16"
	SoftwareNode21       Software = "node21"
	SoftwareBun          Software = "bun"
	SoftwarePhp56        Software = "php56"
	SoftwarePhp70        Software = "php70"
	SoftwarePhp71        Software = "php71"
	SoftwarePhp72        Software = "php72"
	SoftwarePhp73        Software = "php73"
	SoftwarePhp74        Software = "php74"
	SoftwarePhp80        Software = "php80"
	SoftwarePhp81        Software = "php81"
	SoftwarePhp82        Software = "php82"
	SoftwarePhp83        Software = "php83"
	SoftwarePhp84        Software = "php84"
	SoftwareRedis        Software = "redis"
	SoftwareSupervisor   Software = "supervisor"
	SoftwareLaunchAgent  Software = "launch_agent"
)

func (s Software) String() string {
	return string(s)
}

func (s Software) Label() string {
	labels := map[Software]string{
		SoftwareCaddy2:       "Caddy 2",
		SoftwareComposer2:    "Composer 2",
		SoftwareMySql80:      "MySQL 8.0",
		SoftwarePostgreSql16: "PostgreSQL 16",
		SoftwareNode21:       "Node 21",
		SoftwareBun:          "Bun",
		SoftwarePhp56:        "PHP 5.6",
		SoftwarePhp70:        "PHP 7.0",
		SoftwarePhp71:        "PHP 7.1",
		SoftwarePhp72:        "PHP 7.2",
		SoftwarePhp73:        "PHP 7.3",
		SoftwarePhp74:        "PHP 7.4",
		SoftwarePhp80:        "PHP 8.0",
		SoftwarePhp81:        "PHP 8.1",
		SoftwarePhp82:        "PHP 8.2",
		SoftwarePhp83:        "PHP 8.3",
		SoftwarePhp84:        "PHP 8.4",
		SoftwareRedis:        "Redis",
		SoftwareSupervisor:   "Supervisor",
		SoftwareLaunchAgent:  "Launch Agent",
	}
	if label, ok := labels[s]; ok {
		return label
	}
	return "Unknown"
}

func (s Software) IsValid() bool {
	switch s {
	case SoftwareCaddy2, SoftwareComposer2, SoftwareMySql80, SoftwarePostgreSql16,
		SoftwareNode21, SoftwareBun, SoftwarePhp56, SoftwarePhp70, SoftwarePhp71,
		SoftwarePhp72, SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84, SoftwareRedis, SoftwareSupervisor,
		SoftwareLaunchAgent:
		return true
	}
	return false
}

func (s Software) GetVersion() string {
	versions := map[Software]string{
		SoftwarePhp56:        "5.6",
		SoftwarePhp70:        "7.0",
		SoftwarePhp71:        "7.1",
		SoftwarePhp72:        "7.2",
		SoftwarePhp73:        "7.3",
		SoftwarePhp74:        "7.4",
		SoftwarePhp80:        "8.0",
		SoftwarePhp81:        "8.1",
		SoftwarePhp82:        "8.2",
		SoftwarePhp83:        "8.3",
		SoftwarePhp84:        "8.4",
		SoftwareMySql80:      "8.0",
		SoftwarePostgreSql16: "16",
		SoftwareSupervisor:   "latest",
		SoftwareComposer2:    "2.0",
		SoftwareCaddy2:       "2.0",
		SoftwareNode21:       "21",
		SoftwareBun:          "latest",
		SoftwareRedis:        "latest",
		SoftwareLaunchAgent:  "latest",
	}
	if v, ok := versions[s]; ok {
		return v
	}
	return "1.0"
}

func (s Software) GetServiceType() ServiceType {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return ServiceTypePhp
	case SoftwareMySql80:
		return ServiceTypeMySql
	case SoftwarePostgreSql16:
		return ServiceTypePostgreSql
	case SoftwareSupervisor:
		return ServiceTypeSupervisor
	case SoftwareRedis:
		return ServiceTypeRedis
	case SoftwareCaddy2:
		return ServiceTypeCaddy
	case SoftwareComposer2:
		return ServiceTypeComposer
	case SoftwareNode21:
		return ServiceTypeNode
	case SoftwareBun:
		return ServiceTypeBun
	case SoftwareLaunchAgent:
		return ServiceTypeLaunchAgent
	}
	return ""
}

func (s Software) IsPhp() bool {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return true
	}
	return false
}

func (s Software) IsDatabase() bool {
	return s == SoftwareMySql80 || s == SoftwarePostgreSql16
}

func (s Software) Group() string {
	switch s {
	case SoftwarePhp56, SoftwarePhp70, SoftwarePhp71, SoftwarePhp72,
		SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84:
		return "php"
	case SoftwareMySql80:
		return "mysql"
	case SoftwarePostgreSql16:
		return "postgresql"
	case SoftwareSupervisor:
		return "supervisor"
	case SoftwareRedis:
		return "redis"
	case SoftwareCaddy2:
		return "caddy"
	case SoftwareComposer2:
		return "composer"
	case SoftwareNode21:
		return "node"
	case SoftwareBun:
		return "bun"
	case SoftwareLaunchAgent:
		return "launch-agent"
	}
	return ""
}

func (s *Software) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*s = Software(v)
	case string:
		*s = Software(v)
	default:
		return fmt.Errorf("cannot scan type %T into Software", value)
	}
	return nil
}

func (s Software) Value() (driver.Value, error) {
	return string(s), nil
}

func ParseSoftware(str string) (Software, error) {
	s := Software(str)
	if !s.IsValid() {
		return "", fmt.Errorf("invalid software: %s", str)
	}
	return s, nil
}

func AllSoftware() []Software {
	return []Software{
		SoftwareCaddy2, SoftwareComposer2, SoftwareMySql80, SoftwarePostgreSql16,
		SoftwareNode21, SoftwareBun, SoftwarePhp56, SoftwarePhp70, SoftwarePhp71,
		SoftwarePhp72, SoftwarePhp73, SoftwarePhp74, SoftwarePhp80, SoftwarePhp81,
		SoftwarePhp82, SoftwarePhp83, SoftwarePhp84, SoftwareRedis, SoftwareSupervisor,
		SoftwareLaunchAgent,
	}
}

func AllPhpVersions() []Software {
	return []Software{
		SoftwarePhp84, SoftwarePhp83, SoftwarePhp82, SoftwarePhp81, SoftwarePhp80,
		SoftwarePhp74, SoftwarePhp73, SoftwarePhp72, SoftwarePhp71, SoftwarePhp70, SoftwarePhp56,
	}
}

func AllDatabaseTypes() []Software {
	return []Software{SoftwareMySql80, SoftwarePostgreSql16}
}

// ServerFeature represents available server features
type ServerFeature string

const (
	ServerFeatureSites              ServerFeature = "sites"
	ServerFeatureSslCertificates    ServerFeature = "ssl_certificates"
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
		ServerFeatureSslCertificates:    "SSL Certificates",
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
	case ServerFeatureSites, ServerFeatureSslCertificates, ServerFeaturePhpManagement,
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
		ServerFeatureSslCertificates:    "ssl",
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
		ServerFeatureSites, ServerFeatureSslCertificates, ServerFeaturePhpManagement,
		ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
		ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
		ServerFeatureServices,
	}
}

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
