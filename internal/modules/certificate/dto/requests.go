// Package dto contains the request and response types for the
// certificate module's HTTP API.
package dto

// CreateStoredCertificateRequest is the request body for
// POST /api/certificates. Name + cert PEM + private key PEM are
// required; notes is optional.
type CreateStoredCertificateRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Notes       *string `json:"notes" validate:"omitempty,max=2000"`
	Certificate string  `json:"certificate" validate:"required"`
	PrivateKey  string  `json:"private_key" validate:"required"`
}

// UpdateStoredCertificateRequest is the request body for
// PATCH /api/certificates/:id. All fields are optional; updating
// cert+key together replaces the pair and triggers re-parse +
// fanout (handled in Task 2.3 + Phase 6).
type UpdateStoredCertificateRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=255"`
	Notes       *string `json:"notes" validate:"omitempty,max=2000"`
	Certificate *string `json:"certificate" validate:"omitempty"`
	PrivateKey  *string `json:"private_key" validate:"omitempty"`
}
