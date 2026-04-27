package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Service-level sentinel errors carry their final HTTP status and
// message so the global error handler renders them without per-handler
// branching. The wire response matches the previous handler-mapped
// behavior exactly.
var (
	ErrInvalidProvider    = fiberutil.BadRequest("Invalid notification provider")
	ErrConnectionFailed   = fiberutil.BadRequest("Could not connect to the notification channel. Please verify your configuration.")
	ErrNotificationFailed = fiberutil.BadRequest("Failed to send test notification. Please verify your channel configuration.")
	ErrUnauthorized       = fiberutil.Forbidden(fiberutil.MsgForbidden)
)

// GetPreferences returns notification preferences for a team, creating
// defaults if needed. Signature matches IndexFunc.
func (s *NotificationChannelService) GetPreferences(ctx context.Context, teamID string) (dto.NotificationPreferencesResponse, error) {
	pref, err := s.Repos().NotificationPreference().FindOrCreateByTeamID(ctx, teamID)
	if err != nil {
		return dto.NotificationPreferencesResponse{}, err
	}
	return dto.ToNotificationPreferencesResponse(pref), nil
}

// UpdatePreferences updates notification preferences for a team and
// returns the response DTO. PUT to a team-singleton resource so it does
// not fit the generic Update helper.
func (s *NotificationChannelService) UpdatePreferences(ctx context.Context, teamID string, req *dto.UpdateNotificationPreferencesRequest) (dto.NotificationPreferencesResponse, error) {
	pref, err := s.Repos().NotificationPreference().FindOrCreateByTeamID(ctx, teamID)
	if err != nil {
		return dto.NotificationPreferencesResponse{}, err
	}

	pref.EmailServerCreated = req.EmailServerCreated
	pref.EmailServerDeleted = req.EmailServerDeleted
	pref.EmailDeploymentSuccess = req.EmailDeploymentSuccess
	pref.EmailDeploymentFailed = req.EmailDeploymentFailed
	pref.EmailBackupSuccess = req.EmailBackupSuccess
	pref.EmailBackupFailed = req.EmailBackupFailed

	if err := s.Repos().NotificationPreference().UpdateByTeamID(ctx, teamID, pref); err != nil {
		return dto.NotificationPreferencesResponse{}, err
	}
	return dto.ToNotificationPreferencesResponse(pref), nil
}

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

// CreateChannel creates a new notification channel. Signature matches
// CreateFunc and returns the response DTO.
func (s *NotificationChannelService) CreateChannel(ctx context.Context, teamID, userID string, req *dto.CreateChannelRequest) (dto.ChannelResponse, error) {
	channel, err := s.buildAndConnectChannel(ctx, teamID, userID, req)
	if err != nil {
		return dto.ChannelResponse{}, err
	}
	return dto.ToChannelResponse(channel), nil
}

func (s *NotificationChannelService) buildAndConnectChannel(ctx context.Context, teamID, userID string, req *dto.CreateChannelRequest) (*models.NotificationChannel, error) {
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

// UpdateChannel updates an existing notification channel. Signature
// matches UpdateFunc and returns the response DTO.
func (s *NotificationChannelService) UpdateChannel(ctx context.Context, id, teamID, userID string, req *dto.UpdateChannelRequest) (dto.ChannelResponse, error) {
	_ = userID
	channel, err := s.applyChannelUpdate(ctx, id, teamID, req)
	if err != nil {
		return dto.ChannelResponse{}, err
	}
	return dto.ToChannelResponse(channel), nil
}

func (s *NotificationChannelService) applyChannelUpdate(ctx context.Context, id, teamID string, req *dto.UpdateChannelRequest) (*models.NotificationChannel, error) {
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return nil, fiberutil.NotFoundAs(err, "Notification channel not found")
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

// DeleteChannel deletes a notification channel. Signature matches DeleteFunc.
func (s *NotificationChannelService) DeleteChannel(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return fiberutil.NotFoundAs(err, "Notification channel not found")
	}
	if channel.TeamID != teamID {
		return ErrUnauthorized
	}
	return s.Repos().NotificationChannel().Delete(ctx, id)
}

// GetChannel retrieves a notification channel by ID and returns the
// response DTO. Signature matches ShowFunc.
func (s *NotificationChannelService) GetChannel(ctx context.Context, id, teamID string) (dto.ChannelResponse, error) {
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return dto.ChannelResponse{}, fiberutil.NotFoundAs(err, "Notification channel not found")
	}
	return dto.ToChannelResponse(channel), nil
}

// ListChannels lists all notification channels for a team. Signature
// matches IndexFunc and wraps the result in ListChannelsResponse.
func (s *NotificationChannelService) ListChannels(ctx context.Context, teamID string) (dto.ListChannelsResponse, error) {
	chans, err := s.Repos().NotificationChannel().FindByTeamID(ctx, teamID)
	if err != nil {
		return dto.ListChannelsResponse{}, err
	}
	return dto.ListChannelsResponse{Channels: dto.ToChannelResponses(chans)}, nil
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

// TestChannel tests a notification channel by sending a test message.
// Has its own signature (extra `message` param) so it does not fit the
// generic Action helper; the route handler stays bespoke.
func (s *NotificationChannelService) TestChannel(ctx context.Context, id, teamID, message string) error {
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return fiberutil.NotFoundAs(err, "Notification channel not found")
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
	// Check team preferences to see if this notification type should be sent
	pref, err := s.Repos().NotificationPreference().FindOrCreateByTeamID(ctx, teamID)
	if err != nil {
		s.Logger.Warn().Err(err).Str("team_id", teamID).Msg("failed to load notification preferences, sending anyway")
	} else if !pref.ShouldSend(notif.Type()) {
		s.Logger.Info().
			Str("team_id", teamID).
			Str("notification_type", notif.Type().String()).
			Msg("notification suppressed by team preferences")
		return nil
	}

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

// SetChannelDefault sets a channel as the default for its provider type.
// Signature matches ActionFunc.
func (s *NotificationChannelService) SetChannelDefault(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return fiberutil.NotFoundAs(err, "Notification channel not found")
	}
	return s.Repos().NotificationChannel().SetDefault(ctx, id, teamID, channel.Provider)
}

// DisconnectChannel marks a channel as disconnected. Signature matches ActionFunc.
func (s *NotificationChannelService) DisconnectChannel(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	if _, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID); err != nil {
		return fiberutil.NotFoundAs(err, "Notification channel not found")
	}
	return s.Repos().NotificationChannel().SetConnected(ctx, id, false)
}

// ReconnectChannel attempts to reconnect a channel. Signature matches ActionFunc.
func (s *NotificationChannelService) ReconnectChannel(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	channel, err := s.Repos().NotificationChannel().FindByIDAndTeamID(ctx, id, teamID)
	if err != nil {
		return fiberutil.NotFoundAs(err, "Notification channel not found")
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
