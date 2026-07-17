package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

func TestWithDBBindsEveryRepositoryToTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Domain{}, &models.DNSRecord{}))

	repositories := NewRegistry(db)
	expectedError := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		txRepos := repositories.WithDB(tx)
		domain := &models.Domain{
			DomainProviderID: util.NewULID(),
			ProviderID:       "provider-domain-id",
			Label:            "example.com",
			Address:          "example.com",
		}
		domain.ID = util.NewULID()
		domain.UserID = "user-1"
		domain.TeamID = "team-1"
		require.NoError(t, txRepos.Domain().Create(context.Background(), domain))

		record := &models.DNSRecord{
			DomainID:   domain.ID,
			ProviderID: "provider-record-id",
			Type:       "A",
			Name:       "@",
			Value:      "192.0.2.1",
			TTL:        300,
		}
		record.ID = util.NewULID()
		record.TeamID = "team-1"
		require.NoError(t, txRepos.DNSRecord().Create(context.Background(), record))

		return expectedError
	})
	require.ErrorIs(t, err, expectedError)

	var domainCount int64
	require.NoError(t, db.Model(&models.Domain{}).Count(&domainCount).Error)
	assert.Zero(t, domainCount)

	var recordCount int64
	require.NoError(t, db.Model(&models.DNSRecord{}).Count(&recordCount).Error)
	assert.Zero(t, recordCount)
}

func TestUpdateOrCreateDoesNotMutateAttributesAndReturnsCreatedModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Domain{}, &models.DNSRecord{}))
	repositories := NewRegistry(db)
	ctx := context.Background()

	domainMatch := map[string]any{
		"provider_id":        "provider-domain-id",
		"domain_provider_id": util.NewULID(),
		"address":            "example.com",
		"team_id":            "team-1",
	}
	domainUpdates := map[string]any{
		"user_id": "user-1",
		"label":   "example.com",
	}
	domain, err := repositories.Domain().UpdateOrCreate(ctx, domainMatch, domainUpdates)
	require.NoError(t, err)
	assert.NotEmpty(t, domain.ID)
	assert.NotContains(t, domainUpdates, "provider_id")

	recordMatch := map[string]any{
		"domain_id":   domain.ID,
		"provider_id": "provider-record-id",
		"type":        "A",
		"name":        "@",
	}
	recordUpdates := map[string]any{
		"team_id": "team-1",
		"value":   "192.0.2.1",
		"ttl":     300,
	}
	record, err := repositories.DNSRecord().UpdateOrCreate(ctx, recordMatch, recordUpdates)
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.NotContains(t, recordUpdates, "domain_id")

	recordUpdates["value"] = "192.0.2.2"
	updated, err := repositories.DNSRecord().UpdateOrCreate(ctx, recordMatch, recordUpdates)
	require.NoError(t, err)
	assert.Equal(t, record.ID, updated.ID)
	assert.Equal(t, "192.0.2.2", updated.Value)
}
