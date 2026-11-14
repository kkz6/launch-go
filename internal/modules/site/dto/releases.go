package dto

// ReleaseInventoryResponse is a point-in-time observation of the server. An
// unavailable observation never substitutes deployment history for live state.
type ReleaseInventoryResponse struct {
	Status        string            `json:"status"`
	CheckedAt     string            `json:"checked_at"`
	ActiveRelease *string           `json:"active_release"`
	Releases      []RetainedRelease `json:"releases"`
}

type RetainedRelease struct {
	ReleaseName       string   `json:"release_name"`
	DeploymentID      string   `json:"deployment_id,omitempty"`
	DeploymentIDs     []string `json:"deployment_ids"`
	Status            string   `json:"status,omitempty"`
	Active            bool     `json:"active"`
	RollbackAvailable bool     `json:"rollback_available"`
	UnavailableReason string   `json:"unavailable_reason,omitempty"`
}
