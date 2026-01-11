package dns

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestService(t *testing.T) (*Service, *Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&DomainProvider{}, &Domain{}, &DnsRecord{})
	require.NoError(t, err)

	repo := NewRepository(db)
	logger := zerolog.Nop()
	service := NewService(repo, &logger)

	return service, repo, db
}

func createTestServiceProvider(t *testing.T, repo *Repository, teamID string) *DomainProvider {
	ctx := context.Background()
	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      teamID,
		Profile:     "Test Provider",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test-token"}`,
		Connected:   true,
	}
	err := repo.CreateDomainProvider(ctx, dp)
	require.NoError(t, err)
	return dp
}

func createTestServiceDomain(t *testing.T, repo *Repository, providerID, teamID string) *Domain {
	ctx := context.Background()
	d := &Domain{
		UserID:           "user123",
		TeamID:           teamID,
		DomainProviderID: providerID,
		ProviderID:       "ext-123",
		Label:            "Test Domain",
		Address:          "example.com",
	}
	err := repo.CreateDomain(ctx, d)
	require.NoError(t, err)
	return d
}

func createTestServiceRecord(t *testing.T, repo *Repository, domainID string) *DnsRecord {
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

// Service Tests

func TestNewService(t *testing.T) {
	service, _, _ := setupTestService(t)
	assert.NotNil(t, service)
}

// Provider Service Tests

func TestService_ListProviders(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	createTestServiceDomain(t, repo, dp.ID, "team123")

	providers, err := service.ListProviders(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, providers, 1)
	assert.Equal(t, dp.ID, providers[0].ID)
	assert.Equal(t, 1, providers[0].DomainsCount)
}

func TestService_ListProviders_Empty(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	providers, err := service.ListProviders(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, providers, 0)
}

func TestService_GetProvider(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")

	provider, err := service.GetProvider(ctx, dp.ID, "team123")
	require.NoError(t, err)

	assert.Equal(t, dp.ID, provider.ID)
	assert.Equal(t, dp.Profile, provider.Profile)
}

func TestService_GetProvider_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.GetProvider(ctx, "nonexistent", "team123")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestService_GetProvider_WrongTeam(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")

	_, err := service.GetProvider(ctx, dp.ID, "wrong-team")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestService_DeleteProvider(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")

	err := service.DeleteProvider(ctx, dp.ID, "team123")
	require.NoError(t, err)

	_, err = service.GetProvider(ctx, dp.ID, "team123")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestService_DeleteProvider_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.DeleteProvider(ctx, "nonexistent", "team123")
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestService_DeleteProvider_HasDomains(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	createTestServiceDomain(t, repo, dp.ID, "team123")

	err := service.DeleteProvider(ctx, dp.ID, "team123")
	assert.ErrorIs(t, err, ErrProviderHasActiveDomains)
}

// Domain Service Tests

func TestService_GetDomain(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")

	domain, err := service.GetDomain(ctx, d.ID, "team123")
	require.NoError(t, err)

	assert.Equal(t, d.ID, domain.ID)
	assert.Equal(t, d.Address, domain.Address)
}

func TestService_GetDomain_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.GetDomain(ctx, "nonexistent", "team123")
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

func TestService_GetDomain_WrongTeam(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")

	_, err := service.GetDomain(ctx, d.ID, "wrong-team")
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

func TestService_ListDomains(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	createTestServiceDomain(t, repo, dp.ID, "team123")

	domains, err := service.ListDomains(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, domains, 1)
}

func TestService_ListDomains_Empty(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	domains, err := service.ListDomains(ctx, "team123")
	require.NoError(t, err)

	assert.Len(t, domains, 0)
}

func TestService_GetDomainRecords(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")
	createTestServiceRecord(t, repo, d.ID)

	records, err := service.GetDomainRecords(ctx, d.ID, "team123")
	require.NoError(t, err)

	assert.Len(t, records, 1)
	assert.Equal(t, "A", records[0].Type)
}

func TestService_GetDomainRecords_DomainNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.GetDomainRecords(ctx, "nonexistent", "team123")
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

// Record Types Tests

func TestService_GetRecordTypes(t *testing.T) {
	service, _, _ := setupTestService(t)

	types := service.GetRecordTypes()

	assert.Len(t, types, 9)
	assert.Contains(t, types, "A")
	assert.Contains(t, types, "AAAA")
	assert.Contains(t, types, "CNAME")
	assert.Contains(t, types, "MX")
	assert.Contains(t, types, "NS")
	assert.Contains(t, types, "SRV")
	assert.Contains(t, types, "TXT")
	assert.Contains(t, types, "SOA")
	assert.Contains(t, types, "CAA")
}

// Error Constants Tests

func TestErrProviderNotFound(t *testing.T) {
	assert.EqualError(t, ErrProviderNotFound, "provider not found")
}

func TestErrDomainNotFound(t *testing.T) {
	assert.EqualError(t, ErrDomainNotFound, "domain not found")
}

func TestErrRecordNotFound(t *testing.T) {
	assert.EqualError(t, ErrRecordNotFound, "record not found")
}

func TestErrRecordNotEditable(t *testing.T) {
	assert.EqualError(t, ErrRecordNotEditable, "record cannot be edited")
}

func TestErrRecordNotDeletable(t *testing.T) {
	assert.EqualError(t, ErrRecordNotDeletable, "record cannot be deleted")
}

func TestErrProviderHasActiveDomains(t *testing.T) {
	assert.EqualError(t, ErrProviderHasActiveDomains, "provider has active domains")
}

func TestErrInvalidCredentials(t *testing.T) {
	assert.EqualError(t, ErrInvalidCredentials, "invalid credentials")
}

// Service Method Edge Cases

func TestService_DeleteDomain_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.DeleteDomain(ctx, "nonexistent", "team123", false)
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

func TestService_DeleteDomain_DeletesRecords(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")
	createTestServiceRecord(t, repo, d.ID)

	err := service.DeleteDomain(ctx, d.ID, "team123", false)
	require.NoError(t, err)

	records, err := repo.FindDnsRecordsByDomain(ctx, d.ID)
	require.NoError(t, err)
	assert.Len(t, records, 0)
}

func TestService_CreateRecord_DomainNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &CreateDnsRecordRequest{
		Name:  "@",
		Value: "1.2.3.4",
		Type:  "A",
		TTL:   3600,
	}

	_, err := service.CreateRecord(ctx, "nonexistent", "team123", req)
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

func TestService_UpdateRecord_DomainNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	req := &UpdateDnsRecordRequest{
		Name:  "@",
		Value: "1.2.3.4",
		Type:  "A",
		TTL:   3600,
	}

	_, err := service.UpdateRecord(ctx, "record123", "nonexistent", "team123", req)
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

func TestService_UpdateRecord_RecordNotFound(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")

	req := &UpdateDnsRecordRequest{
		Name:  "@",
		Value: "1.2.3.4",
		Type:  "A",
		TTL:   3600,
	}

	_, err := service.UpdateRecord(ctx, "nonexistent", d.ID, "team123", req)
	assert.ErrorIs(t, err, ErrRecordNotFound)
}

func TestService_UpdateRecord_NSRecordNotEditable(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")

	// Create NS record
	nsRecord := &DnsRecord{
		DomainID:   d.ID,
		ProviderID: "ns-rec-123",
		Type:       RecordTypeNS,
		Name:       "@",
		Value:      "ns1.example.com",
		TTL:        86400,
	}
	repo.CreateDnsRecord(ctx, nsRecord)

	req := &UpdateDnsRecordRequest{
		Name:  "@",
		Value: "ns2.example.com",
		Type:  "NS",
		TTL:   86400,
	}

	_, err := service.UpdateRecord(ctx, nsRecord.ID, d.ID, "team123", req)
	assert.ErrorIs(t, err, ErrRecordNotEditable)
}

func TestService_DeleteRecord_DomainNotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.DeleteRecord(ctx, "record123", "nonexistent", "team123")
	assert.ErrorIs(t, err, ErrDomainNotFound)
}

func TestService_DeleteRecord_RecordNotFound(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")

	err := service.DeleteRecord(ctx, "nonexistent", d.ID, "team123")
	assert.ErrorIs(t, err, ErrRecordNotFound)
}

func TestService_DeleteRecord_NSRecordNotDeletable(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")
	d := createTestServiceDomain(t, repo, dp.ID, "team123")

	// Create NS record
	nsRecord := &DnsRecord{
		DomainID:   d.ID,
		ProviderID: "ns-rec-123",
		Type:       RecordTypeNS,
		Name:       "@",
		Value:      "ns1.example.com",
		TTL:        86400,
	}
	repo.CreateDnsRecord(ctx, nsRecord)

	err := service.DeleteRecord(ctx, nsRecord.ID, d.ID, "team123")
	assert.ErrorIs(t, err, ErrRecordNotDeletable)
}

func TestService_MarkSyncFailed(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()

	dp := createTestServiceProvider(t, repo, "team123")

	service.markSyncFailed(ctx, dp.ID, "Test error message")

	updated, err := repo.FindDomainProviderByID(ctx, dp.ID)
	require.NoError(t, err)

	assert.Equal(t, SyncStatusFailed, updated.SyncStatus)
	assert.Equal(t, "Test error message", *updated.SyncErrorMessage)
}
