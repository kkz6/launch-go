package notification

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&NotificationChannel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  ChannelTypeEmail,
		Label:     "Test Email",
		Data:      ChannelData{Email: "test@example.com"},
		Connected: true,
	}

	err := repo.Create(ctx, channel)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// Verify it was created
	found, err := repo.FindByID(ctx, channel.ID)
	if err != nil {
		t.Errorf("FindByID() error = %v", err)
	}
	if found.Label != "Test Email" {
		t.Errorf("Label = %v, want 'Test Email'", found.Label)
	}
}

func TestRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "Original Label",
		Data:     ChannelData{Email: "test@example.com"},
	}

	_ = repo.Create(ctx, channel)

	channel.Label = "Updated Label"
	err := repo.Update(ctx, channel)
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}

	found, _ := repo.FindByID(ctx, channel.ID)
	if found.Label != "Updated Label" {
		t.Errorf("Label = %v, want 'Updated Label'", found.Label)
	}
}

func TestRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "To Delete",
		Data:     ChannelData{Email: "test@example.com"},
	}

	_ = repo.Create(ctx, channel)

	err := repo.Delete(ctx, channel.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify it was deleted (soft delete)
	_, err = repo.FindByID(ctx, channel.ID)
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeSlack,
		Label:    "Slack Channel",
		Data:     ChannelData{WebhookURL: "https://hooks.slack.com/xxx"},
	}

	_ = repo.Create(ctx, channel)

	found, err := repo.FindByID(ctx, channel.ID)
	if err != nil {
		t.Errorf("FindByID() error = %v", err)
	}
	if found.Provider != ChannelTypeSlack {
		t.Errorf("Provider = %v, want slack", found.Provider)
	}
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestRepository_FindByIDAndTeamID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "Team Channel",
		Data:     ChannelData{Email: "test@example.com"},
	}

	_ = repo.Create(ctx, channel)

	// Find with correct team
	found, err := repo.FindByIDAndTeamID(ctx, channel.ID, "team123")
	if err != nil {
		t.Errorf("FindByIDAndTeamID() error = %v", err)
	}
	if found.Label != "Team Channel" {
		t.Errorf("Label = %v, want 'Team Channel'", found.Label)
	}

	// Find with wrong team
	_, err = repo.FindByIDAndTeamID(ctx, channel.ID, "wrong-team")
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestRepository_FindByTeamID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channels := []NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 1"},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team1", Provider: ChannelTypeSlack, Label: "Slack 1"},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user2", TeamID: "team2", Provider: ChannelTypeEmail, Label: "Email 2"},
	}

	for _, ch := range channels {
		_ = repo.Create(ctx, &ch)
	}

	found, err := repo.FindByTeamID(ctx, "team1")
	if err != nil {
		t.Errorf("FindByTeamID() error = %v", err)
	}
	if len(found) != 2 {
		t.Errorf("len(found) = %d, want 2", len(found))
	}
}

func TestRepository_FindByUserID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channels := []NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 1"},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team2", Provider: ChannelTypeSlack, Label: "Slack 1"},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user2", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 2"},
	}

	for _, ch := range channels {
		_ = repo.Create(ctx, &ch)
	}

	found, err := repo.FindByUserID(ctx, "user1")
	if err != nil {
		t.Errorf("FindByUserID() error = %v", err)
	}
	if len(found) != 2 {
		t.Errorf("len(found) = %d, want 2", len(found))
	}
}

func TestRepository_FindByProvider(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channels := []NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 1"},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team1", Provider: ChannelTypeSlack, Label: "Slack 1"},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 2"},
	}

	for _, ch := range channels {
		_ = repo.Create(ctx, &ch)
	}

	found, err := repo.FindByProvider(ctx, "team1", ChannelTypeEmail)
	if err != nil {
		t.Errorf("FindByProvider() error = %v", err)
	}
	if len(found) != 2 {
		t.Errorf("len(found) = %d, want 2", len(found))
	}
}

func TestRepository_FindConnected(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channels := []NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 1", Connected: true},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team1", Provider: ChannelTypeSlack, Label: "Slack 1", Connected: false},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 2", Connected: true},
	}

	for _, ch := range channels {
		_ = repo.Create(ctx, &ch)
	}

	found, err := repo.FindConnected(ctx, "team1")
	if err != nil {
		t.Errorf("FindConnected() error = %v", err)
	}
	if len(found) != 2 {
		t.Errorf("len(found) = %d, want 2", len(found))
	}
}

