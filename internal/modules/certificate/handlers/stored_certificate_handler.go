// Package handlers contains the HTTP handlers for the certificate
// module. Routes are mounted by Module.RegisterRoutes onto
// /api/certificates with the AuthenticatedChain (auth + team + subscription).
package handlers

import (
	"errors"
	"strconv"

	gofiber "github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/certificate/dto"
	"github.com/kkz6/launch-go/internal/modules/certificate/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// StoredCertificateHandler binds the certificate service to HTTP.
type StoredCertificateHandler struct {
	svc *services.StoredCertificateService
}

func NewStoredCertificateHandler(svc *services.StoredCertificateService) *StoredCertificateHandler {
	return &StoredCertificateHandler{svc: svc}
}

// List returns the team's stored certificates.
func (h *StoredCertificateHandler) List(r *fiberutil.Request) error {
	rows, err := h.svc.List(r.Context(), r.TeamID)
	if err != nil {
		return err
	}
	return fiberutil.OK(r.Ctx, "Stored certificates retrieved", rows)
}

// Get returns one stored certificate.
func (h *StoredCertificateHandler) Get(r *fiberutil.Request) error {
	row, err := h.svc.Get(r.Context(), r.TeamID, r.Params("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound("Stored certificate not found")
		}
		return err
	}
	return fiberutil.OK(r.Ctx, "Stored certificate retrieved", row)
}

// Create saves a new stored certificate. Returns 201 on success, 409
// with {existing:{id,name}} on fingerprint dedupe, 422 on validation
// failure / cert parse failure / key mismatch.
func (h *StoredCertificateHandler) Create(r *fiberutil.Request, req *dto.CreateStoredCertificateRequest) error {
	uid := r.UserID
	row, err := h.svc.Create(r.Context(), r.TeamID, &uid, *req)
	if err != nil {
		return mapCreateOrUpdateError(r.Ctx, err)
	}
	return fiberutil.Created(r.Ctx, "Stored certificate created", row)
}

// Update modifies an existing stored certificate. Same error mapping as Create.
func (h *StoredCertificateHandler) Update(r *fiberutil.Request, req *dto.UpdateStoredCertificateRequest) error {
	row, pendingRedeploys, err := h.svc.Update(r.Context(), r.TeamID, r.Params("id"), *req)
	if err != nil {
		return mapCreateOrUpdateError(r.Ctx, err)
	}
	r.Set("X-Pending-Redeploys", strconv.Itoa(pendingRedeploys))
	return fiberutil.OK(r.Ctx, "Stored certificate updated", row)
}

// Delete soft-deletes a stored certificate. If ?force=true is set,
// referenced sites/domains are flipped back to Let's Encrypt first
// (cascading via DeleteWithForce). Without force, returns 409 with
// {usages:[...]} when the cert is still referenced.
func (h *StoredCertificateHandler) Delete(r *fiberutil.Request) error {
	id := r.Params("id")
	force := r.Query("force") == "true" || r.Query("force") == "1"

	if force {
		if err := h.svc.DeleteWithForce(r.Context(), r.TeamID, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiberutil.NotFound("Stored certificate not found")
			}
			return err
		}
		return fiberutil.NoContent(r.Ctx)
	}

	if err := h.svc.Delete(r.Context(), r.TeamID, id); err != nil {
		var inUseErr services.ErrInUse
		if errors.As(err, &inUseErr) {
			return r.Status(gofiber.StatusConflict).JSON(gofiber.Map{
				"message": "Stored certificate is still in use",
				"usages":  inUseErr.Usages,
			})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound("Stored certificate not found")
		}
		return err
	}
	return fiberutil.NoContent(r.Ctx)
}

// Usages returns the list of sites and docker domains that reference
// the stored certificate. Used by the UI's "in use by N resources"
// badge and by the delete confirmation dialog.
func (h *StoredCertificateHandler) Usages(r *fiberutil.Request) error {
	usages, err := h.svc.Usages(r.Context(), r.TeamID, r.Params("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound("Stored certificate not found")
		}
		return err
	}
	return fiberutil.OK(r.Ctx, "Usages retrieved", usages)
}

// mapCreateOrUpdateError maps service errors to HTTP responses:
//   - ErrDuplicateFingerprint   → 409 with {existing:{id,name}}
//   - ErrPartialCertKeyUpdate   → 422 with field error
//   - ErrInvalidCertificatePEM  → 422 with field error
//   - ErrPrivateKeyMismatch     → 422 with field error
//   - gorm.ErrRecordNotFound    → 404
//   - everything else           → fall through to the global error handler
func mapCreateOrUpdateError(c *gofiber.Ctx, err error) error {
	var dupErr services.ErrDuplicateFingerprint
	if errors.As(err, &dupErr) {
		body := gofiber.Map{
			"message": "Certificate already exists in this team",
		}
		if dupErr.Existing != nil {
			body["existing"] = gofiber.Map{
				"id":   dupErr.Existing.ID,
				"name": dupErr.Existing.Name,
			}
		}
		return c.Status(gofiber.StatusConflict).JSON(body)
	}
	if errors.Is(err, services.ErrPartialCertKeyUpdate) {
		return c.Status(gofiber.StatusUnprocessableEntity).JSON(gofiber.Map{
			"message": "certificate and private_key must be updated together",
			"errors": gofiber.Map{
				"certificate": "must be sent together with private_key",
				"private_key": "must be sent together with certificate",
			},
		})
	}
	if errors.Is(err, services.ErrInvalidCertificatePEM) {
		return c.Status(gofiber.StatusUnprocessableEntity).JSON(gofiber.Map{
			"message": "Invalid certificate PEM",
			"errors": gofiber.Map{
				"certificate": "not a valid PEM-encoded certificate",
			},
		})
	}
	if errors.Is(err, services.ErrPrivateKeyMismatch) {
		return c.Status(gofiber.StatusUnprocessableEntity).JSON(gofiber.Map{
			"message": "Private key does not match certificate",
			"errors": gofiber.Map{
				"private_key": "does not match the certificate",
			},
		})
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiberutil.NotFound("Stored certificate not found")
	}
	return err
}
