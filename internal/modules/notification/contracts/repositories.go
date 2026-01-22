package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// NotificationChannelRepository defines the interface for notification channel persistence
type NotificationChannelRepository interface {
	// Create creates a new notification channel
	Create(ctx context.Context, channel *models.NotificationChannel) error

	// Update updates an existing notification channel
	Update(ctx context.Context, channel *models.NotificationChannel) error

	// Delete deletes a notification channel by ID
	Delete(ctx context.Context, id string) error

	// FindByID finds a notification channel by ID
	FindByID(ctx context.Context, id string) (*models.NotificationChannel, error)

	// FindByIDAndTeamID finds a notification channel by ID and team ID
	FindByIDAndTeamID(ctx context.Context, id, teamID string) (*models.NotificationChannel, error)

	// FindByTeamID finds all notification channels for a team
	FindByTeamID(ctx context.Context, teamID string) ([]models.NotificationChannel, error)

	// FindByUserID finds all notification channels for a user
	FindByUserID(ctx context.Context, userID string) ([]models.NotificationChannel, error)

	// FindByProvider finds all notification channels by provider type
	FindByProvider(ctx context.Context, teamID string, provider notificationtypes.ChannelType) ([]models.NotificationChannel, error)

	// FindConnected finds all connected notification channels for a team
	FindConnected(ctx context.Context, teamID string) ([]models.NotificationChannel, error)

	// SetConnected sets the connected status of a notification channel
	SetConnected(ctx context.Context, id string, connected bool) error

	// SetDefault sets a notification channel as the default for its type
	SetDefault(ctx context.Context, id string, teamID string, provider notificationtypes.ChannelType) error

	// Exists checks if a notification channel exists
	Exists(ctx context.Context, id string) (bool, error)

	// CountByTeamID counts the notification channels for a team
	CountByTeamID(ctx context.Context, teamID string) (int64, error)

	// UpdateData updates only the data field of a notification channel
	UpdateData(ctx context.Context, id string, data models.ChannelData) error
}
