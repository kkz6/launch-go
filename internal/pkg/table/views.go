package table

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ViewDTO is the API-facing shape; the payload is unmarshalled JSON.
type ViewDTO struct {
	ID             string         `json:"id"`
	UserID         *string        `json:"user_id,omitempty"`
	TableKey       string         `json:"table_key"`
	Title          string         `json:"title"`
	RequestPayload map[string]any `json:"request_payload"`
	CreatedAt      *time.Time     `json:"created_at,omitempty"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
}

// StoreViewRequest is the POST body.
type StoreViewRequest struct {
	Title          string         `json:"title" validate:"required,max=160"`
	RequestPayload map[string]any `json:"request_payload" validate:"required"`
}

// ViewService owns CRUD on saved views.
//
// TODO: persist saved views (table_views migration) — stubbed for now.
type ViewService struct {
	db *gorm.DB
}

// NewViewService wires the service with the global GORM connection.
func NewViewService(db *gorm.DB) *ViewService { return &ViewService{db: db} }

// List returns every view a user has saved against the given table.
// Pass userID empty for "global" / unscoped lookups.
//
// TODO: persist saved views (table_views migration) — stubbed for now.
func (s *ViewService) List(_ context.Context, _, _ string) ([]ViewDTO, error) {
	return []ViewDTO{}, nil
}

// Create persists a new view.
//
// TODO: persist saved views (table_views migration) — stubbed for now.
func (s *ViewService) Create(_ context.Context, _, _ string, _ StoreViewRequest) (*ViewDTO, error) {
	return nil, fiber.NewError(fiber.StatusNotImplemented, "saved views not enabled")
}

// Delete removes a view; callers should pass the userID so users can only
// delete their own saved views.
//
// TODO: persist saved views (table_views migration) — stubbed for now.
func (s *ViewService) Delete(_ context.Context, _, _, _ string) error {
	return nil
}
