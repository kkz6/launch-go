package dto

// CreateServerRequest represents the request body for creating a server
type CreateServerRequest struct {
	Name            string   `json:"name" validate:"required,min=2,max=255"`
	Description     *string  `json:"description" validate:"omitempty,max=1000"`
	Provider        string   `json:"provider" validate:"required,oneof=digitalocean hetzner linode vultr aws custom_server"`
	Type            string   `json:"type" validate:"required,oneof=php database"`
	OperatingSystem string   `json:"operating_system" validate:"omitempty,oneof=ubuntu_20 ubuntu_22 ubuntu_24"`
	Region          string   `json:"region" validate:"required_unless=Provider custom_server"`
	Size            string   `json:"size" validate:"required_unless=Provider custom_server"`
	PHPVersion      string   `json:"php_version" validate:"omitempty,oneof=5.6 7.0 7.1 7.2 7.3 7.4 8.0 8.1 8.2 8.3 8.4"`
	DatabaseType    string   `json:"database_type" validate:"omitempty,oneof=mysql80 postgresql16"`
	CredentialID    string   `json:"credential_id" validate:"required_unless=Provider custom_server,omitempty,ulid"`
	SSHKeyIDs       []string `json:"ssh_key_ids" validate:"omitempty"`

	// For custom servers
	IPAddress  string `json:"ip_address" validate:"required_if=Provider custom_server,omitempty,ip"`
	SSHPort    int    `json:"ssh_port" validate:"omitempty,min=1,max=65535"`
	SSHUser    string `json:"ssh_user" validate:"omitempty,max=100"`
	PrivateKey string `json:"private_key" validate:"required_if=Provider custom_server"`
}

// UpdateServerRequest represents the request body for updating a server
type UpdateServerRequest struct {
	Name              *string `json:"name" validate:"omitempty,min=2,max=255"`
	Description       *string `json:"description" validate:"omitempty,max=1000"`
	MonitoringEnabled *bool   `json:"monitoring_enabled"`
	AutoUpdate        *bool   `json:"auto_update"`
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

// UpdateFirewallRuleRequest represents the request body for updating a firewall rule
type UpdateFirewallRuleRequest struct {
	Name     *string `json:"name" validate:"omitempty,min=1,max=255"`
	Action   *string `json:"action" validate:"omitempty,oneof=allow deny reject"`
	Port     *string `json:"port" validate:"omitempty,max=50"`
	FromIPv4 *string `json:"from_ipv4" validate:"omitempty,ip|cidr"`
	Mask     *string `json:"mask" validate:"omitempty,max=10"`
	Note     *string `json:"note" validate:"omitempty,max=500"`
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

// CreateSshKeyRequest represents the request body for creating an SSH key
type CreateSshKeyRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	PublicKey   string  `json:"public_key" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	IsGlobal    bool    `json:"is_global"`
}

// AttachSshKeyRequest represents the request body for attaching an SSH key to a server
type AttachSshKeyRequest struct {
	SshKeyID string `json:"ssh_key_id" validate:"required,ulid"`
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
	Enabled                bool   `json:"enabled"`
	EnableCLI              bool   `json:"enable_cli"`
	MemoryConsumption      int    `json:"memory_consumption" validate:"omitempty,min=32,max=1024"`
	InternedStringsBuffer  int    `json:"interned_strings_buffer" validate:"omitempty,min=4,max=128"`
	MaxAcceleratedFiles    int    `json:"max_accelerated_files" validate:"omitempty,min=200,max=1000000"`
	ValidateTimestamps     bool   `json:"validate_timestamps"`
	RevalidateFreq         int    `json:"revalidate_freq" validate:"omitempty,min=0,max=3600"`
	SaveComments           bool   `json:"save_comments"`
	JITEnabled             bool   `json:"jit_enabled"`
	JITBufferSize          string `json:"jit_buffer_size" validate:"omitempty,max=10"`
	JITMode                string `json:"jit_mode" validate:"omitempty,oneof=disable tracing function"`
}

// UpdateComposerAuthRequest represents the request body for updating Composer auth.json
type UpdateComposerAuthRequest struct {
	Contents string `json:"contents" validate:"required"`
}
