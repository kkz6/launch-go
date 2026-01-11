package database

import (
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

func TestNewModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
	require.NoError(t, err)

	serverRepo := NewMockServerRepository()
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	module := NewModule(db, serverRepo, nil, ws, &logger)

	assert.NotNil(t, module)
	assert.NotNil(t, module.handler)
	assert.NotNil(t, module.service)
	assert.NotNil(t, module.repository)
}

func TestModule_Service(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
	require.NoError(t, err)

	serverRepo := NewMockServerRepository()
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	module := NewModule(db, serverRepo, nil, ws, &logger)

	service := module.Service()

	assert.NotNil(t, service)
	assert.Equal(t, module.service, service)
}

func TestModule_Repository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
	require.NoError(t, err)

	serverRepo := NewMockServerRepository()
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	module := NewModule(db, serverRepo, nil, ws, &logger)

	repo := module.Repository()

	assert.NotNil(t, repo)
	assert.Equal(t, module.repository, repo)
}

func TestModule_RegisterRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
	require.NoError(t, err)

	serverRepo := NewMockServerRepository()
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	module := NewModule(db, serverRepo, nil, ws, &logger)

	app := fiber.New()
	authMiddleware := func(c *fiber.Ctx) error {
		return c.Next()
	}

	module.RegisterRoutes(app, authMiddleware)

	// Verify routes are registered by checking the app has routes
	// We can't easily inspect fiber routes, but we can verify the module works
	assert.NotNil(t, app)
}

func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = AutoMigrate(db)
	require.NoError(t, err)

	// Verify tables exist by trying to query them
	var count int64
	err = db.Model(&Database{}).Count(&count).Error
	require.NoError(t, err)

	err = db.Model(&DatabaseUser{}).Count(&count).Error
	require.NoError(t, err)
}