func TestRepository_SetConnected(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:        "01HXYZ123456789ABCDEFGHIJ",
		UserID:    "user123",
		TeamID:    "team123",
		Provider:  ChannelTypeEmail,
		Label:     "Test",
		Connected: false,
	}

	_ = repo.Create(ctx, channel)

	err := repo.SetConnected(ctx, channel.ID, true)
	if err != nil {
		t.Errorf("SetConnected() error = %v", err)
	}

	found, _ := repo.FindByID(ctx, channel.ID)
	if !found.Connected {
		t.Error("Connected should be true")
	}
}

func TestRepository_SetDefault(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channels := []NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 1", IsDefault: true},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 2", IsDefault: false},
	}

	for _, ch := range channels {
		_ = repo.Create(ctx, &ch)
	}

	// Set second channel as default
	err := repo.SetDefault(ctx, "01HXYZ123456789ABCDEFGHI2", "team1", ChannelTypeEmail)
	if err != nil {
		t.Errorf("SetDefault() error = %v", err)
	}

	// First should no longer be default
	ch1, _ := repo.FindByID(ctx, "01HXYZ123456789ABCDEFGHI1")
	if ch1.IsDefault {
		t.Error("Channel 1 IsDefault should be false")
	}

	// Second should be default
	ch2, _ := repo.FindByID(ctx, "01HXYZ123456789ABCDEFGHI2")
	if !ch2.IsDefault {
		t.Error("Channel 2 IsDefault should be true")
	}
}

func TestRepository_Exists(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "Test",
	}

	_ = repo.Create(ctx, channel)

	exists, err := repo.Exists(ctx, channel.ID)
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() should return true")
	}

	exists, err = repo.Exists(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() should return false for nonexistent")
	}
}

func TestRepository_CountByTeamID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channels := []NotificationChannel{
		{ID: "01HXYZ123456789ABCDEFGHI1", UserID: "user1", TeamID: "team1", Provider: ChannelTypeEmail, Label: "Email 1"},
		{ID: "01HXYZ123456789ABCDEFGHI2", UserID: "user1", TeamID: "team1", Provider: ChannelTypeSlack, Label: "Slack 1"},
		{ID: "01HXYZ123456789ABCDEFGHI3", UserID: "user2", TeamID: "team2", Provider: ChannelTypeEmail, Label: "Email 2"},
	}

	for _, ch := range channels {
		_ = repo.Create(ctx, &ch)
	}

	count, err := repo.CountByTeamID(ctx, "team1")
	if err != nil {
		t.Errorf("CountByTeamID() error = %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestRepository_UpdateData(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "Test",
		Data:     ChannelData{Email: "old@example.com"},
	}

	_ = repo.Create(ctx, channel)

	newData := ChannelData{Email: "new@example.com", AppDeploy: true}
	err := repo.UpdateData(ctx, channel.ID, newData)
	if err != nil {
		t.Errorf("UpdateData() error = %v", err)
	}

	found, _ := repo.FindByID(ctx, channel.ID)
	if found.Data.Email != "new@example.com" {
		t.Errorf("Email = %v, want new@example.com", found.Data.Email)
	}
}

func TestNotificationChannel_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create without ID - should auto-generate
	channel := &NotificationChannel{
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "Auto ID",
		Data:     ChannelData{Email: "test@example.com"},
	}

	err := db.WithContext(ctx).Create(channel).Error
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}

	if channel.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if len(channel.ID) != 26 {
		t.Errorf("ID length = %d, want 26 (ULID)", len(channel.ID))
	}
}

func TestRepository_CreatedAt_UpdatedAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	channel := &NotificationChannel{
		ID:       "01HXYZ123456789ABCDEFGHIJ",
		UserID:   "user123",
		TeamID:   "team123",
		Provider: ChannelTypeEmail,
		Label:    "Test",
	}

	beforeCreate := time.Now()
	_ = repo.Create(ctx, channel)
	afterCreate := time.Now()

	found, _ := repo.FindByID(ctx, channel.ID)

	if found.CreatedAt.Before(beforeCreate) || found.CreatedAt.After(afterCreate) {
		t.Error("CreatedAt should be between beforeCreate and afterCreate")
	}
}
