package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

var (
	ErrInvalidProvider    = errors.New("invalid provider")
	ErrConnectionFailed   = errors.New("failed to connect to channel")
	ErrNotificationFailed = errors.New("failed to send notification")
	ErrUnauthorized       = errors.New("unauthorized access to channel")
)

// NotificationChannelService handles notification channel business logic
type NotificationChannelService struct {
	*BaseService
}

// NewNotificationChannelService creates a new notification channel service
func NewNotificationChannelService(deps *ServiceDeps) *NotificationChannelService {
	return &NotificationChannelService{
		BaseService: NewBaseService(deps),
	}
}

// CreateChannel creates a new notification channel
func (s *NotificationChannelService) CreateChannel(ctx context.Context, userID, teamID string, req *dto.CreateChannelRequest) (*models.NotificationChannel, error) {
	// Parse and validate provider
	provider, err := notificationtypes.ParseChannelType(req.Provider)
	if err != nil {
		return nil, ErrInvalidProvider
	}

	// Create the channel model
	channel := &models.NotificationChannel{
		UserID:    userID,
		TeamID:    teamID,
		Provider:  provider,
		Label:     req.Label,
		Data:      req.ToChannelData(),
		Connected: false,
		IsDefault: false,
	}

	// Get the channel driver
	driver, err := s.ChannelFactory().CreateChannel(channel.ToChannelsNotificationChannel())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProvider, err)
	}

	// Test connection
	if err := driver.Connect(ctx); err != nil {
		s.Logger.Warn().Err(err).Str("provider", req.Provider).Msg("failed to connect to notification channel")
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// Mark as connected
	channel.Connected = true

	// Save to database
	if err := s.Repos().NotificationChannel().Create(ctx, channel); err != nil {
		return nil, err
	}

	return channel, nil
}

// UpdateChannel updates an existing notification channel
func (s *NotificationChannelService) UpdateChannel(ctx context.Context, id, userID, teamID string, req *dto.UpdateChannelRequest) (*models.NotificationChannel, error) {
	// Find the existing channel
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return nil, err
	}

	// Update the channel data
	channel.Label = req.Label

	// Preserve existing data and update only provided fields
	newData := channel.Data
	if req.Email != "" {
		newData.Email = req.Email
	}

	if req.WebhookURL != "" {
		newData.WebhookURL = req.WebhookURL
	}

	if req.BotToken != "" {
		newData.BotToken = req.BotToken
	}

	if req.ChatID != "" {
		newData.ChatID = req.ChatID
	}

	newData.AppDeploy = req.AppDeploy
	newData.DatabaseBackup = req.DatabaseBackup
	channel.Data = newData

	// Get the channel driver
	driver, err := s.ChannelFactory().CreateChannel(channel.ToChannelsNotificationChannel())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProvider, err)
	}

	// Test connection
	if err := driver.Connect(ctx); err != nil {
		s.Logger.Warn().Err(err).Str("id", id).Msg("failed to reconnect notification channel")
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	channel.Connected = true

	// Save updates
	if err := s.Repos().NotificationChannel().Update(ctx, channel); err != nil {
		return nil, err
	}

	return channel, nil
}

// DeleteChannel deletes a notification channel
func (s *NotificationChannelService) DeleteChannel(ctx context.Context, id, teamID string) error {
	// Verify the channel belongs to the team
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return err
	}

	if channel.TeamID != teamID {
		return ErrUnauthorized
	}

	return s.Repos().NotificationChannel().Delete(ctx, id)
}

// GetChannel retrieves a notification channel by ID
func (s *NotificationChannelService) GetChannel(ctx context.Context, id, teamID string) (*models.NotificationChannel, error) {
	return s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
}

// ListChannels lists all notification channels for a team
func (s *NotificationChannelService) ListChannels(ctx context.Context, teamID string) ([]models.NotificationChannel, error) {
	return s.Repos().NotificationChannel().FindByTeamID(ctx, teamID)
}

// notificationAdapter adapts our Notification interface to the channels.Notification interface
type notificationAdapter struct {
	notification models.Notification
}

func (a *notificationAdapter) RawText() string {
	return a.notification.RawText()
}

func (a *notificationAdapter) ToEmail() *channels.EmailMessage {
	return a.notification.ToEmail()
}

func (a *notificationAdapter) ToSlack() string {
	return a.notification.ToSlack()
}

