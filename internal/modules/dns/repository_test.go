package dns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
)

func setupTestRepository(t *testing.T) (*repositories.DomainProviderRepository, *repositories.DomainRepository, *repositories.DnsRecordRepository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DomainProvider{}, &models.Domain{}, &models.DnsRecord{})
	require.NoError(t, err)

	providerRepo := repositories.NewDomainProviderRepository(db)
	domainRepo := repositories.NewDomainRepository(db)
	dnsRecordRepo := repositories.NewDnsRecordRepository(db)

	return providerRepo, domainRepo, dnsRecordRepo, db
}

func createTestProvider(t *testing.T, repo *repositories.DomainProviderRepository) *models.DomainProvider {
	ctx := context.Background()
	dp := &models.DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Provider",
		Provider:    enums.DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
		Connected:   true,
	}
	err := repo.Create(ctx, dp)
	require.NoError(t, err)
	return dp
}

func createTestDomain(t *testing.T, repo *repositories.DomainRepository, providerID string) *models.Domain {
	ctx := context.Background()
	d := &models.Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: providerID,
		ProviderID:       "ext-123",
		Label:            "Test Domain",
		Address:          "example.com",
	}
	err := repo.Create(ctx, d)
	require.NoError(t, err)
	return d
}

func createTestRecord(t *testing.T, repo *repositories.DnsRecordRepository, domainID string) *models.DnsRecord {
	ctx := context.Background()
	r := &models.DnsRecord{
		DomainID:   domainID,
		ProviderID: "rec-123",
		Type:       enums.RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}
	err := repo.Create(ctx, r)
	require.NoError(t, err)
	return r
}

// DomainProvider Tests

func TestRepository_CreateDomainProvider(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	dp := &models.DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Provider",
		Provider:    enums.DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
		Connected:   true,
	}

	err := providerRepo.Create(ctx, dp)
	require.NoError(t, err)

	assert.NotEmpty(t, dp.ID)
}

func TestRepository_FindDomainProviderByID(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	found, err := providerRepo.FindByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, dp.ID, found.ID)
	assert.Equal(t, dp.Profile, found.Profile)
}

func TestRepository_FindDomainProviderByID_NotFound(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := providerRepo.FindByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindDomainProviderByIDAndTeam(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	found, err := providerRepo.FindByIDAndTeam(ctx, dp.ID, "team123")
	require.NoError(t, err)

	assert.Equal(t, dp.ID, found.ID)
}

func TestRepository_FindDomainProviderByIDAndTeam_WrongTeam(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	_, err := providerRepo.FindByIDAndTeam(ctx, dp.ID, "wrong-team")
	assert.Error(t, err)
}

func TestRepository_FindDomainProvidersByTeam(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	dp1 := createTestProvider(t, providerRepo)
	dp2 := &models.DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Second Provider",
		Provider:    enums.DnsProviderDigitalOcean,
		Credentials: `{"token": "test2"}`,
	}
	providerRepo.Create(ctx, dp2)

	providers, err := providerRepo.FindByTeam(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, providers, 2)
	assert.Equal(t, dp2.ID, providers[0].ID) // Most recent first
	assert.Equal(t, dp1.ID, providers[1].ID)
}

func TestRepository_FindDomainProvidersByTeamWithDomainCount(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	dp := createTestProvider(t, providerRepo)
	createTestDomain(t, domainRepo, dp.ID)
	// Create second domain with different address
	d2 := &models.Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "ext-456",
		Label:            "Test Domain 2",
		Address:          "example2.com",
	}
	domainRepo.Create(ctx, d2)

	providers, counts, err := providerRepo.FindByTeamWithDomainCount(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, providers, 1)
	assert.Equal(t, 2, counts[dp.ID])
}

func TestRepository_UpdateDomainProvider(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	dp.Profile = "Updated Profile"
	dp.Connected = false

	err := providerRepo.Update(ctx, dp)
	require.NoError(t, err)

	found, err := providerRepo.FindByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, "Updated Profile", found.Profile)
	assert.False(t, found.Connected)
}

func TestRepository_UpdateDomainProviderFields(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	err := providerRepo.UpdateFields(ctx, dp.ID, map[string]interface{}{
		"connected":   false,
		"sync_status": enums.SyncStatusFailed,
	})
	require.NoError(t, err)

	found, err := providerRepo.FindByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.False(t, found.Connected)
	assert.Equal(t, enums.SyncStatusFailed, found.SyncStatus)
}

func TestRepository_DeleteDomainProvider(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	err := providerRepo.Delete(ctx, dp.ID)
	require.NoError(t, err)

	_, err = providerRepo.FindByID(ctx, dp.ID)
	assert.Error(t, err)
}

func TestRepository_CountDomainsByProvider(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	dp := createTestProvider(t, providerRepo)
	createTestDomain(t, domainRepo, dp.ID)
	// Create second domain with different address
	d2 := &models.Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "ext-456",
		Label:            "Test Domain 2",
		Address:          "example2.com",
	}
	domainRepo.Create(ctx, d2)

	count, err := providerRepo.CountDomainsByProvider(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

// Domain Tests

func TestRepository_CreateDomain(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	d := &models.Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "ext-123",
		Label:            "Test Domain",
		Address:          "example.com",
	}

	err := domainRepo.Create(ctx, d)
	require.NoError(t, err)

	assert.NotEmpty(t, d.ID)
}

