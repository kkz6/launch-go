package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
)

// NotificationChannelService defines the interface for notification channel business logic
type NotificationChannelService interface {
	// CreateChannel creates a new notification channel
	CreateChannel(ctx context.Context, userID, teamID string, req *dto.CreateChannelRequest) (*models.NotificationChannel, error)

	// UpdateChannel updates an existing notification channel
	UpdateChannel(ctx context.Context, id, userID, teamID string, req *dto.UpdateChannelRequest) (*models.NotificationChannel, error)

	// DeleteChannel deletes a notification channel
	DeleteChannel(ctx context.Context, id, teamID string) error

	// GetChannel retrieves a notification channel by ID
	GetChannel(ctx context.Context, id, teamID string) (*models.NotificationChannel, error)

	// ListChannels lists all notification channels for a team
	ListChannels(ctx context.Context, teamID string) ([]models.NotificationChannel, error)

	// TestChannel tests a notification channel by sending a test message
	TestChannel(ctx context.Context, id, teamID string, message string) error

	// SendToTeam sends a notification to all connected channels for a team
	SendToTeam(ctx context.Context, teamID string, notif models.Notification) error

	// SendToChannel sends a notification to a specific channel
	SendToChannel(ctx context.Context, channelID string, notif models.Notification) error

	// SetChannelDefault sets a channel as the default for its provider type
	SetChannelDefault(ctx context.Context, id, teamID string) error

	// DisconnectChannel marks a channel as disconnected
	DisconnectChannel(ctx context.Context, id, teamID string) error

	// ReconnectChannel attempts to reconnect a channel
	ReconnectChannel(ctx context.Context, id, teamID string) error
}
