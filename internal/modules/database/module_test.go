package database

import (
	"context"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/websocket"
)

// MockServerRepository implements ServerRepository for testing
type MockServerRepository struct {
	servers map[string]interface{}
}

func NewMockServerRepository() *MockServerRepository {
	return &MockServerRepository{
		servers: make(map[string]interface{}),
	}
}

func (m *MockServerRepository) AddServer(id string, server interface{}) {
	m.servers[id] = server
}

func (m *MockServerRepository) FindByID(ctx context.Context, id string) (interface{}, error) {
	if server, ok := m.servers[id]; ok {
		return server, nil
	}

	return nil, services.ErrServerNotFound
}

func (m *MockServerRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (interface{}, error) {
	if server, ok := m.servers[id]; ok {
		return server, nil
	}

	return nil, services.ErrServerNotFound
}

func TestNewModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Database{}, &models.DatabaseUser{}, &models.DatabaseDatabaseUser{})
	require.NoError(t, err)

	serverRepo := NewMockServerRepository()
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	module := NewModule(db, serverRepo, nil, ws, &logger)

	assert.NotNil(t, module)
	assert.NotNil(t, module.handler)
}

func TestModule_RegisterRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Database{}, &models.DatabaseUser{}, &models.DatabaseDatabaseUser{})
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

	assert.NotNil(t, app)
}
