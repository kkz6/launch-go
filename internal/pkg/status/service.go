package status

// ServiceStatus represents the status of a service running on a server.
type ServiceStatus struct {
	ID       string `json:"id"`
	Software string `json:"software"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	IsActive bool   `json:"is_active"`
	Memory   string `json:"memory,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
	PID      int    `json:"pid,omitempty"`
}
