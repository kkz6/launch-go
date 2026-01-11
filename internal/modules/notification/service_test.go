package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/dto"
	"github.com/kkz6/launch-go/internal/modules/notification/enums"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
)

func setupTestService(t *testing.T) (*services.NotificationChannelService, *repositories.NotificationChannelRepository) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&models.NotificationChannel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	repo := repositories.NewNotificationChannelRepository(db)

	mockHTTP := &channels.MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			return nil, 200, nil
		},
	}
	mockEmail := &channels.MockEmailSender{}
	factory := channels.NewFactoryWithEmail(mockHTTP, mockEmail)

	log := zerolog.Nop()
	service := services.NewNotificationChannelService(repo, factory, &log)

	return service, repo
}

func TestService_CreateChannel(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	tests := []struct {
		name      string
		req       *dto.CreateChannelRequest
		expectErr bool
	}{
		{
			name: "create email channel",
			req: &dto.CreateChannelRequest{
				Provider: "email",
				Label:    "My Email",
				Email:    "test@example.com",
			},
			expectErr: false,
		},
		{
			name: "create slack channel",
			req: &dto.CreateChannelRequest{
				Provider:   "slack",
				Label:      "My Slack",
				WebhookURL: "https://hooks.slack.com/xxx",
			},
			expectErr: false,
		},
		{
			name: "invalid provider",
			req: &dto.CreateChannelRequest{
				Provider: "invalid",
				Label:    "Test",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, err := service.CreateChannel(ctx, "user123", "team123", tt.req)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if channel.Label != tt.req.Label {
				t.Errorf("Label = %s, want %s", channel.Label, tt.req.Label)
			}
			if channel.UserID != "user123" {
				t.Errorf("UserID = %s, want 'user123'", channel.UserID)
			}
			if channel.TeamID != "team123" {
				t.Errorf("TeamID = %s, want 'team123'", channel.TeamID)
			}
			if !channel.Connected {
				t.Error("channel should be connected")
			}
		})
	}
}

func TestService_CreateChannel_ConnectionFailed(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = db.AutoMigrate(&models.NotificationChannel{})
	repo := repositories.NewNotificationChannelRepository(db)

	mockHTTP := &channels.MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			return nil, 500, errors.New("connection failed")
		},
	}
	factory := channels.NewFactory(mockHTTP)
	log := zerolog.Nop()
	service := services.NewNotificationChannelService(repo, factory, &log)

	req := &dto.CreateChannelRequest{
		Provider:   "slack",
		Label:      "My Slack",
		WebhookURL: "https://hooks.slack.com/xxx",
	}

	_, err := service.CreateChannel(context.Background(), "user123", "team123", req)
	if err == nil {
		t.Error("expected error for connection failure")
	}
	if !errors.Is(err, services.ErrConnectionFailed) {
		t.Errorf("expected ErrConnectionFailed, got %v", err)
	}
}

func TestService_UpdateChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	// Create a channel first
	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Original",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	req := &dto.UpdateChannelRequest{
		Label:      "Updated",
		WebhookURL: "https://hooks.slack.com/yyy",
		AppDeploy:  true,
	}

	updated, err := service.UpdateChannel(ctx, channel.ID, "user123", "team123", req)
	if err != nil {
		t.Errorf("UpdateChannel() error = %v", err)
		return
	}

	if updated.Label != "Updated" {
		t.Errorf("Label = %s, want 'Updated'", updated.Label)
	}
	if !updated.Data.AppDeploy {
		t.Error("AppDeploy should be true")
	}
}

func TestService_UpdateChannel_NotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	req := &dto.UpdateChannelRequest{Label: "Test"}
	_, err := service.UpdateChannel(ctx, "nonexistent", "user123", "team123", req)

	if err == nil {
		t.Error("expected error for non-existent channel")
	}
}

