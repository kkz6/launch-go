package dns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&DomainProvider{}, &Domain{}, &DnsRecord{})
	require.NoError(t, err)

	return NewRepository(db), db
}

func createTestProvider(t *testing.T, repo *Repository) *DomainProvider {
	ctx := context.Background()
	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Provider",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
		Connected:   true,
	}
	err := repo.CreateDomainProvider(ctx, dp)
	require.NoError(t, err)
	return dp
}

func createTestDomain(t *testing.T, repo *Repository, providerID string) *Domain {
	ctx := context.Background()
	d := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: providerID,
		ProviderID:       "ext-123",
		Label:            "Test Domain",
		Address:          "example.com",
	}
	err := repo.CreateDomain(ctx, d)
	require.NoError(t, err)
	return d
}

func createTestRecord(t *testing.T, repo *Repository, domainID string) *DnsRecord {
	ctx := context.Background()
	r := &DnsRecord{
		DomainID:   domainID,
		ProviderID: "rec-123",
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}
	err := repo.CreateDnsRecord(ctx, r)
	require.NoError(t, err)
	return r
}

// DomainProvider Tests

func TestRepository_CreateDomainProvider(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Provider",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
		Connected:   true,
	}

	err := repo.CreateDomainProvider(ctx, dp)
	require.NoError(t, err)

	assert.NotEmpty(t, dp.ID)
}

func TestRepository_FindDomainProviderByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	found, err := repo.FindDomainProviderByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, dp.ID, found.ID)
	assert.Equal(t, dp.Profile, found.Profile)
}

func TestRepository_FindDomainProviderByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindDomainProviderByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindDomainProviderByIDAndTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	found, err := repo.FindDomainProviderByIDAndTeam(ctx, dp.ID, "team123")
	require.NoError(t, err)

	assert.Equal(t, dp.ID, found.ID)
}

func TestRepository_FindDomainProviderByIDAndTeam_WrongTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	_, err := repo.FindDomainProviderByIDAndTeam(ctx, dp.ID, "wrong-team")
	assert.Error(t, err)
}

func TestRepository_FindDomainProvidersByTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	dp1 := createTestProvider(t, repo)
	dp2 := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Second Provider",
		Provider:    DnsProviderDigitalOcean,
		Credentials: `{"token": "test2"}`,
	}
	repo.CreateDomainProvider(ctx, dp2)

	providers, err := repo.FindDomainProvidersByTeam(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, providers, 2)
	assert.Equal(t, dp2.ID, providers[0].ID) // Most recent first
	assert.Equal(t, dp1.ID, providers[1].ID)
}

func TestRepository_FindDomainProvidersByTeamWithDomainCount(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	dp := createTestProvider(t, repo)
	createTestDomain(t, repo, dp.ID)
	createTestDomain(t, repo, dp.ID)

	providers, counts, err := repo.FindDomainProvidersByTeamWithDomainCount(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, providers, 1)
	assert.Equal(t, 2, counts[dp.ID])
}

func TestRepository_UpdateDomainProvider(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	dp.Profile = "Updated Profile"
	dp.Connected = false

	err := repo.UpdateDomainProvider(ctx, dp)
	require.NoError(t, err)

	found, err := repo.FindDomainProviderByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, "Updated Profile", found.Profile)
	assert.False(t, found.Connected)
}

func TestRepository_UpdateDomainProviderFields(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	err := repo.UpdateDomainProviderFields(ctx, dp.ID, map[string]interface{}{
		"connected":   false,
		"sync_status": SyncStatusFailed,
	})
	require.NoError(t, err)

	found, err := repo.FindDomainProviderByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.False(t, found.Connected)
	assert.Equal(t, SyncStatusFailed, found.SyncStatus)
}

func TestRepository_DeleteDomainProvider(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	err := repo.DeleteDomainProvider(ctx, dp.ID)
	require.NoError(t, err)

	_, err = repo.FindDomainProviderByID(ctx, dp.ID)
	assert.Error(t, err)
}

