package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

var createServerTypeLabels = map[types.ServerType]string{
	types.ServerTypePhp:          "PHP Application Server",
	types.ServerTypeDatabase:     "Database Server",
	types.ServerTypeLoadBalancer: "Load Balancer",
	types.ServerTypeDocker:       "Docker Application Server",
}

// localizeCreateServerOptions returns a request-local copy so translating the
// static option labels can never mutate a map shared with another request.
func localizeCreateServerOptions(ctx context.Context, options dto.CreateServerOptionsResponse) dto.CreateServerOptionsResponse {
	localized := options
	localized.ServerTypes = make(map[string]string, len(options.ServerTypes))
	for value, fallback := range options.ServerTypes {
		label, stable := createServerTypeLabels[types.ServerType(value)]
		if !stable {
			localized.ServerTypes[value] = fallback
			continue
		}
		localized.ServerTypes[value] = i18n.TContext(ctx, label)
	}
	return localized
}

// GetCreateOptions returns the static options menu for creating a
// server. Not team-scoped, so it does not fit Index.
func (h *Handler) GetCreateOptions(c *fiber.Ctx) error {
	options := localizeCreateServerOptions(c.Context(), dto.GetCreateServerOptions())
	return fiberctx.OK(c, "Create options retrieved", options)
}