func TestService_DeleteChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeEmail,
		Label:    "To Delete",
	}
	_ = repo.Create(ctx, channel)

	err := service.DeleteChannel(ctx, channel.ID, "team123")
	if err != nil {
		t.Errorf("DeleteChannel() error = %v", err)
	}

	// Verify deleted
	_, err = repo.FindByID(ctx, channel.ID)
	if err != repositories.ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestService_DeleteChannel_WrongTeam(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeEmail,
		Label:    "Test",
	}
	_ = repo.Create(ctx, channel)

	err := service.DeleteChannel(ctx, channel.ID, "wrong-team")
	if err == nil {
		t.Error("expected error for wrong team")
	}
}

func TestService_GetChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeEmail,
		Label:    "Test",
	}
	_ = repo.Create(ctx, channel)

	found, err := service.GetChannel(ctx, channel.ID, "team123")
	if err != nil {
		t.Errorf("GetChannel() error = %v", err)
		return
	}

	if found.Label != "Test" {
		t.Errorf("Label = %s, want 'Test'", found.Label)
	}
}

func TestService_ListChannels(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	chans := []models.NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeEmail, Label: "Email 1"},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeSlack, Label: "Slack 1"},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user2", TeamID: "other-team", Provider: enums.ChannelTypeEmail, Label: "Email 2"},
	}

	for _, ch := range chans {
		_ = repo.Create(ctx, &ch)
	}

	found, err := service.ListChannels(ctx, "team123")
	if err != nil {
		t.Errorf("ListChannels() error = %v", err)
		return
	}

	if len(found) != 2 {
		t.Errorf("len(found) = %d, want 2", len(found))
	}
}

func TestService_TestChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test Slack",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	err := service.TestChannel(ctx, channel.ID, "team123", "Test message")
	if err != nil {
		t.Errorf("TestChannel() error = %v", err)
	}
}

func TestService_TestChannel_NotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	err := service.TestChannel(ctx, "nonexistent", "team123", "Test")
	if err == nil {
		t.Error("expected error for non-existent channel")
	}
}

func TestService_SendToTeam(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	chans := []models.NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeSlack, Label: "Slack 1", Connected: true, Data: models.ChannelData{WebhookURL: "https://hooks.slack.com/1"}},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeDiscord, Label: "Discord 1", Connected: true, Data: models.ChannelData{WebhookURL: "https://discord.com/api/webhooks/1"}},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeEmail, Label: "Email 1", Connected: false}, // Not connected
	}

	for _, ch := range chans {
		_ = repo.Create(ctx, &ch)
	}

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Server is ready")

	err := service.SendToTeam(ctx, "team123", notif)
	if err != nil {
		t.Errorf("SendToTeam() error = %v", err)
	}
}

func TestService_SendToTeam_AllFailed(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = db.AutoMigrate(&models.NotificationChannel{})
	repo := repositories.NewNotificationChannelRepository(db)

	mockHTTP := &channels.MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			return nil, 500, errors.New("all channels fail")
		},
	}
	factory := channels.NewFactory(mockHTTP)
	log := zerolog.Nop()
	service := services.NewNotificationChannelService(repo, factory, &log)

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(context.Background(), channel)

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test")
	err := service.SendToTeam(context.Background(), "team123", notif)

	if err == nil {
		t.Error("expected error when all channels fail")
	}
}

func TestService_SendToChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test Slack",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test notification")

	err := service.SendToChannel(ctx, channel.ID, notif)
	if err != nil {
		t.Errorf("SendToChannel() error = %v", err)
	}
}

func TestService_SendToChannel_NotConnected(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test Slack",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: false,
	}
	_ = repo.Create(ctx, channel)

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test")

	err := service.SendToChannel(ctx, channel.ID, notif)
	if err == nil {
		t.Error("expected error for disconnected channel")
	}
}

func TestService_SetChannelDefault(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	chans := []models.NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeEmail, Label: "Email 1", IsDefault: true},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team123", Provider: enums.ChannelTypeEmail, Label: "Email 2", IsDefault: false},
	}

	for _, ch := range chans {
		_ = repo.Create(ctx, &ch)
	}

	err := service.SetChannelDefault(ctx, "01HXYZ123456789ABCDEFGHI2", "team123")
	if err != nil {
		t.Errorf("SetChannelDefault() error = %v", err)
		return
	}

	ch1, _ := repo.FindByID(ctx, "01HXYZ123456789ABCDEFGHI1")
	if ch1.IsDefault {
		t.Error("Channel 1 should not be default anymore")
	}

	ch2, _ := repo.FindByID(ctx, "01HXYZ123456789ABCDEFGHI2")
	if !ch2.IsDefault {
		t.Error("Channel 2 should be default")
	}
}