func (a *notificationAdapter) ToDiscord() string {
	return a.notification.ToDiscord()
}

func (a *notificationAdapter) ToTelegram() string {
	return a.notification.ToTelegram()
}

// TestChannel tests a notification channel by sending a test message
func (s *NotificationChannelService) TestChannel(ctx context.Context, id, teamID string, message string) error {
	// Find the channel
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return err
	}

	// Get the channel driver
	driver, err := s.ChannelFactory().CreateChannel(channel.ToChannelsNotificationChannel())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProvider, err)
	}

	// Create a test notification
	testNotif := models.NewBaseNotification(
		notificationtypes.NotificationType("test"),
		message,
	)

	// Send the test notification
	if err := driver.Send(ctx, &notificationAdapter{notification: testNotif}); err != nil {
		s.Logger.Warn().Err(err).Str("id", id).Msg("failed to send test notification")
		return fmt.Errorf("%w: %v", ErrNotificationFailed, err)
	}

	return nil
}

// SendToTeam sends a notification to all connected channels for a team
func (s *NotificationChannelService) SendToTeam(ctx context.Context, teamID string, notif models.Notification) error {
	// Get all connected channels for the team
	chans, err := s.Repos().NotificationChannel().FindConnected(ctx, teamID)
	if err != nil {
		return err
	}

	var sendErrors []error

	for _, ch := range chans {
		driver, err := s.ChannelFactory().CreateChannel(ch.ToChannelsNotificationChannel())
		if err != nil {
			s.Logger.Warn().Err(err).Uint64("channel_id", ch.ID).Msg("failed to create channel driver")
			sendErrors = append(sendErrors, err)
			continue
		}

		if err := driver.Send(ctx, &notificationAdapter{notification: notif}); err != nil {
			s.Logger.Warn().Err(err).Uint64("channel_id", ch.ID).Msg("failed to send notification")
			sendErrors = append(sendErrors, err)
			continue
		}

		s.Logger.Info().
			Uint64("channel_id", ch.ID).
			Str("provider", ch.Provider.String()).
			Str("notification_type", notif.Type().String()).
			Msg("notification sent successfully")
	}

	if len(sendErrors) > 0 && len(sendErrors) == len(chans) {
		return fmt.Errorf("%w: all channels failed", ErrNotificationFailed)
	}

	return nil
}

// SendToChannel sends a notification to a specific channel
func (s *NotificationChannelService) SendToChannel(ctx context.Context, channelID string, notif models.Notification) error {
	channel, err := s.Repos().NotificationChannel().FindByID(ctx, channelID)
	if err != nil {
		return err
	}

	if !channel.Connected {
		return channels.ErrChannelDisabled
	}

	driver, err := s.ChannelFactory().CreateChannel(channel.ToChannelsNotificationChannel())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProvider, err)
	}

	if err := driver.Send(ctx, &notificationAdapter{notification: notif}); err != nil {
		s.Logger.Warn().Err(err).Str("channel_id", channelID).Msg("failed to send notification")
		return fmt.Errorf("%w: %v", ErrNotificationFailed, err)
	}

	s.Logger.Info().
		Str("channel_id", channelID).
		Str("provider", channel.Provider.String()).
		Str("notification_type", notif.Type().String()).
		Msg("notification sent successfully")

	return nil
}

// SetChannelDefault sets a channel as the default for its provider type
func (s *NotificationChannelService) SetChannelDefault(ctx context.Context, id, teamID string) error {
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return err
	}

	return s.Repos().NotificationChannel().SetDefault(ctx, id, teamID, channel.Provider)
}

// DisconnectChannel marks a channel as disconnected
func (s *NotificationChannelService) DisconnectChannel(ctx context.Context, id, teamID string) error {
	// Verify the channel belongs to the team
	_, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return err
	}

	return s.Repos().NotificationChannel().SetConnected(ctx, id, false)
}

// ReconnectChannel attempts to reconnect a channel
func (s *NotificationChannelService) ReconnectChannel(ctx context.Context, id, teamID string) error {
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return err
	}

	driver, err := s.ChannelFactory().CreateChannel(channel.ToChannelsNotificationChannel())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProvider, err)
	}

	if err := driver.Connect(ctx); err != nil {
		s.Logger.Warn().Err(err).Str("id", id).Msg("failed to reconnect notification channel")
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	return s.Repos().NotificationChannel().SetConnected(ctx, id, true)
}
