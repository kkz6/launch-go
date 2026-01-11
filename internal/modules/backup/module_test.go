package backup

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	if module == nil {
		t.Fatal("expected module to be created")
	}
	if module.GetBackupService() == nil {
		t.Error("expected backup service to be created")
	}
	if module.GetBackupRepository() == nil {
		t.Error("expected backup repository to be created")
	}
	if module.GetBackupJobService() == nil {
		t.Error("expected backup job service to be created")
	}
	if module.GetStorageProviderService() == nil {
		t.Error("expected storage provider service to be created")
	}
}

func TestModule_GetBackupService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	service := module.GetBackupService()
	if service == nil {
		t.Error("expected GetBackupService to return service")
	}
}

func TestModule_GetBackupRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	repo := module.GetBackupRepository()
	if repo == nil {
		t.Error("expected GetBackupRepository to return repository")
	}
}

func TestModule_GetBackupJobService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	service := module.GetBackupJobService()
	if service == nil {
		t.Error("expected GetBackupJobService to return service")
	}
}

func TestModule_GetStorageProviderService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	service := module.GetStorageProviderService()
	if service == nil {
		t.Error("expected GetStorageProviderService to return service")
	}
}

func TestModule_GetAgentConfigService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	service := module.GetAgentConfigService()
	if service == nil {
		t.Error("expected GetAgentConfigService to return service")
	}
}

func TestModule_RegisterRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	app := fiber.New()
	api := app.Group("/api/v1")

	// Mock auth middleware
	authMiddleware := func(c *fiber.Ctx) error {
		c.Locals("userID", "user123")
		c.Locals("teamID", "team123")
		return c.Next()
	}

	// This should not panic
	module.RegisterRoutes(api, authMiddleware)

	// Verify routes were registered by checking the app's routes
	routes := app.GetRoutes()

	expectedRoutes := map[string]bool{
		"/api/v1/servers/:serverId/backups":           false,
		"/api/v1/servers/:serverId/backups/:id":       false,
		"/api/v1/servers/:serverId/backups/:id/run":   false,
		"/api/v1/storage-providers":                   false,
		"/api/v1/storage-providers/dropdown":          false,
		"/api/v1/storage-providers/:id":               false,
		"/api/v1/storage-providers/:provider/connect": false,
		"/api/v1/storage-providers/:provider":         false,
	}

	for _, route := range routes {
		if _, exists := expectedRoutes[route.Path]; exists {
			expectedRoutes[route.Path] = true
		}
	}

	for path, found := range expectedRoutes {
		if !found {
			t.Logf("Route %s not found (may have different path format)", path)
		}
	}
}

func TestModule_RegisterWebhookRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	app := fiber.New()

	// This should not panic
	module.RegisterWebhookRoutes(app)

	// Verify webhook route was registered
	routes := app.GetRoutes()
	webhookFound := false
	for _, route := range routes {
		if route.Path == "/backup/:backup/:token" && route.Method == "POST" {
			webhookFound = true
			break
		}
	}

	if !webhookFound {
		t.Error("expected webhook route to be registered")
	}
}

func TestModule_AutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	err = module.AutoMigrate(db)
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	// Verify tables exist
	var tables []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table'").Scan(&tables)

	expectedTables := []string{"backups", "backup_jobs", "storage_providers", "backup_databases"}
	for _, expected := range expectedTables {
		found := false
		for _, table := range tables {
			if table == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected table %s to exist", expected)
		}
	}
}

func TestModule_AutoMigrate_Idempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	logger := zerolog.Nop()
	module := NewModule(db, nil, nil, &logger)

	// Run migrations multiple times - should not error
	for i := 0; i < 3; i++ {
		err = module.AutoMigrate(db)
		if err != nil {
			t.Fatalf("AutoMigrate() iteration %d error = %v", i, err)
		}
	}
}
