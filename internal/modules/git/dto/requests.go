package dto

// ConnectProviderRequest is the request to connect a git provider
type ConnectProviderRequest struct {
	Provider       string `json:"provider" validate:"required,oneof=github gitlab bitbucket"`
	InstallationID string `json:"installation_id" validate:"required"`
}

// RefreshRepositoriesRequest is the request to refresh repositories
type RefreshRepositoriesRequest struct {
	Provider       string `json:"provider" validate:"required,oneof=github gitlab bitbucket"`
	InstallationID string `json:"installation_id" validate:"required"`
}
