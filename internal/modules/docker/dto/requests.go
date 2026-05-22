package dto

// CreateProjectRequest is the request body for creating a docker project.
type CreateProjectRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// UpdateProjectRequest is the partial-update body for a project.
// Both fields are optional; only present keys are applied.
type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}
