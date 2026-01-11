package auth

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

func TestNewModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{
		App: config.AppConfig{
			Name: "TestApp",
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			Expiration: 24,
		},
	}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)

	assert.NotNil(t, module)
	assert.NotNil(t, module.Handler())
	assert.NotNil(t, module.Service())
	assert.NotNil(t, module.Repository())
}

func TestModule_RegisterRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{
		App: config.AppConfig{
			Name: "TestApp",
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			Expiration: 24,
		},
	}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)
	app := fiber.New()

	// This should not panic
	module.RegisterRoutes(app)

	// Verify routes are registered by checking the stack
	stack := app.Stack()
	assert.NotEmpty(t, stack)
}

func TestModule_Service(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)

	service := module.Service()
	assert.NotNil(t, service)
}

func TestModule_Repository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)

	repo := module.Repository()
	assert.NotNil(t, repo)
}

func TestModule_Handler(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)

	handler := module.Handler()
	assert.NotNil(t, handler)
}

func TestModule_AutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)

	err = module.AutoMigrate(db)
	require.NoError(t, err)

	// Verify tables were created
	assert.True(t, db.Migrator().HasTable(&models.User{}))
	assert.True(t, db.Migrator().HasTable(&models.Team{}))
	assert.True(t, db.Migrator().HasTable(&models.TeamMember{}))
	assert.True(t, db.Migrator().HasTable(&models.TeamInvitation{}))
	assert.True(t, db.Migrator().HasTable(&models.PersonalAccessToken{}))
	assert.True(t, db.Migrator().HasTable(&models.PasswordResetToken{}))
}

func TestModule_GetModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{}
	logger := zerolog.Nop()

	module := NewModule(db, cfg, &logger)

	modelsSlice := module.GetModels()
	assert.Len(t, modelsSlice, 6)
}
