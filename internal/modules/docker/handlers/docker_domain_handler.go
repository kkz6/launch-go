package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DockerDomainHandler handles HTTP requests for docker service domains
type DockerDomainHandler struct {
	domainRepo  *repositories.DockerDomainRepository
	serviceRepo *repositories.DockerServiceRepository
}

// NewDockerDomainHandler creates a new DockerDomainHandler
func NewDockerDomainHandler(domainRepo *repositories.DockerDomainRepository, serviceRepo *repositories.DockerServiceRepository) *DockerDomainHandler {
	return &DockerDomainHandler{
		domainRepo:  domainRepo,
		serviceRepo: serviceRepo,
	}
}

// List retrieves all domains for a docker service
func (h *DockerDomainHandler) List(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	domains, err := h.domainRepo.FindByServiceID(c.Context(), serviceID)
	if err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Domains retrieved", domains)
}

// Create creates a new domain for a docker service
func (h *DockerDomainHandler) Create(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	if _, err := h.serviceRepo.FindByID(c.Context(), serviceID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Docker service not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	var req struct {
		Host            string                `json:"host" validate:"required"`
		Path            string                `json:"path"`
		ContainerPort   int                   `json:"container_port" validate:"required,min=1,max=65535"`
		HTTPS           *bool                 `json:"https"`
		CertificateType types.CertificateType `json:"certificate_type"`
		ForceSSL        *bool                 `json:"force_ssl"`
	}

	if err := fiberutil.ParseAndValidate(c, &req); err != nil {
		return err
	}

	domain := &models.DockerDomain{
		DockerServiceID: serviceID,
		Host:            req.Host,
		Path:            "/",
		ContainerPort:   req.ContainerPort,
		HTTPS:           true,
		CertificateType: types.CertificateTypeLetsEncrypt,
		ForceSSL:        true,
	}

	if req.Path != "" {
		domain.Path = req.Path
	}

	if req.HTTPS != nil {
		domain.HTTPS = *req.HTTPS
	}

	if req.CertificateType != "" {
		domain.CertificateType = req.CertificateType
	}

	if req.ForceSSL != nil {
		domain.ForceSSL = *req.ForceSSL
	}

	if err := h.domainRepo.Create(c.Context(), domain); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.Created(c, "Domain created", domain)
}

// Update modifies an existing domain
func (h *DockerDomainHandler) Update(c *fiber.Ctx) error {
	domainID := c.Params("domainId")

	domain, err := h.domainRepo.FindByID(c.Context(), domainID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Domain not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := c.BodyParser(domain); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}

	if err := h.domainRepo.Update(c.Context(), domain); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Domain updated", domain)
}

// Delete removes a domain
func (h *DockerDomainHandler) Delete(c *fiber.Ctx) error {
	domainID := c.Params("domainId")

	if _, err := h.domainRepo.FindByID(c.Context(), domainID); err != nil {
		if fiberutil.IsNotFound(err) {
			return fiberutil.RespondNotFound(c, "Domain not found")
		}

		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	if err := h.domainRepo.Delete(c.Context(), domainID); err != nil {
		return fiberutil.RespondInternalError(c, fiberutil.MsgInternalError)
	}

	return fiberutil.OK(c, "Domain deleted", nil)
}
