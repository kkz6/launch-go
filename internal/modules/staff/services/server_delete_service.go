package services

import (
	"context"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ServerDeleter preserves the server module's resource checks and provider jobs.
type ServerDeleter interface {
	DeleteServer(context.Context, string, string, string) error
	RemoveServerRecord(context.Context, string, string, string) error
	UninstallServer(context.Context, string, string, string) (bool, error)
}

func (s *Service) SetServerDeleter(deleter ServerDeleter) { s.serverDeleter = deleter }

type ServerDeletionResult struct {
	ServerID string `json:"server_id"`
	Status   string `json:"status"`
}

// DeleteServer resolves the owning team from the record, never the staff member's
// active team or a client-supplied team id. The caller must pass the staff gate.
func (s *Service) DeleteServer(ctx context.Context, serverID, actorID, confirmation, mode string) (ServerDeletionResult, error) {
	var result ServerDeletionResult
	if mode != "" && mode != "remove_record" && mode != "uninstall" {
		return result, fiberutil.Validation("Invalid server deletion mode")
	}
	server, err := s.repos.ServerByID(ctx, serverID)
	if err != nil {
		return result, err
	}
	if server == nil {
		return result, ErrServerNotFound
	}
	if confirmation == "" || confirmation != server.Name {
		return result, fiberutil.Validation("Type the server name exactly to confirm deletion")
	}
	if actorID == "" {
		return result, fiberutil.Unauthorized()
	}
	if s.serverDeleter == nil {
		return result, fiberutil.Internal("Server deletion is not configured")
	}
	if mode == "remove_record" {
		if err := s.serverDeleter.RemoveServerRecord(ctx, server.ID, server.TeamID, actorID); err != nil {
			return result, err
		}
		return ServerDeletionResult{ServerID: server.ID, Status: "removed"}, nil
	}
	if mode == "uninstall" {
		offline, err := s.serverDeleter.UninstallServer(ctx, server.ID, server.TeamID, actorID)
		if err != nil {
			return result, err
		}
		if offline {
			return ServerDeletionResult{ServerID: server.ID, Status: "removed_offline"}, nil
		}
	} else {
		if err := s.serverDeleter.DeleteServer(ctx, server.ID, server.TeamID, actorID); err != nil {
			return result, err
		}
	}
	result = ServerDeletionResult{ServerID: server.ID, Status: "deletion_requested"}
	if server.Provider == servertypes.ProviderCustom {
		result.Status = "deleted"
	}
	return result, nil
}
