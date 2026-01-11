package billing

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDefaultPlans(t *testing.T) {
	plans := models.DefaultPlans()

	assert.Len(t, plans, 3)

	// Verify first plan (Starter)
	starter := plans[0]
	assert.Equal(t, "1", starter.ID)
	assert.Equal(t, "Starter", starter.Name)
	assert.Equal(t, 3, starter.Options.MaxServers)
	assert.Equal(t, 5, starter.Options.MaxSitesPerServer)
	assert.False(t, starter.Options.HasBackups)
	assert.False(t, starter.Options.HasMonitoring)

	// Verify second plan (Pro)
	pro := plans[1]
	assert.Equal(t, "2", pro.ID)
	assert.Equal(t, "Pro", pro.Name)
	assert.Equal(t, 10, pro.Options.MaxServers)
	assert.Equal(t, 20, pro.Options.MaxSitesPerServer)
	assert.False(t, pro.Options.HasBackups)
	assert.True(t, pro.Options.HasMonitoring)

	// Verify third plan (Enterprise)
	enterprise := plans[2]
	assert.Equal(t, "3", enterprise.ID)
	assert.Equal(t, "Enterprise", enterprise.Name)
	assert.Equal(t, 999999, enterprise.Options.MaxServers)
	assert.Equal(t, 999999, enterprise.Options.MaxSitesPerServer)
	assert.True(t, enterprise.Options.HasBackups)
	assert.True(t, enterprise.Options.HasMonitoring)
}

func TestNewModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	log := zerolog.Nop()

	t.Run("with LemonSqueezy config", func(t *testing.T) {
		config := &ModuleConfig{
			DB:                   db,
			Logger:               &log,
			SubscriptionsEnabled: true,
			Plans:                models.DefaultPlans(),
			WebhookSecret:        "test_secret",
			LemonSqueezy: &providers.LemonSqueezyConfig{
				APIKey:  "test_key",
				StoreID: 123,
			},
		}

		module := NewModule(config)

		assert.NotNil(t, module)
		assert.NotNil(t, module.GetService())
		assert.NotNil(t, module.GetRepository())
		assert.NotNil(t, module.GetWebhookHandler())
	})

	t.Run("without LemonSqueezy config", func(t *testing.T) {
		config := &ModuleConfig{
			DB:                   db,
			Logger:               &log,
			SubscriptionsEnabled: true,
			Plans:                models.DefaultPlans(),
			WebhookSecret:        "test_secret",
		}

		module := NewModule(config)

		assert.NotNil(t, module)
		assert.NotNil(t, module.GetService())
	})

	t.Run("with empty LemonSqueezy API key", func(t *testing.T) {
		config := &ModuleConfig{
			DB:                   db,
			Logger:               &log,
			SubscriptionsEnabled: true,
			Plans:                models.DefaultPlans(),
			WebhookSecret:        "test_secret",
			LemonSqueezy: &providers.LemonSqueezyConfig{
				APIKey:  "",
				StoreID: 123,
			},
		}

		module := NewModule(config)

		assert.NotNil(t, module)
	})
}

func TestModule_RegisterRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	log := zerolog.Nop()

	config := &ModuleConfig{
		DB:                   db,
		Logger:               &log,
		SubscriptionsEnabled: true,
		Plans:                models.DefaultPlans(),
		WebhookSecret:        "test_secret",
	}

	module := NewModule(config)
	app := fiber.New()

	// This should not panic
	module.RegisterRoutes(app, nil)

	// Verify routes are registered
	routes := app.GetRoutes()
	assert.NotEmpty(t, routes)
}

func TestModule_Migrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	log := zerolog.Nop()

	config := &ModuleConfig{
		DB:                   db,
		Logger:               &log,
		SubscriptionsEnabled: true,
		Plans:                models.DefaultPlans(),
		WebhookSecret:        "test_secret",
	}

	module := NewModule(config)

	err = module.Migrate(db)
	assert.NoError(t, err)

	// Verify tables exist
	assert.True(t, db.Migrator().HasTable(&models.Subscription{}))
	assert.True(t, db.Migrator().HasTable(&models.Order{}))
	assert.True(t, db.Migrator().HasTable(&models.WebhookEvent{}))
}

func TestModule_GettersReturnNonNil(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	log := zerolog.Nop()

	config := &ModuleConfig{
		DB:                   db,
		Logger:               &log,
		SubscriptionsEnabled: true,
		Plans:                models.DefaultPlans(),
		WebhookSecret:        "test_secret",
	}

	module := NewModule(config)

	assert.NotNil(t, module.GetService())
	assert.NotNil(t, module.GetRepository())
	assert.NotNil(t, module.GetWebhookHandler())
}