func TestRepository_FindDomainByID(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	found, err := domainRepo.FindByID(ctx, d.ID)
	require.NoError(t, err)

	assert.Equal(t, d.ID, found.ID)
	assert.Equal(t, d.Address, found.Address)
	assert.NotNil(t, found.Provider)
}

func TestRepository_FindDomainByID_NotFound(t *testing.T) {
	_, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := domainRepo.FindByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindDomainByIDAndTeam(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	found, err := domainRepo.FindByIDAndTeam(ctx, d.ID, "team123")
	require.NoError(t, err)

	assert.Equal(t, d.ID, found.ID)
}

func TestRepository_FindDomainByIDAndTeam_WrongTeam(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	_, err := domainRepo.FindByIDAndTeam(ctx, d.ID, "wrong-team")
	assert.Error(t, err)
}

func TestRepository_FindDomainsByTeam(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	createTestDomain(t, domainRepo, dp.ID)

	domains, err := domainRepo.FindByTeam(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, domains, 1)
}

func TestRepository_FindDomainsByProvider(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	createTestDomain(t, domainRepo, dp.ID)
	// Create second domain with different address
	d2 := &models.Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "ext-456",
		Label:            "Test Domain 2",
		Address:          "example2.com",
	}
	domainRepo.Create(ctx, d2)

	domains, err := domainRepo.FindByProvider(ctx, dp.ID)
	require.NoError(t, err)

	assert.Len(t, domains, 2)
}

func TestRepository_FindDomainByAddressAndProvider(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	found, err := domainRepo.FindByAddressAndProvider(ctx, d.Address, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, d.ID, found.ID)
}

func TestRepository_UpdateDomain(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	d.Label = "Updated Label"

	err := domainRepo.Update(ctx, d)
	require.NoError(t, err)

	found, err := domainRepo.FindByID(ctx, d.ID)
	require.NoError(t, err)

	assert.Equal(t, "Updated Label", found.Label)
}

func TestRepository_UpdateOrCreateDomain_Create(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)

	domain, err := domainRepo.UpdateOrCreate(ctx, map[string]interface{}{
		"provider_id":        "new-ext-123",
		"domain_provider_id": dp.ID,
		"address":            "new.example.com",
		"team_id":            "team123",
	}, map[string]interface{}{
		"user_id": "user123",
		"label":   "New Domain",
	})
	require.NoError(t, err)

	assert.NotNil(t, domain)
	assert.Equal(t, "new.example.com", domain.Address)
}

func TestRepository_UpdateOrCreateDomain_Update(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	domain, err := domainRepo.UpdateOrCreate(ctx, map[string]interface{}{
		"provider_id":        d.ProviderID,
		"domain_provider_id": dp.ID,
		"address":            d.Address,
		"team_id":            d.TeamID,
	}, map[string]interface{}{
		"label": "Updated Label",
	})
	require.NoError(t, err)

	assert.Equal(t, d.ID, domain.ID)
}

func TestRepository_DeleteDomain(t *testing.T) {
	providerRepo, domainRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	err := domainRepo.Delete(ctx, d.ID)
	require.NoError(t, err)

	_, err = domainRepo.FindByID(ctx, d.ID)
	assert.Error(t, err)
}

// DnsRecord Tests

func TestRepository_CreateDnsRecord(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	r := &models.DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-123",
		Type:       enums.RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}

	err := dnsRecordRepo.Create(ctx, r)
	require.NoError(t, err)

	assert.NotEmpty(t, r.ID)
}

func TestRepository_FindDnsRecordByID(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	r := createTestRecord(t, dnsRecordRepo, d.ID)

	found, err := dnsRecordRepo.FindByID(ctx, r.ID)
	require.NoError(t, err)

	assert.Equal(t, r.ID, found.ID)
	assert.Equal(t, r.Value, found.Value)
}

func TestRepository_FindDnsRecordByID_NotFound(t *testing.T) {
	_, _, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := dnsRecordRepo.FindByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindDnsRecordByIDAndDomain(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	r := createTestRecord(t, dnsRecordRepo, d.ID)

	found, err := dnsRecordRepo.FindByIDAndDomain(ctx, r.ID, d.ID)
	require.NoError(t, err)

	assert.Equal(t, r.ID, found.ID)
}

func TestRepository_FindDnsRecordByIDAndDomain_WrongDomain(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	r := createTestRecord(t, dnsRecordRepo, d.ID)

	_, err := dnsRecordRepo.FindByIDAndDomain(ctx, r.ID, "wrong-domain")
	assert.Error(t, err)
}

func TestRepository_FindDnsRecordsByDomain(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	createTestRecord(t, dnsRecordRepo, d.ID)

	r2 := &models.DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-456",
		Type:       enums.RecordTypeCNAME,
		Name:       "www",
		Value:      "example.com",
		TTL:        3600,
	}
	dnsRecordRepo.Create(ctx, r2)

	records, err := dnsRecordRepo.FindByDomain(ctx, d.ID)
	require.NoError(t, err)

	assert.Len(t, records, 2)
}

