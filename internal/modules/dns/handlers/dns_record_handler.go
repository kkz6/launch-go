package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
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
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("id")

	records, err := h.domainService.GetDomainRecords(c.Context(), domainID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		return response.HandleError(c, err)
	}

	return response.OK(c, "Records retrieved", records)
}

// CreateRecord creates a new DNS record
func (h *DnsRecordHandler) CreateRecord(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[dto.CreateDnsRecordRequest](c)
	if err != nil {
		return err
	}

	record, err := h.recordService.CreateRecord(c.Context(), domainID, teamID, req)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		return response.HandleError(c, err)
	}

	return response.Created(c, "Record created", dto.ToDnsRecordResponse(record))
}

// UpdateRecord updates a DNS record
func (h *DnsRecordHandler) UpdateRecord(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	req, err := fiberctx.MustParseAndValidate[dto.UpdateDnsRecordRequest](c)
	if err != nil {
		return err
	}

	record, err := h.recordService.UpdateRecord(c.Context(), recordID, domainID, teamID, req)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		if errors.Is(err, services.ErrRecordNotFound) {
			return response.NotFound(c, response.MsgDNSRecordNotFound)
		}
		if errors.Is(err, services.ErrRecordNotEditable) {
			return response.Forbidden(c, "This record type cannot be edited")
		}
		return response.HandleError(c, err)
	}

	return response.OK(c, "Record updated", dto.ToDnsRecordResponse(record))
}

// DeleteRecord deletes a DNS record
func (h *DnsRecordHandler) DeleteRecord(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	err = h.recordService.DeleteRecord(c.Context(), recordID, domainID, teamID)
	if err != nil {
		if errors.Is(err, services.ErrDomainNotFound) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		if errors.Is(err, services.ErrRecordNotFound) {
			return response.NotFound(c, response.MsgDNSRecordNotFound)
		}
		if errors.Is(err, services.ErrRecordNotDeletable) {
			return response.Forbidden(c, "This record type cannot be deleted")
		}
		return response.HandleError(c, err)
	}

	return response.NoContent(c)
}

// GetRecordTypes returns all available record types
func (h *DnsRecordHandler) GetRecordTypes(c *fiber.Ctx) error {
	types := h.recordService.GetRecordTypes()

	return response.OK(c, "Record types retrieved", types)
}
