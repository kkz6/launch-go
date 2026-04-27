package dto

import (
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// CredentialResponse is the JSON shape returned to clients. The password
// is intentionally never included.
type CredentialResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	TypeLabel string  `json:"type_label"`
	URL       string  `json:"url"`
	Username  string  `json:"username"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// ToCredentialResponse converts a model to its API response shape.
func ToCredentialResponse(c *models.Credential) CredentialResponse {
	return CredentialResponse{
		ID:        c.ID,
		Name:      c.Name,
		Type:      c.Type.String(),
		TypeLabel: c.Type.Label(),
		URL:       c.URL,
		Username:  c.Username.String(),
		CreatedAt: pkgdto.FormatTime(c.CreatedAt),
		UpdatedAt: pkgdto.FormatTime(c.UpdatedAt),
	}
}

// ToCredentialResponseList converts a slice of models.
func ToCredentialResponseList(in []models.Credential) []CredentialResponse {
	out := make([]CredentialResponse, len(in))
	for i := range in {
		out[i] = ToCredentialResponse(&in[i])
	}
	return out
}