func TestRepository_CountDomainsByProvider(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	dp := createTestProvider(t, repo)
	createTestDomain(t, repo, dp.ID)
	createTestDomain(t, repo, dp.ID)

	count, err := repo.CountDomainsByProvider(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

// Domain Tests

func TestRepository_CreateDomain(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	d := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "ext-123",
		Label:            "Test Domain",
		Address:          "example.com",
	}

	err := repo.CreateDomain(ctx, d)
	require.NoError(t, err)

	assert.NotEmpty(t, d.ID)
}

func TestRepository_FindDomainByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	found, err := repo.FindDomainByID(ctx, d.ID)
	require.NoError(t, err)

	assert.Equal(t, d.ID, found.ID)
	assert.Equal(t, d.Address, found.Address)
	assert.NotNil(t, found.Provider)
}

func TestRepository_FindDomainByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindDomainByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindDomainByIDAndTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	found, err := repo.FindDomainByIDAndTeam(ctx, d.ID, "team123")
	require.NoError(t, err)

	assert.Equal(t, d.ID, found.ID)
}

func TestRepository_FindDomainByIDAndTeam_WrongTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	_, err := repo.FindDomainByIDAndTeam(ctx, d.ID, "wrong-team")
	assert.Error(t, err)
}

func TestRepository_FindDomainsByTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	createTestDomain(t, repo, dp.ID)

	domains, err := repo.FindDomainsByTeam(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, domains, 1)
}

func TestRepository_FindDomainsByProvider(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	createTestDomain(t, repo, dp.ID)
	createTestDomain(t, repo, dp.ID)

	domains, err := repo.FindDomainsByProvider(ctx, dp.ID)
	require.NoError(t, err)

	assert.Len(t, domains, 2)
}

func TestRepository_FindDomainByAddressAndProvider(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	found, err := repo.FindDomainByAddressAndProvider(ctx, d.Address, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, d.ID, found.ID)
}

func TestRepository_UpdateDomain(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	d.Label = "Updated Label"

	err := repo.UpdateDomain(ctx, d)
	require.NoError(t, err)

	found, err := repo.FindDomainByID(ctx, d.ID)
	require.NoError(t, err)

	assert.Equal(t, "Updated Label", found.Label)
}

func TestRepository_UpdateOrCreateDomain_Create(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)

	domain, err := repo.UpdateOrCreateDomain(ctx, map[string]interface{}{
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
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	domain, err := repo.UpdateOrCreateDomain(ctx, map[string]interface{}{
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
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	err := repo.DeleteDomain(ctx, d.ID)
	require.NoError(t, err)

	_, err = repo.FindDomainByID(ctx, d.ID)
	assert.Error(t, err)
}

// DnsRecord Tests

func TestRepository_CreateDnsRecord(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	r := &DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-123",
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}

	err := repo.CreateDnsRecord(ctx, r)
	require.NoError(t, err)

	assert.NotEmpty(t, r.ID)
}

func TestRepository_FindDnsRecordByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	r := createTestRecord(t, repo, d.ID)

	found, err := repo.FindDnsRecordByID(ctx, r.ID)
	require.NoError(t, err)

	assert.Equal(t, r.ID, found.ID)
	assert.Equal(t, r.Value, found.Value)
}

func TestRepository_FindDnsRecordByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindDnsRecordByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRepository_FindDnsRecordByIDAndDomain(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	r := createTestRecord(t, repo, d.ID)

	found, err := repo.FindDnsRecordByIDAndDomain(ctx, r.ID, d.ID)
	require.NoError(t, err)

	assert.Equal(t, r.ID, found.ID)
}

func TestRepository_FindDnsRecordByIDAndDomain_WrongDomain(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	r := createTestRecord(t, repo, d.ID)

	_, err := repo.FindDnsRecordByIDAndDomain(ctx, r.ID, "wrong-domain")
	assert.Error(t, err)
}

func TestRepository_FindDnsRecordsByDomain(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	createTestRecord(t, repo, d.ID)

	r2 := &DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-456",
		Type:       RecordTypeCNAME,
		Name:       "www",
		Value:      "example.com",
		TTL:        3600,
	}
	repo.CreateDnsRecord(ctx, r2)

	records, err := repo.FindDnsRecordsByDomain(ctx, d.ID)
	require.NoError(t, err)

	assert.Len(t, records, 2)
}

