package dto

type ContaboVPS struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IP               string `json:"ip,omitempty"`
	Region           string `json:"region"`
	Plan             string `json:"plan"`
	Status           string `json:"status"`
	OSType           string `json:"os_type"`
	CanImport        bool   `json:"can_import"`
	ImportedServerID string `json:"imported_server_id,omitempty"`
}

type ContaboVPSPage struct {
	Instances  []ContaboVPS `json:"instances"`
	Page       int          `json:"page"`
	TotalPages int          `json:"total_pages"`
}

type ImportContaboVPSRequest struct {
	InstanceID      string `json:"instance_id" validate:"required,numeric,max=19"`
	Type            string `json:"type" validate:"required,oneof=php database loadbalancer docker"`
	OperatingSystem string `json:"operating_system" validate:"required,oneof=ubuntu_22 ubuntu_24"`
	SSHPort         int    `json:"port" validate:"omitempty,min=1,max=65535"`
}
