package dto

// CreateServerRequest represents the request body for creating a server
type CreateServerRequest struct {
	Name            string   `json:"name" validate:"required,min=2,max=255"`
	Description     *string  `json:"description" validate:"omitempty,max=1000"`
	Provider        string   `json:"provider" validate:"required,oneof=digitalocean hetzner linode vultr aws custom_server"`
	Type            string   `json:"type" validate:"required,oneof=php database loadbalancer"`
	OperatingSystem string   `json:"operating_system" validate:"omitempty,oneof=ubuntu_20 ubuntu_22 ubuntu_24"`
	Region          string   `json:"region" validate:"required_unless=Provider custom_server"`
	Size            string   `json:"size" validate:"required_unless=Provider custom_server"`
	PHPVersion      string   `json:"php_version" validate:"omitempty,oneof=none php56 php70 php71 php72 php73 php74 php80 php81 php82 php83 php84"`
	DatabaseType    string   `json:"database_type" validate:"omitempty,oneof=none mysql80 postgresql16"`
	CredentialID    string   `json:"credential_id" validate:"required_unless=Provider custom_server,omitempty,ulid"`
	SSHKeyIDs       []string `json:"ssh_key_ids" validate:"omitempty"`
	InstallAgent    *bool    `json:"install_agent"`

	// For custom servers (key pair is auto-generated, not user-provided)
	IP      string `json:"ip" validate:"required_if=Provider custom_server,omitempty,ip"`
	SSHPort int    `json:"port" validate:"omitempty,min=1,max=65535"`
	SSHUser string `json:"ssh_user" validate:"omitempty,max=100"`
}

// UpdateServerRequest represents the request body for updating a server
type UpdateServerRequest struct {
	Name              *string `json:"name" validate:"omitempty,min=2,max=255"`
	Description       *string `json:"description" validate:"omitempty,max=1000"`
	MonitoringEnabled *bool   `json:"monitoring_enabled"`
	AutoUpdate        *bool   `json:"auto_update"`
	SSHPort           *int    `json:"ssh_port" validate:"omitempty,min=1,max=65535"`
}

// ArchiveServerRequest represents the request body for archiving a server
type ArchiveServerRequest struct {
	Confirm bool `json:"confirm" validate:"required,eq=true"`
}

// CreateServiceRequest represents the request body for installing a service
type CreateServiceRequest struct {
	Software string `json:"software" validate:"required"`
}

// ServiceOperationRequest represents the request body for service operations
type ServiceOperationRequest struct {
	Operation string `json:"operation" validate:"required,oneof=start stop restart remove status"`
}

// CreateFirewallRuleRequest represents the request body for creating a firewall rule
type CreateFirewallRuleRequest struct {
	Name     string  `json:"name" validate:"required,min=1,max=255"`
	Action   string  `json:"action" validate:"required,oneof=allow deny reject"`
	Port     string  `json:"port" validate:"required,max=50"`
	FromIPv4 *string `json:"from_ipv4" validate:"omitempty,ip|cidr"`
	Mask     *string `json:"mask" validate:"omitempty,max=10"`
	Note     *string `json:"note" validate:"omitempty,max=500"`
}

// UpdateFirewallRuleRequest represents the request body for updating a firewall rule.
// Only name and note can be updated — action, port, and IP are immutable after creation
// because changing them would require uninstalling the old UFW rule and reinstalling the new one.
type UpdateFirewallRuleRequest struct {
	Name *string `json:"name" validate:"omitempty,min=1,max=255"`
	Note *string `json:"note" validate:"omitempty,max=500"`
}

// CreateCronRequest represents the request body for creating a cron job
type CreateCronRequest struct {
	User       string  `json:"user" validate:"omitempty,max=100"`
	Expression string  `json:"expression" validate:"required,max=100"`
	Command    string  `json:"command" validate:"required"`
	Frequency  *string `json:"frequency" validate:"omitempty,max=100"`
	SiteID     *string `json:"site_id" validate:"omitempty,ulid"`
}

// UpdateCronRequest represents the request body for updating a cron job
type UpdateCronRequest struct {
	User       *string `json:"user" validate:"omitempty,max=100"`
	Expression *string `json:"expression" validate:"omitempty,max=100"`
	Command    *string `json:"command"`
	Frequency  *string `json:"frequency" validate:"omitempty,max=100"`
}

// CreateDaemonRequest represents the request body for creating a daemon
type CreateDaemonRequest struct {
	User            string  `json:"user" validate:"omitempty,max=100"`
	Directory       *string `json:"directory" validate:"omitempty,max=255"`
	Command         string  `json:"command" validate:"required"`
	Processes       int     `json:"processes" validate:"omitempty,min=1,max=100"`
	StopWaitSeconds int     `json:"stop_wait_seconds" validate:"omitempty,min=1,max=600"`
	StopSignal      *string `json:"stop_signal" validate:"omitempty,max=20"`
}

// UpdateDaemonRequest represents the request body for updating a daemon
type UpdateDaemonRequest struct {
	User            *string `json:"user" validate:"omitempty,max=100"`
	Directory       *string `json:"directory" validate:"omitempty,max=255"`
	Command         *string `json:"command"`
	Processes       *int    `json:"processes" validate:"omitempty,min=1,max=100"`
	StopWaitSeconds *int    `json:"stop_wait_seconds" validate:"omitempty,min=1,max=600"`
	StopSignal      *string `json:"stop_signal" validate:"omitempty,max=20"`
}