func TestRepository_FindDnsRecordsByType(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	createTestRecord(t, repo, d.ID)

	r2 := &DnsRecord{
		DomainID:   d.ID,
		ProviderID: "rec-456",
		Type:       RecordTypeCNAME,
		Name:       "www",
		Value:      "example.com",
		TTL:        3600,
	}
	repo.CreateDnsRecord(ctx, r2)

	records, err := repo.FindDnsRecordsByType(ctx, d.ID, RecordTypeA)
	require.NoError(t, err)

	assert.Len(t, records, 1)
	assert.Equal(t, RecordTypeA, records[0].Type)
}

func TestRepository_UpdateDnsRecord(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	r := createTestRecord(t, repo, d.ID)

	r.Value = "5.6.7.8"
	r.TTL = 7200

	err := repo.UpdateDnsRecord(ctx, r)
	require.NoError(t, err)

	found, err := repo.FindDnsRecordByID(ctx, r.ID)
	require.NoError(t, err)

	assert.Equal(t, "5.6.7.8", found.Value)
	assert.Equal(t, 7200, found.TTL)
}

func TestRepository_UpdateOrCreateDnsRecord_Create(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)

	record, err := repo.UpdateOrCreateDnsRecord(ctx, map[string]interface{}{
		"domain_id":   d.ID,
		"type":        RecordTypeA,
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
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	r := createTestRecord(t, repo, d.ID)

	record, err := repo.UpdateOrCreateDnsRecord(ctx, map[string]interface{}{
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
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	r := createTestRecord(t, repo, d.ID)

	err := repo.DeleteDnsRecord(ctx, r.ID)
	require.NoError(t, err)

	_, err = repo.FindDnsRecordByID(ctx, r.ID)
	assert.Error(t, err)
}

func TestRepository_DeleteDnsRecordsByDomain(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()
	dp := createTestProvider(t, repo)
	d := createTestDomain(t, repo, dp.ID)
	createTestRecord(t, repo, d.ID)
	createTestRecord(t, repo, d.ID)

	err := repo.DeleteDnsRecordsByDomain(ctx, d.ID)
	require.NoError(t, err)

	records, err := repo.FindDnsRecordsByDomain(ctx, d.ID)
	require.NoError(t, err)

	assert.Len(t, records, 0)
}

// Transaction Tests

func TestRepository_BeginTransaction(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	tx := repo.BeginTransaction(ctx)
	assert.NotNil(t, tx)

	tx.Rollback()
}

func TestRepository_WithTransaction_Success(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	err := repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		dp := &DomainProvider{
			UserID:      "user123",
			TeamID:      "team123",
			Profile:     "Test",
			Provider:    DnsProviderCloudflare,
			Credentials: `{"token": "test"}`,
		}
		return tx.Create(dp).Error
	})
	require.NoError(t, err)

	providers, err := repo.FindDomainProvidersByTeam(ctx, "team123")
	require.NoError(t, err)
	assert.Len(t, providers, 1)
}

func TestRepository_WithTransaction_Rollback(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	err := repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		dp := &DomainProvider{
			UserID:      "user123",
			TeamID:      "team123",
			Profile:     "Test",
			Provider:    DnsProviderCloudflare,
			Credentials: `{"token": "test"}`,
		}
		if err := tx.Create(dp).Error; err != nil {
			return err
		}
		return assert.AnError
	})
	assert.Error(t, err)

	providers, err := repo.FindDomainProvidersByTeam(ctx, "team123")
	require.NoError(t, err)
	assert.Len(t, providers, 0)
}
