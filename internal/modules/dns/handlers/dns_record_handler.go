package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// DNSRecordHandler handles HTTP requests for DNS records
type DNSRecordHandler struct {
	recordService *services.DNSRecordService
	domainService *services.DomainService
}

// NewDNSRecordHandler creates a new DNSRecordHandler instance
func NewDNSRecordHandler(recordService *services.DNSRecordService, domainService *services.DomainService) *DNSRecordHandler {
	return &DNSRecordHandler{
		recordService: recordService,
		domainService: domainService,
	}
}

// ListRecords lists all DNS records for a domain
func (h *DNSRecordHandler) ListRecords(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("id")

	records, err := h.domainService.GetDomainRecords(c.Context(), domainID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		return response.HandleError(c, err)
	}

	return response.OK(c, "Records retrieved", records)
}

// CreateRecord creates a new DNS record
func (h *DNSRecordHandler) CreateRecord(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("id")

	req, err := fiberutil.MustParseAndValidate[dto.CreateDNSRecordRequest](c)
	if err != nil {
		return err
	}

	record, err := h.recordService.CreateRecord(c.Context(), domainID, teamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		return response.HandleError(c, err)
	}

	return response.Created(c, "Record created", dto.ToDNSRecordResponse(record))
}

// UpdateRecord updates a DNS record
func (h *DNSRecordHandler) UpdateRecord(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	req, err := fiberutil.MustParseAndValidate[dto.UpdateDNSRecordRequest](c)
	if err != nil {
		return err
	}

	record, err := h.recordService.UpdateRecord(c.Context(), recordID, domainID, teamID, req)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgDNSRecordNotFound)
		}
		if errors.Is(err, services.ErrRecordNotEditable) {
			return response.Forbidden(c, "This record type cannot be edited")
		}
		return response.HandleError(c, err)
	}

	return response.OK(c, "Record updated", dto.ToDNSRecordResponse(record))
}

// DeleteRecord deletes a DNS record
func (h *DNSRecordHandler) DeleteRecord(c *fiber.Ctx) error {
	teamID, err := fiberutil.MustGetTeamID(c)
	if err != nil {
		return err
	}
	domainID := c.Params("domainId")
	recordID := c.Params("recordId")

	err = h.recordService.DeleteRecord(c.Context(), recordID, domainID, teamID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return response.NotFound(c, response.MsgDomainNotFound)
		}
		if fiberutil.IsNotFound(err) {
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
func (h *DNSRecordHandler) GetRecordTypes(c *fiber.Ctx) error {
	types := h.recordService.GetRecordTypes()

	return response.OK(c, "Record types retrieved", types)
}
