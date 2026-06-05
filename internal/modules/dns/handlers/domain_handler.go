package handlers

import (
	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DomainHandler holds the only domain endpoint that does not fit a generic
// route helper: DeleteDomain accepts an optional body controlling whether
// the domain should also be removed at the provider.
type DomainHandler struct {
	domainService *services.DomainService
}

// NewDomainHandler creates a new DomainHandler.
func NewDomainHandler(domainService *services.DomainService) *DomainHandler {
	return &DomainHandler{domainService: domainService}
}

// DeleteDomain removes a domain. The optional JSON body
// `{ "delete_from_provider": true }` also deletes the domain at the
// provider before removing it locally.
func (h *DomainHandler) DeleteDomain(r *fiberutil.Request) error {
	var req dto.DeleteDomainRequest
	_ = r.BodyParser(&req) // body is optional; defaults to false

	if err := h.domainService.DeleteDomain(r.Context(), r.Params("id"), r.TeamID, r.UserID, req.DeleteFromProvider); err != nil {
		return err
	}
	return fiberutil.NoContent(r.Ctx)
}
