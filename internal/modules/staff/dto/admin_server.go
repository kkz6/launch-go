package dto

import "time"

// AdminServerOwner identifies who a server belongs to: the owning team and the
// user the server is scoped to. Only display fields are exposed.
type AdminServerOwner struct {
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
	PersonalTeam bool   `json:"personal_team"`
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name"`
	UserEmail    string `json:"user_email"`
}

// AdminServerDetail is the back-office detail view of a server. It exposes only
// an allow-list of non-sensitive fields — never the private key, host key,
// passwords, launch token, provider credentials or private IP.
type AdminServerDetail struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Provider    string  `json:"provider"`
	Type        *string `json:"type,omitempty"`
	Status      string  `json:"status"`
	Connected   bool    `json:"connected"`
	PublicIPv4  *string `json:"public_ipv4,omitempty"`

	CPUCores    *int `json:"cpu_cores,omitempty"`
	MemoryInMB  *int `json:"memory_in_mb,omitempty"`
	StorageInGB *int `json:"storage_in_gb,omitempty"`

	OperatingSystem   *string `json:"operating_system,omitempty"`
	DetectedOSID      *string `json:"detected_os_id,omitempty"`
	DetectedOSVersion *string `json:"detected_os_version,omitempty"`
	DetectedArch      *string `json:"detected_arch,omitempty"`
	DetectedKernel    *string `json:"detected_kernel,omitempty"`

	MonitoringEnabled bool `json:"monitoring_enabled"`
	AutoUpdate        bool `json:"auto_update"`

	ProvisionedAt         *time.Time `json:"provisioned_at,omitempty"`
	LastConnectivityCheck *time.Time `json:"last_connectivity_check,omitempty"`
	CreatedAt             *time.Time `json:"created_at,omitempty"`

	Owner AdminServerOwner `json:"owner"`
}
