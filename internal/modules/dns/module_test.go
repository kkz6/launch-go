package dns

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestModule(t *testing.T) (*Module, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&DomainProvider{}, &Domain{}, &DnsRecord{})
	require.NoError(t, err)

	logger := zerolog.Nop()
	module := NewModule(db, &logger)

	return module, db
}

func TestNewModule(t *testing.T) {
	module, _ := setupTestModule(t)

	assert.NotNil(t, module)
	assert.NotNil(t, module.handler)
	assert.NotNil(t, module.service)
	assert.NotNil(t, module.repo)
}

func TestModule_GetService(t *testing.T) {
	module, _ := setupTestModule(t)

	service := module.GetService()
	assert.NotNil(t, service)
}

func TestModule_GetRepository(t *testing.T) {
	module, _ := setupTestModule(t)

	repo := module.GetRepository()
	assert.NotNil(t, repo)
}

func TestModule_RegisterRoutes(t *testing.T) {
	module, _ := setupTestModule(t)

	app := fiber.New()
	router := app.Group("/api")

	// Mock auth middleware
	authMiddleware := func(c *fiber.Ctx) error {
		c.Locals("userID", "user123")
		c.Locals("teamID", "team123")
		return c.Next()
	}

	// Should not panic
	module.RegisterRoutes(router, authMiddleware)

	// Verify routes were registered by checking the app's routes
	routes := app.GetRoutes()
	routePaths := make(map[string]bool)
	for _, route := range routes {
		routePaths[route.Method+" "+route.Path] = true
	}

	// Provider routes
	assert.True(t, routePaths["GET /api/dns-providers/"], "GET /api/dns-providers should be registered")
	assert.True(t, routePaths["POST /api/dns-providers/"], "POST /api/dns-providers should be registered")
	assert.True(t, routePaths["DELETE /api/dns-providers/:id"], "DELETE /api/dns-providers/:id should be registered")
	assert.True(t, routePaths["POST /api/dns-providers/:id/check"], "POST /api/dns-providers/:id/check should be registered")
	assert.True(t, routePaths["POST /api/dns-providers/:id/sync"], "POST /api/dns-providers/:id/sync should be registered")

	// Domain routes
	assert.True(t, routePaths["GET /api/domains/"], "GET /api/domains should be registered")
	assert.True(t, routePaths["POST /api/domains/"], "POST /api/domains should be registered")
	assert.True(t, routePaths["GET /api/domains/:id"], "GET /api/domains/:id should be registered")
	assert.True(t, routePaths["DELETE /api/domains/:id"], "DELETE /api/domains/:id should be registered")

	// Record routes
	assert.True(t, routePaths["GET /api/domains/:id/records"], "GET /api/domains/:id/records should be registered")
	assert.True(t, routePaths["POST /api/domains/:id/records"], "POST /api/domains/:id/records should be registered")
	assert.True(t, routePaths["PUT /api/domains/:domainId/records/:recordId"], "PUT /api/domains/:domainId/records/:recordId should be registered")
	assert.True(t, routePaths["DELETE /api/domains/:domainId/records/:recordId"], "DELETE /api/domains/:domainId/records/:recordId should be registered")

	// Utility routes
	assert.True(t, routePaths["GET /api/dns/record-types"], "GET /api/dns/record-types should be registered")
}

func TestModule_AutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	logger := zerolog.Nop()
	module := NewModule(db, &logger)

	// AutoMigrate should create all tables
	err = module.AutoMigrate(db)
	require.NoError(t, err)

	// Verify tables exist by checking if we can query them
	var count int64
	err = db.Model(&DomainProvider{}).Count(&count).Error
	require.NoError(t, err)

	err = db.Model(&Domain{}).Count(&count).Error
	require.NoError(t, err)

	err = db.Model(&DnsRecord{}).Count(&count).Error
	require.NoError(t, err)
}

func TestModule_AutoMigrate_Idempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	logger := zerolog.Nop()
	module := NewModule(db, &logger)

	// Run AutoMigrate twice - should not fail
	err = module.AutoMigrate(db)
	require.NoError(t, err)

	err = module.AutoMigrate(db)
	require.NoError(t, err)
}
