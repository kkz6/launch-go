package handlers

import "github.com/kkz6/launch-go/internal/modules/auth/services"

// BaseHandler provides common dependencies for auth handlers.
// Embed this in handlers to avoid repeating service field declarations.
type BaseHandler struct {
	service *services.Service
}

// NewBaseHandler creates a new base handler with the given service
func NewBaseHandler(service *services.Service) BaseHandler {
	return BaseHandler{service: service}
}

// Service returns the auth service
func (h *BaseHandler) Service() *services.Service {
	return h.service
}
