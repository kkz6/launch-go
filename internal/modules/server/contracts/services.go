package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CronService defines the cross-module surface for cron operations.
// HTTP routes in the server module use the helper-shaped methods on the
// concrete service directly; this contract is only the subset other
// modules need.
type CronService interface {
	// CreateCronRaw creates a cron job and returns the model. Used by
	// the site module during site provisioning.
	CreateCronRaw(ctx context.Context, serverID, teamID string, req *dto.CreateCronRequest) (*models.Cron, error)
	// CountCronsBySite counts cron jobs associated with a site.
	CountCronsBySite(ctx context.Context, siteID string) (int64, error)
}
