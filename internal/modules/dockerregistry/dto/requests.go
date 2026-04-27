package dto

import "github.com/kkz6/launch-go/internal/modules/dockerregistry/types"

// CreateCredentialRequest is the body of POST /docker-registries.
type CreateCredentialRequest struct {
	Name     string     `json:"name" validate:"required,min=1,max=255"`
	Type     types.Type `json:"type" validate:"required"`
	URL      string     `json:"url" validate:"omitempty,max=255"`
	Username string     `json:"username" validate:"required,min=1,max=255"`
	Password string     `json:"password" validate:"required,min=1,max=4096"`
}

// UpdateCredentialRequest is the body of PUT /docker-registries/:id.
// Password is optional; an empty string means "do not change".
type UpdateCredentialRequest struct {
	Name     string `json:"name" validate:"omitempty,min=1,max=255"`
	URL      string `json:"url" validate:"omitempty,max=255"`
	Username string `json:"username" validate:"omitempty,min=1,max=255"`
	Password string `json:"password" validate:"omitempty,min=1,max=4096"`
}
