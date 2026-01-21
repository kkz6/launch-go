package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testModel is a simple model for testing scopes
type testModel struct {
	ID       string `gorm:"primaryKey"`
	Name     string
	ServerID string
	TeamID   string
	UserID   string
	SiteID   string
	Status   string
	Type     string
	Address  string
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	if err := db.AutoMigrate(&testModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func seedTestData(t *testing.T, db *gorm.DB) {
	models := []testModel{
		{ID: "1", Name: "test1", ServerID: "srv1", TeamID: "team1", UserID: "user1", SiteID: "site1", Status: "active", Type: "web", Address: "example.com"},
		{ID: "2", Name: "test2", ServerID: "srv1", TeamID: "team1", UserID: "user2", SiteID: "site1", Status: "active", Type: "api", Address: "api.example.com"},
		{ID: "3", Name: "test3", ServerID: "srv2", TeamID: "team2", UserID: "user1", SiteID: "site2", Status: "inactive", Type: "web", Address: "other.com"},
	}
	for _, m := range models {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("failed to seed data: %v", err)
		}
	}
}

func TestWithID(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	result, err := FindOne[testModel](ctx, db, WithID("1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected ID 1, got %s", result.ID)
	}
}

func TestWithServerID(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, WithServerID("srv1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestWithTeamID(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, WithTeamID("team1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestWithUserID(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, WithUserID("user1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestWithSiteID(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, WithSiteID("site1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestWithName(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	result, err := FindOne[testModel](ctx, db, WithName("test1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Name != "test1" {
		t.Errorf("expected name test1, got %s", result.Name)
	}
}

func TestWithStatus(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, WithStatus("active"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestWithType(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, WithType("web"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestWithAddress(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	result, err := FindOne[testModel](ctx, db, WithAddress("example.com"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Address != "example.com" {
		t.Errorf("expected address example.com, got %s", result.Address)
	}
}

func TestCombinedScopes(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	// Find by server and status
	results, err := FindAll[testModel](ctx, db, WithServerID("srv1"), WithStatus("active"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Find by server and type
	results, err = FindAll[testModel](ctx, db, WithServerID("srv1"), WithType("web"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestFindOneNotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	_, err := FindOne[testModel](ctx, db, WithID("nonexistent"))
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFindOneOrFail(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	// Should succeed
	result, err := FindOneOrFail[testModel](ctx, db, WithID("1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected ID 1, got %s", result.ID)
	}

	// Should fail with typed error
	_, err = FindOneOrFail[testModel](ctx, db, WithID("nonexistent"))
	if err == nil {
		t.Error("expected error, got nil")
	}
	_, ok := err.(*ModelError)
	if !ok {
		t.Errorf("expected *ModelError, got %T", err)
	}
}

func TestCount(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	count, err := Count[testModel](ctx, db, WithServerID("srv1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestExists(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	exists, err := Exists[testModel](ctx, db, WithID("1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !exists {
		t.Error("expected exists to be true")
	}

	exists, err = Exists[testModel](ctx, db, WithID("nonexistent"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if exists {
		t.Error("expected exists to be false")
	}
}

func TestLimit(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, Limit(2))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestOffset(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	results, err := FindAll[testModel](ctx, db, Offset(1), Limit(10))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestUpdateAll(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	err := UpdateAll[testModel](ctx, db, map[string]interface{}{"status": "updated"}, WithServerID("srv1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	results, _ := FindAll[testModel](ctx, db, WithStatus("updated"))
	if len(results) != 2 {
		t.Errorf("expected 2 updated results, got %d", len(results))
	}
}

func TestDeleteAll(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	ctx := context.Background()

	err := DeleteAll[testModel](ctx, db, WithServerID("srv1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	count, _ := Count[testModel](ctx, db)
	if count != 1 {
		t.Errorf("expected 1 remaining record, got %d", count)
	}
}
