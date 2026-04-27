package contracts

import (
	"context"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServerReader exposes the read-side of the server module that the
// docker-service module needs (lookup with team-scope enforcement).
type ServerReader interface {
	FindByIDAndTeam(ctx context.Context, id, teamID string, preloads ...string) (*servermodels.Server, error)
}
