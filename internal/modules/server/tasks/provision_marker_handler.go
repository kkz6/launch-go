package tasks

import (
	"context"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

// ProvisionMarkerHandler handles markers during server provisioning.
// It updates the server's progress and status in the database and broadcasts
// events to connected clients via WebSocket.
type ProvisionMarkerHandler struct {
	db          *gorm.DB
	broadcaster broadcast.TeamBroadcaster
	logger      *zerolog.Logger
	serverID    string
	teamID      string

	// Track the last progress to avoid duplicate updates
	lastProgress int
}

// ProvisionMarkerHandlerConfig holds configuration for the marker handler
type ProvisionMarkerHandlerConfig struct {
	DB          *gorm.DB
	Broadcaster broadcast.TeamBroadcaster
	Logger      *zerolog.Logger
	ServerID    string
	TeamID      string
}

// NewProvisionMarkerHandler creates a new marker handler for provisioning
func NewProvisionMarkerHandler(cfg ProvisionMarkerHandlerConfig) *ProvisionMarkerHandler {
	return &ProvisionMarkerHandler{
		db:          cfg.DB,
		broadcaster: cfg.Broadcaster,
		logger:      cfg.Logger,
		serverID:    cfg.ServerID,
		teamID:      cfg.TeamID,
	}
}

// OnMarker handles a marker received during provisioning.
// Implements taskrunner.MarkerHandler interface.
func (h *ProvisionMarkerHandler) OnMarker(ctx context.Context, taskID string, marker *markers.Marker) error {
	switch marker.Type {
	case markers.Progress:
		return h.handleProgress(ctx, marker)

	case markers.StepCompleted:
		return h.handleStepCompleted(ctx, marker)

	case markers.SoftwareInstalled:
		return h.handleSoftwareInstalled(ctx, marker)

	case markers.Status:
		return h.handleStatus(ctx, marker)

	case markers.Error:
		return h.handleError(ctx, marker)
	}

	return nil
}

func (h *ProvisionMarkerHandler) handleProgress(ctx context.Context, marker *markers.Marker) error {
	progress := marker.ProgressValue()

	// Skip if progress hasn't changed
	if progress == h.lastProgress {
		return nil
	}
	h.lastProgress = progress

	// Update server progress in database
	if h.db != nil {
		if err := h.db.Model(&models.Server{}).
			Where("id = ?", h.serverID).
			Update("progress", progress).Error; err != nil {
			if h.logger != nil {
				h.logger.Warn().Err(err).
					Int("progress", progress).
					Msg("Failed to update server progress")
			}
		}
	}

	// Broadcast progress update
	h.broadcast("server.provision_progress", map[string]interface{}{
		"server_id": h.serverID,
		"progress":  progress,
	})

	if h.logger != nil {
		h.logger.Debug().
			Str("server_id", h.serverID).
			Int("progress", progress).
			Msg("Provision progress updated")
	}

	return nil
}

func (h *ProvisionMarkerHandler) handleStepCompleted(ctx context.Context, marker *markers.Marker) error {
	step := marker.Value

	// Update server's current step in database
	if h.db != nil {
		if err := h.db.Model(&models.Server{}).
			Where("id = ?", h.serverID).
			Update("progress_step", step).Error; err != nil {
			if h.logger != nil {
				h.logger.Warn().Err(err).
					Str("step", step).
					Msg("Failed to update server progress step")
			}
		}
	}

	// Broadcast step completion
	h.broadcast("server.provision_step", map[string]interface{}{
		"server_id": h.serverID,
		"step":      step,
	})

	if h.logger != nil {
		h.logger.Debug().
			Str("server_id", h.serverID).
			Str("step", step).
			Msg("Provision step completed")
	}

	return nil
}

func (h *ProvisionMarkerHandler) handleSoftwareInstalled(ctx context.Context, marker *markers.Marker) error {
	software := marker.Value

	// Mark the software as installed in the database
	if h.db != nil {
		// Update installed_services table
		if err := h.db.Model(&models.InstalledService{}).
			Where("server_id = ? AND software = ?", h.serverID, software).
			Update("status", "running").Error; err != nil {
			if h.logger != nil {
				h.logger.Warn().Err(err).
					Str("software", software).
					Msg("Failed to update installed service status")
			}
		}
	}

	// Broadcast software installation
	h.broadcast("server.software_installed", map[string]interface{}{
		"server_id": h.serverID,
		"software":  software,
	})

	if h.logger != nil {
		h.logger.Debug().
			Str("server_id", h.serverID).
			Str("software", software).
			Msg("Software installed")
	}

	return nil
}

func (h *ProvisionMarkerHandler) handleStatus(ctx context.Context, marker *markers.Marker) error {
	message := marker.Value

	// Update server status label in database
	if h.db != nil {
		if err := h.db.Model(&models.Server{}).
			Where("id = ?", h.serverID).
			Update("status_label", message).Error; err != nil {
			if h.logger != nil {
				h.logger.Warn().Err(err).
					Str("message", message).
					Msg("Failed to update server status label")
			}
		}
	}

	// Broadcast status update
	h.broadcast("server.provision_status", map[string]interface{}{
		"server_id": h.serverID,
		"message":   message,
	})

	return nil
}

func (h *ProvisionMarkerHandler) handleError(ctx context.Context, marker *markers.Marker) error {
	message := marker.Value

	// Log the error
	if h.logger != nil {
		h.logger.Warn().
			Str("server_id", h.serverID).
			Str("error", message).
			Msg("Provision error (non-fatal)")
	}

	// Broadcast error (non-fatal)
	h.broadcast("server.provision_error", map[string]interface{}{
		"server_id": h.serverID,
		"message":   message,
		"fatal":     false,
	})

	return nil
}

func (h *ProvisionMarkerHandler) broadcast(event string, data map[string]interface{}) {
	if h.broadcaster == nil {
		return
	}

	h.broadcaster.BroadcastToTeam(h.teamID, event, data)
}
