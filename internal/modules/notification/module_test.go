package notification

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
)

func TestNewModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}
	log := zerolog.Nop()

	module := NewModule(db, cfg, &log)

	if module == nil {
		t.Error("NewModule() returned nil")
	}
	if module.handler == nil {
		t.Error("handler should not be nil")
	}
	if module.config != cfg {
		t.Error("config not set correctly")
	}
}

func TestNewModuleWithDependencies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	repo := NewRepository(db)
	factory := channels.NewFactory(&channels.MockHTTPClient{})
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}
	log := zerolog.Nop()

	module := NewModuleWithDependencies(repo, factory, cfg, &log)

	if module == nil {
		t.Error("NewModuleWithDependencies() returned nil")
	}
}

func TestModule_RegisterRoutes(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = db.AutoMigrate(&NotificationChannel{})

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}
	log := zerolog.Nop()

	module := NewModule(db, cfg, &log)

	app := fiber.New()
	api := app.Group("/api")

	module.RegisterRoutes(api)

	// Get all registered routes
	routes := app.GetRoutes()

	expectedPaths := []string{
		"/api/settings/notifications/",
		"/api/settings/notifications/:id",
		"/api/settings/notifications/:id/test",
		"/api/settings/notifications/:id/default",
		"/api/settings/notifications/:id/disconnect",
		"/api/settings/notifications/:id/reconnect",
	}

	registeredPaths := make(map[string]bool)
	for _, route := range routes {
		registeredPaths[route.Path] = true
	}

	for _, path := range expectedPaths {
		if !registeredPaths[path] {
			t.Errorf("expected route %s to be registered", path)
		}
	}
}

func TestModule_GetService(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}
	log := zerolog.Nop()

	module := NewModule(db, cfg, &log)

	service := module.GetService()

	if service == nil {
		t.Error("GetService() returned nil")
	}
}

func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	err = AutoMigrate(db)
	if err != nil {
		t.Errorf("AutoMigrate() error = %v", err)
	}

	// Verify table exists by trying to query it
	var count int64
	err = db.Model(&NotificationChannel{}).Count(&count).Error
	if err != nil {
		t.Errorf("table should exist after migration: %v", err)
	}
}

func TestModule_RouteMethods(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = db.AutoMigrate(&NotificationChannel{})

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}
	log := zerolog.Nop()

	module := NewModule(db, cfg, &log)

	app := fiber.New()
	api := app.Group("/api")

	module.RegisterRoutes(api)

	routes := app.GetRoutes()

	methodChecks := map[string][]string{
		"/api/settings/notifications/":             {"GET", "POST"},
		"/api/settings/notifications/:id":          {"GET", "PUT", "DELETE"},
		"/api/settings/notifications/:id/test":     {"POST"},
		"/api/settings/notifications/:id/default":  {"POST"},
		"/api/settings/notifications/:id/disconnect": {"POST"},
		"/api/settings/notifications/:id/reconnect":  {"POST"},
	}

	registeredMethods := make(map[string][]string)
	for _, route := range routes {
		registeredMethods[route.Path] = append(registeredMethods[route.Path], route.Method)
	}

	for path, expectedMethods := range methodChecks {
		actualMethods := registeredMethods[path]
		for _, method := range expectedMethods {
			found := false
			for _, actual := range actualMethods {
				if actual == method {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %s %s to be registered", method, path)
			}
		}
	}
}