func TestService_DisconnectChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeEmail,
		Label:     "Test",
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	err := service.DisconnectChannel(ctx, channel.ID, "team123")
	if err != nil {
		t.Errorf("DisconnectChannel() error = %v", err)
		return
	}

	found, _ := repo.FindByID(ctx, channel.ID)
	if found.Connected {
		t.Error("channel should be disconnected")
	}
}

func TestService_ReconnectChannel(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: false,
	}
	_ = repo.Create(ctx, channel)

	err := service.ReconnectChannel(ctx, channel.ID, "team123")
	if err != nil {
		t.Errorf("ReconnectChannel() error = %v", err)
		return
	}

	found, _ := repo.FindByID(ctx, channel.ID)
	if !found.Connected {
		t.Error("channel should be reconnected")
	}
}

func TestService_ReconnectChannel_Failed(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = db.AutoMigrate(&models.NotificationChannel{})
	repo := repositories.NewNotificationChannelRepository(db)

	mockHTTP := &channels.MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			return nil, 500, errors.New("connection failed")
		},
	}
	factory := channels.NewFactory(mockHTTP)
	log := zerolog.Nop()
	service := services.NewNotificationChannelService(repo, factory, &log)

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeSlack,
		Label:     "Test",
		Data:      models.ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
		Connected: false,
	}
	_ = repo.Create(context.Background(), channel)

	err := service.ReconnectChannel(context.Background(), channel.ID, "team123")
	if err == nil {
		t.Error("expected error for reconnection failure")
	}
}

func TestNewService(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	repo := repositories.NewNotificationChannelRepository(db)
	factory := channels.NewFactory(&channels.MockHTTPClient{})
	log := zerolog.Nop()

	service := services.NewNotificationChannelService(repo, factory, &log)

	if service == nil {
		t.Error("NewNotificationChannelService() returned nil")
	}
}

func TestServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidProvider", services.ErrInvalidProvider},
		{"ErrConnectionFailed", services.ErrConnectionFailed},
		{"ErrNotificationFailed", services.ErrNotificationFailed},
		{"ErrUnauthorized", services.ErrUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s.Error() should not be empty", tt.name)
			}
		})
	}
}

func TestService_SendToTeam_NoChannels(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test")
	err := service.SendToTeam(ctx, "nonexistent-team", notif)

	// Should not error if no channels, just no-op
	if err != nil {
		t.Errorf("SendToTeam() unexpected error: %v", err)
	}
}

func TestService_SendToChannel_NotFound(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test")
	err := service.SendToChannel(ctx, "nonexistent", notif)

	if err == nil {
		t.Error("expected error for non-existent channel")
	}
}

func TestService_SendToTeam_WithEmail(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  enums.ChannelTypeEmail,
		Label:     "Test Email",
		Data:      models.ChannelData{Email: "test@example.com"},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test email notification")

	err := service.SendToTeam(ctx, "team123", notif)
	if err != nil {
		t.Errorf("SendToTeam() error = %v", err)
	}
}

func TestService_SendToTeam_WithTelegram(t *testing.T) {
	service, repo := setupTestService(t)
	ctx := context.Background()

	channel := &models.NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: enums.ChannelTypeTelegram,
		Label:    "Test Telegram",
		Data: models.ChannelData{
			BotToken: "123456:ABC",
			ChatID:   "-123456789",
		},
		Connected: true,
	}
	_ = repo.Create(ctx, channel)

	notif := models.NewBaseNotification(enums.NotificationTypeServerProvisioned, "Test telegram notification")

	err := service.SendToTeam(ctx, "team123", notif)
	if err != nil {
		t.Errorf("SendToTeam() error = %v", err)
	}
}
