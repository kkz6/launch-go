package tasks

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
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
	if h.logger != nil {
		h.logger.Debug().
			Str("task_id", taskID).
			Str("server_id", h.serverID).
			Str("marker_type", marker.Type).
			Str("marker_value", marker.Value).
			Msg("ProvisionMarkerHandler: received marker")
	}

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

	case markers.DetectedOS:
		return h.handleDetectedOS(ctx, marker)
	}

	return nil
}

// handleDetectedOS persists the OS facts the detect_os provision step
// captured from /etc/os-release + uname. Value format:
//
//	os_id|os_version|os_version_codename|arch|kernel
//
// Any individual field may be empty — that's how the script signals
// "couldn't determine this one" without us treating it as an error.
// We zero out empty fields (nil pointer) rather than writing empty
// strings so downstream code that checks `IfNotNil` still works.
func (h *ProvisionMarkerHandler) handleDetectedOS(ctx context.Context, marker *markers.Marker) error {
	parts := strings.SplitN(marker.Value, "|", 5)
	for len(parts) < 5 {
		parts = append(parts, "")
	}

	updates := map[string]any{
		"detected_os_id":               stringOrNil(parts[0]),
		"detected_os_version":          stringOrNil(parts[1]),
		"detected_os_version_codename": stringOrNil(parts[2]),
		"detected_arch":                stringOrNil(parts[3]),
		"detected_kernel":              stringOrNil(parts[4]),
		"detected_at":                  time.Now(),
	}

	if h.db != nil {
		if err := h.db.WithContext(ctx).
			Model(&models.Server{}).
			Where("id = ?", h.serverID).
			Updates(updates).Error; err != nil {
			if h.logger != nil {
				h.logger.Warn().Err(err).
					Str("server_id", h.serverID).
					Msg("Failed to persist detected OS facts")
			}
			return err
		}
	}

	// Two broadcasts: a dedicated event for any UI that wants the
	// detected payload directly (saves a refetch), and the generic
	// server.updated so the existing model-event listeners refresh.
	h.broadcast("server.detected_os", map[string]any{
		"server_id":                    h.serverID,
		"detected_os_id":               parts[0],
		"detected_os_version":          parts[1],
		"detected_os_version_codename": parts[2],
		"detected_arch":                parts[3],
		"detected_kernel":              parts[4],
	})
	h.broadcast("server.updated", map[string]any{
		"id":        h.serverID,
		"model":     "server",
		"action":    "updated",
		"server_id": h.serverID,
		"team_id":   h.teamID,
	})

	if h.logger != nil {
		h.logger.Info().
			Str("server_id", h.serverID).
			Str("os_id", parts[0]).
			Str("os_version", parts[1]).
			Str("os_codename", parts[2]).
			Str("arch", parts[3]).
			Str("kernel", parts[4]).
			Msg("Detected OS facts recorded")
	}
	return nil
}

// stringOrNil returns nil for empty strings so DB columns end up as
// NULL rather than empty strings. Matches the *string semantics on
// the Server model's Detected* fields.
func stringOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
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

	// Also broadcast server.updated for the model events
	h.broadcast("server.updated", map[string]interface{}{
		"id":        h.serverID,
		"model":     "server",
		"action":    "updated",
		"server_id": h.serverID,
		"team_id":   h.teamID,
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

	// Update server's completed steps in database
	if h.db != nil {
		// Get current server to append to completed_provision_steps
		var server models.Server
		if err := h.db.Select("id", "completed_provision_steps").
			Where("id = ?", h.serverID).
			First(&server).Error; err != nil {
			if h.logger != nil {
				h.logger.Warn().Err(err).
					Str("step", step).
					Msg("Failed to get server for step completion")
			}
		} else {
			// Append step if not already in the list
			alreadyExists := false
			for _, s := range server.CompletedProvisionSteps {
				if s == step {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				server.CompletedProvisionSteps = append(server.CompletedProvisionSteps, step)
				if err := h.db.Model(&models.Server{}).
					Where("id = ?", h.serverID).
					Updates(map[string]interface{}{
						"progress_step":             step,
						"completed_provision_steps": server.CompletedProvisionSteps,
					}).Error; err != nil {
					if h.logger != nil {
						h.logger.Warn().Err(err).
							Str("step", step).
							Msg("Failed to update server completed steps")
					}
				}
			}
		}
	}

	// Broadcast step completion
	h.broadcast("server.provision_step", map[string]interface{}{
		"server_id": h.serverID,
		"step":      step,
	})

	// Also broadcast server.updated for the model events
	h.broadcast("server.updated", map[string]interface{}{
		"id":        h.serverID,
		"model":     "server",
		"action":    "updated",
		"server_id": h.serverID,
		"team_id":   h.teamID,
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
	// Use "installed" status to match Laravel behavior during provisioning
	if h.db != nil {
		// Update installed_services table
		if err := h.db.Model(&models.InstalledService{}).
			Where("server_id = ? AND software = ?", h.serverID, software).
			Update("status", types.ServiceStatusInstalled).Error; err != nil {
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

	// Also broadcast server.updated for the model events
	h.broadcast("server.updated", map[string]interface{}{
		"id":        h.serverID,
		"model":     "server",
		"action":    "updated",
		"server_id": h.serverID,
		"team_id":   h.teamID,
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

	// Broadcast status update (status_label is not persisted, only broadcast for real-time UI)
	h.broadcast("server.provision_status", map[string]interface{}{
		"server_id": h.serverID,
		"message":   message,
	})

	if h.logger != nil {
		h.logger.Debug().
			Str("server_id", h.serverID).
			Str("message", message).
			Msg("Provision status broadcast")
	}

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
		if h.logger != nil {
			h.logger.Warn().
				Str("event", event).
				Str("server_id", h.serverID).
				Str("team_id", h.teamID).
				Msg("ProvisionMarkerHandler: broadcaster is nil, cannot broadcast event")
		}
		return
	}

	if h.logger != nil {
		h.logger.Debug().
			Str("event", event).
			Str("server_id", h.serverID).
			Str("team_id", h.teamID).
			Interface("data", data).
			Msg("ProvisionMarkerHandler: broadcasting event to team")
	}

	h.broadcaster.BroadcastToTeam(h.teamID, event, data)
}