// CreateSSHKeyRequest represents the request body for creating an SSH key
type CreateSSHKeyRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	PublicKey   string  `json:"public_key" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	IsGlobal    bool    `json:"is_global"`
}

// GenerateSSHKeyRequest represents the request body for generating an SSH key pair
type GenerateSSHKeyRequest struct {
	Type string `json:"type" validate:"omitempty,oneof=rsa ed25519"`
}

// AttachSSHKeyRequest represents the request body for attaching an SSH key to a server
type AttachSSHKeyRequest struct {
	SSHKeyID string `json:"ssh_key_id" validate:"required,ulid"`
}

// RunCommandRequest represents the request body for running a command on a server
type RunCommandRequest struct {
	Command string `json:"command" validate:"required"`
	User    string `json:"user" validate:"omitempty,max=100"`
}

// CreateDatabaseRequest represents the request body for creating a database
type CreateDatabaseRequest struct {
	Name string `json:"name" validate:"required,min=1,max=64"`
}

// ConfigureOpcacheRequest represents the request body for configuring OPcache
type ConfigureOpcacheRequest struct {
	Enabled               bool   `json:"enabled"`
	EnableCLI             bool   `json:"enable_cli"`
	MemoryConsumption     int    `json:"memory_consumption" validate:"omitempty,min=32,max=1024"`
	InternedStringsBuffer int    `json:"interned_strings_buffer" validate:"omitempty,min=4,max=128"`
	MaxAcceleratedFiles   int    `json:"max_accelerated_files" validate:"omitempty,min=200,max=1000000"`
	ValidateTimestamps    bool   `json:"validate_timestamps"`
	RevalidateFreq        int    `json:"revalidate_freq" validate:"omitempty,min=0,max=3600"`
	SaveComments          bool   `json:"save_comments"`
	JITEnabled            bool   `json:"jit_enabled"`
	JITBufferSize         string `json:"jit_buffer_size" validate:"omitempty,max=10"`
	JITMode               string `json:"jit_mode" validate:"omitempty,oneof=disable tracing function"`
}

// UpdateComposerAuthRequest represents the request body for updating Composer auth.json
type UpdateComposerAuthRequest struct {
	Contents string `json:"contents" validate:"required"`
}

// VulnerabilityAuditRequest represents the request body for running a vulnerability audit
type VulnerabilityAuditRequest struct {
	Email *string `json:"email" validate:"omitempty,email"`
}

// InstallPhpExtensionRequest represents the request body for installing a PHP extension
type InstallPhpExtensionRequest struct {
	Extension string `json:"extension" validate:"required,min=1,max=50"`
}

// CreateUpstreamRequest represents the request body for creating a load balancer upstream
type CreateUpstreamRequest struct {
	Name                 string `json:"name" validate:"required,min=1,max=255"`
	Address              string `json:"address" validate:"required,hostname_rfc1123"`
	Port                 int    `json:"port" validate:"omitempty,min=1,max=65535"`
	TLSSetting           string `json:"tls_setting" validate:"omitempty,oneof=auto custom internal off"`
	LBPolicy             string `json:"lb_policy" validate:"required,oneof=round_robin least_conn ip_hash first random"`
	HealthCheckPath      string `json:"health_check_path" validate:"omitempty,startswith=/,max=255,excludesall=\n\r"`
	HealthCheckInterval  string `json:"health_check_interval" validate:"omitempty,max=20,oneof=5s 10s 15s 20s 30s 45s 1m 2m 5m"`
	HealthCheckTimeout   string `json:"health_check_timeout" validate:"omitempty,max=20,oneof=5s 10s 15s 20s 30s 45s 1m 2m 5m"`
	AutoAddExistingSites bool   `json:"auto_add_existing_sites"`
}

// UpdateUpstreamRequest represents the request body for updating an upstream
type UpdateUpstreamRequest struct {
	Name                *string `json:"name" validate:"omitempty,min=1,max=255"`
	TLSSetting          *string `json:"tls_setting" validate:"omitempty,oneof=auto custom internal off"`
	LBPolicy            *string `json:"lb_policy" validate:"omitempty,oneof=round_robin least_conn ip_hash first random"`
	HealthCheckPath     *string `json:"health_check_path" validate:"omitempty,startswith=/,max=255,excludesall=\n\r"`
	HealthCheckInterval *string `json:"health_check_interval" validate:"omitempty,max=20,oneof=5s 10s 15s 20s 30s 45s 1m 2m 5m"`
	HealthCheckTimeout  *string `json:"health_check_timeout" validate:"omitempty,max=20,oneof=5s 10s 15s 20s 30s 45s 1m 2m 5m"`
}

// AddBackendRequest represents the request body for adding a backend to an upstream
type AddBackendRequest struct {
	SiteID string `json:"site_id" validate:"required,ulid"`
	Port   int    `json:"port" validate:"omitempty,min=1,max=65535"`
}

// UpdateBackendRequest represents the request body for updating a backend
type UpdateBackendRequest struct {
	Port   *int  `json:"port" validate:"omitempty,min=1,max=65535"`
	IsDown *bool `json:"is_down"`
}
