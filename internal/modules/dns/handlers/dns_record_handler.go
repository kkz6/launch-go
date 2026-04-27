package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/dns/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// DNSRecordHandler holds the static record-types endpoint, which is not
// team-scoped and therefore does not fit any team-scoped route helper.
type DNSRecordHandler struct {
	recordService *services.DNSRecordService
}

// NewDNSRecordHandler creates a new DNSRecordHandler.
func NewDNSRecordHandler(recordService *services.DNSRecordService) *DNSRecordHandler {
	return &DNSRecordHandler{recordService: recordService}
}

// GetRecordTypes returns all available DNS record types.
func (h *DNSRecordHandler) GetRecordTypes(c *fiber.Ctx) error {
	return fiberutil.OK(c, "Record types retrieved", h.recordService.GetRecordTypes())
}