func TestRepository_FindDnsRecordsByType(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	createTestRecord(t, dnsRecordRepo, d.ID)

	r2 := &models.DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-456",
		Type:       enums.RecordTypeCNAME,
		Name:       "www",
		Value:      "example.com",
		TTL:        3600,
	}
	dnsRecordRepo.Create(ctx, r2)

	records, err := dnsRecordRepo.FindByType(ctx, d.ID, enums.RecordTypeA)
	require.NoError(t, err)

	assert.Len(t, records, 1)
	assert.Equal(t, enums.RecordTypeA, records[0].Type)
}

func TestRepository_UpdateDnsRecord(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	r := createTestRecord(t, dnsRecordRepo, d.ID)

	r.Value = "5.6.7.8"
	r.TTL = 7200

	err := dnsRecordRepo.Update(ctx, r)
	require.NoError(t, err)

	found, err := dnsRecordRepo.FindByID(ctx, r.ID)
	require.NoError(t, err)

	assert.Equal(t, "5.6.7.8", found.Value)
	assert.Equal(t, 7200, found.TTL)
}

func TestRepository_UpdateOrCreateDnsRecord_Create(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)

	record, err := dnsRecordRepo.UpdateOrCreate(ctx, map[string]interface{}{
		"domain_id":   d.ID,
		"type":        enums.RecordTypeA,
		"name":        "new",
		"provider_id": "new-rec-123",
	}, map[string]interface{}{
		"value": "1.2.3.4",
		"ttl":   3600,
	})
	require.NoError(t, err)

	assert.NotNil(t, record)
}

func TestRepository_UpdateOrCreateDnsRecord_Update(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	r := createTestRecord(t, dnsRecordRepo, d.ID)

	record, err := dnsRecordRepo.UpdateOrCreate(ctx, map[string]interface{}{
		"domain_id":   d.ID,
		"type":        r.Type,
		"name":        r.Name,
		"provider_id": r.ProviderID,
	}, map[string]interface{}{
		"value": "5.6.7.8",
		"ttl":   7200,
	})
	require.NoError(t, err)

	assert.Equal(t, r.ID, record.ID)
}

func TestRepository_DeleteDnsRecord(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	r := createTestRecord(t, dnsRecordRepo, d.ID)

	err := dnsRecordRepo.Delete(ctx, r.ID)
	require.NoError(t, err)

	_, err = dnsRecordRepo.FindByID(ctx, r.ID)
	assert.Error(t, err)
}

func TestRepository_DeleteDnsRecordsByDomain(t *testing.T) {
	providerRepo, domainRepo, dnsRecordRepo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, providerRepo)
	d := createTestDomain(t, domainRepo, dp.ID)
	createTestRecord(t, dnsRecordRepo, d.ID)
	// Create second record
	r2 := &models.DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-456",
		Type:       enums.RecordTypeCNAME,
		Name:       "www",
		Value:      "example.com",
		TTL:        3600,
	}
	dnsRecordRepo.Create(ctx, r2)

	err := dnsRecordRepo.DeleteByDomain(ctx, d.ID)
	require.NoError(t, err)

	records, err := dnsRecordRepo.FindByDomain(ctx, d.ID)
	require.NoError(t, err)

	assert.Len(t, records, 0)
}

// Transaction Tests

func TestRepository_BeginTransaction(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	tx := providerRepo.BeginTransaction(ctx)
	assert.NotNil(t, tx)

	tx.Rollback()
}

func TestRepository_WithTransaction_Success(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	err := providerRepo.WithTransaction(ctx, func(tx *gorm.DB) error {
		dp := &models.DomainProvider{
			UserID:      "user123",
			TeamID:      "team123",
			Profile:     "Test",
			Provider:    enums.DnsProviderCloudflare,
			Credentials: `{"token": "test"}`,
		}
		return tx.Create(dp).Error
	})
	require.NoError(t, err)

	providers, err := providerRepo.FindByTeam(ctx, "team123")
	require.NoError(t, err)
	assert.Len(t, providers, 1)
}

func TestRepository_WithTransaction_Rollback(t *testing.T) {
	providerRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	err := providerRepo.WithTransaction(ctx, func(tx *gorm.DB) error {
		dp := &models.DomainProvider{
			UserID:      "user123",
			TeamID:      "team123",
			Profile:     "Test",
			Provider:    enums.DnsProviderCloudflare,
			Credentials: `{"token": "test"}`,
		}
		if err := tx.Create(dp).Error; err != nil {
			return err
		}
		return assert.AnError
	})
	assert.Error(t, err)

	providers, err := providerRepo.FindByTeam(ctx, "team123")
	require.NoError(t, err)
	assert.Len(t, providers, 0)
}
