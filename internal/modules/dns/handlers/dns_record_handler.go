package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// DnsRecordHandler handles HTTP requests for DNS records
type DnsRecordHandler struct {
	recordService *services.DnsRecordService
	domainService *services.DomainService
}

// NewDnsRecordHandler creates a new DnsRecordHandler instance
func NewDnsRecordHandler(recordService *services.DnsRecordService, domainService *services.DomainService) *DnsRecordHandler {
	return &DnsRecordHandler{
		recordService: recordService,
		domainService: domainService,
	}
}

// ListRecords lists all DNS records for a domain
func (h *DnsRecordHandler) ListRecords(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("id")

	records, err := h.domainService.GetDomainRecords(c.Context(), domainID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Records retrieved", records)
}

// CreateRecord creates a new DNS record
func (h *DnsRecordHandler) CreateRecord(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("id")

	var req dto.CreateDnsRecordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	record, err := h.recordService.CreateRecord(c.Context(), domainID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Record created", dto.ToDnsRecordResponse(record))
}

// UpdateRecord updates a DNS record
func (h *DnsRecordHandler) UpdateRecord(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	var req dto.UpdateDnsRecordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errs := validator.Validate(&req); errs != nil {
		return response.ValidationError(c, errs)
	}

	record, err := h.recordService.UpdateRecord(c.Context(), recordID, domainID, teamID, &req)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		if errors.Is(err, services.ErrRecordNotFound) {
			return response.NotFound(c, "Record not found")
		}
		if errors.Is(err, services.ErrRecordNotEditable) {
			return response.Error(c, fiber.StatusForbidden, "This record type cannot be edited")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Record updated", dto.ToDnsRecordResponse(record))
}

// DeleteRecord deletes a DNS record
func (h *DnsRecordHandler) DeleteRecord(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	err := h.recordService.DeleteRecord(c.Context(), recordID, domainID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, "Domain not found")
		}
		if errors.Is(err, services.ErrRecordNotFound) {
			return response.NotFound(c, "Record not found")
		}
		if errors.Is(err, services.ErrRecordNotDeletable) {
			return response.Error(c, fiber.StatusForbidden, "This record type cannot be deleted")
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.NoContent(c)
}

// GetRecordTypes returns all available record types
func (h *DnsRecordHandler) GetRecordTypes(c *fiber.Ctx) error {
	types := h.recordService.GetRecordTypes()

	return response.OK(c, "Record types retrieved", types)
}
