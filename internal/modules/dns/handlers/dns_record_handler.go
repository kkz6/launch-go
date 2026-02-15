package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/dto"
	"github.com/kkz6/launch-go/internal/modules/dns/providers"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
			return fiberutil.RespondNotFound(c, "Domain not found")
		}
		return fiberutil.HandleError(c, err)
	}

	return fiberutil.OK(c, "Records retrieved", records)
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
			return fiberutil.RespondNotFound(c, "Domain not found")
		}
		return handleProviderOrDefaultError(c, err)
	}

	return fiberutil.Created(c, "Record created", dto.ToDNSRecordResponse(record))
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
			return fiberutil.RespondNotFound(c, "Record or domain not found")
		}
		if errors.Is(err, services.ErrRecordNotEditable) {
			return fiberutil.RespondForbidden(c, "This record type cannot be edited")
		}
		return handleProviderOrDefaultError(c, err)
	}

	return fiberutil.OK(c, "Record updated", dto.ToDNSRecordResponse(record))
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
			return fiberutil.RespondNotFound(c, "Record or domain not found")
		}
		if errors.Is(err, services.ErrRecordNotDeletable) {
			return fiberutil.RespondForbidden(c, "This record type cannot be deleted")
		}
		return handleProviderOrDefaultError(c, err)
	}

	return fiberutil.NoContent(c)
}

// GetRecordTypes returns all available record types
func (h *DNSRecordHandler) GetRecordTypes(c *fiber.Ctx) error {
	types := h.recordService.GetRecordTypes()

	return fiberutil.OK(c, "Record types retrieved", types)
}

// handleProviderOrDefaultError converts ProviderError to an HTTP response or falls through to HandleError.
func handleProviderOrDefaultError(c *fiber.Ctx, err error) error {
	var providerErr *providers.ProviderError
	if errors.As(err, &providerErr) {
		code := fiber.StatusBadGateway
		if providerErr.Code == 404 {
			code = fiber.StatusNotFound
		} else if providerErr.Code == 403 {
			code = fiber.StatusForbidden
		}
		return fiberutil.Error(c, code, providerErr.Message)
	}

	return fiberutil.HandleError(c, err)
}
