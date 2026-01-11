package dns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&DomainProvider{}, &Domain{}, &DnsRecord{})
	require.NoError(t, err)

	return db
}

func TestDomainProvider_TableName(t *testing.T) {
	dp := DomainProvider{}
	assert.Equal(t, "domain_providers", dp.TableName())
}

func TestDomainProvider_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Profile",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
		Connected:   true,
	}

	err := db.Create(dp).Error
	require.NoError(t, err)

	assert.NotEmpty(t, dp.ID)
	assert.Len(t, dp.ID, 26) // ULID length
}

func TestDomainProvider_BeforeCreate_WithExistingID(t *testing.T) {
	db := setupTestDB(t)

	existingID := "01HQXYZ123456789ABCDEFGH"
	dp := &DomainProvider{
		ID:          existingID,
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Profile",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
		Connected:   true,
	}

	err := db.Create(dp).Error
	require.NoError(t, err)

	assert.Equal(t, existingID, dp.ID)
}

func TestDomainProvider_GetCredentials(t *testing.T) {
	dp := &DomainProvider{
		Credentials: `{"token": "test-token", "key": "test-key"}`,
	}

	creds, err := dp.GetCredentials()
	require.NoError(t, err)

	assert.Equal(t, "test-token", creds["token"])
	assert.Equal(t, "test-key", creds["key"])
}

func TestDomainProvider_GetCredentials_Invalid(t *testing.T) {
	dp := &DomainProvider{
		Credentials: "invalid json",
	}

	_, err := dp.GetCredentials()
	assert.Error(t, err)
}

func TestDomainProvider_SetCredentials(t *testing.T) {
	dp := &DomainProvider{}

	err := dp.SetCredentials(map[string]string{
		"token": "new-token",
		"key":   "new-key",
	})
	require.NoError(t, err)

	creds, err := dp.GetCredentials()
	require.NoError(t, err)

	assert.Equal(t, "new-token", creds["token"])
	assert.Equal(t, "new-key", creds["key"])
}

func TestDomainProvider_GetAdditionalData(t *testing.T) {
	data := `{"account_id": "abc123", "count": 42}`
	dp := &DomainProvider{
		AdditionalData: &data,
	}

	result, err := dp.GetAdditionalData()
	require.NoError(t, err)

	assert.Equal(t, "abc123", result["account_id"])
	assert.Equal(t, float64(42), result["count"])
}

func TestDomainProvider_GetAdditionalData_Nil(t *testing.T) {
	dp := &DomainProvider{
		AdditionalData: nil,
	}

	result, err := dp.GetAdditionalData()
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDomainProvider_GetAdditionalData_Invalid(t *testing.T) {
	data := "invalid json"
	dp := &DomainProvider{
		AdditionalData: &data,
	}

	_, err := dp.GetAdditionalData()
	assert.Error(t, err)
}

func TestDomainProvider_SetAdditionalData(t *testing.T) {
	dp := &DomainProvider{}

	err := dp.SetAdditionalData(map[string]interface{}{
		"account_id": "abc123",
		"count":      42,
	})
	require.NoError(t, err)

	result, err := dp.GetAdditionalData()
	require.NoError(t, err)

	assert.Equal(t, "abc123", result["account_id"])
	assert.Equal(t, float64(42), result["count"])
}

func TestDomainProvider_SetAdditionalData_Nil(t *testing.T) {
	data := `{"test": "data"}`
	dp := &DomainProvider{
		AdditionalData: &data,
	}

	err := dp.SetAdditionalData(nil)
	require.NoError(t, err)
	assert.Nil(t, dp.AdditionalData)
}

func TestDomainProvider_ProviderLabel(t *testing.T) {
	tests := []struct {
		provider DnsProvider
		expected string
	}{
		{DnsProviderCloudflare, "Cloudflare"},
		{DnsProviderDigitalOcean, "DigitalOcean"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			dp := &DomainProvider{Provider: tt.provider}
			assert.Equal(t, tt.expected, dp.ProviderLabel())
		})
	}
}

func TestDomain_TableName(t *testing.T) {
	d := Domain{}
	assert.Equal(t, "domains", d.TableName())
}

func TestDomain_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// Create provider first
	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	db.Create(dp)

	d := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test Domain",
		Address:          "example.com",
	}

	err := db.Create(d).Error
	require.NoError(t, err)

	assert.NotEmpty(t, d.ID)
	assert.Len(t, d.ID, 26)
}

func TestDomain_BeforeCreate_WithExistingID(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	db.Create(dp)

	existingID := "01HQXYZ123456789ABCDEFGH"
	d := &Domain{
		ID:               existingID,
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test Domain",
		Address:          "example.com",
	}

	err := db.Create(d).Error
	require.NoError(t, err)

	assert.Equal(t, existingID, d.ID)
}

func TestDomain_GetAdditionalData(t *testing.T) {
	data := `{"key": "value"}`
	d := &Domain{
		AdditionalData: &data,
	}

	result, err := d.GetAdditionalData()
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestDomain_GetAdditionalData_Nil(t *testing.T) {
	d := &Domain{
		AdditionalData: nil,
	}

	result, err := d.GetAdditionalData()
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDomain_SetAdditionalData(t *testing.T) {
	d := &Domain{}

	err := d.SetAdditionalData(map[string]interface{}{"key": "value"})
	require.NoError(t, err)

	result, err := d.GetAdditionalData()
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestDomain_SetAdditionalData_Nil(t *testing.T) {
	data := `{"test": "data"}`
	d := &Domain{
		AdditionalData: &data,
	}

	err := d.SetAdditionalData(nil)
	require.NoError(t, err)
	assert.Nil(t, d.AdditionalData)
}

func TestDnsRecord_TableName(t *testing.T) {
	r := DnsRecord{}
	assert.Equal(t, "dns_records", r.TableName())
}

func TestDnsRecord_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	db.Create(dp)

	d := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test",
		Address:          "example.com",
	}
	db.Create(d)

	r := &DnsRecord{
		DomainID:   d.ID,
		ProviderID: "record123",
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}

	err := db.Create(r).Error
	require.NoError(t, err)

	assert.NotEmpty(t, r.ID)
	assert.Len(t, r.ID, 26)
}

func TestDnsRecord_BeforeCreate_WithExistingID(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	db.Create(dp)

	d := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test",
		Address:          "example.com",
	}
	db.Create(d)

	existingID := "01HQXYZ123456789ABCDEFGH"
	r := &DnsRecord{
		ID:         existingID,
		DomainID:   d.ID,
		ProviderID: "record123",
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}

	err := db.Create(r).Error
	require.NoError(t, err)

	assert.Equal(t, existingID, r.ID)
}

func TestDnsRecord_IsEditable(t *testing.T) {
	tests := []struct {
		recordType RecordType
		editable   bool
	}{
		{RecordTypeA, true},
		{RecordTypeAAAA, true},
		{RecordTypeCNAME, true},
		{RecordTypeMX, true},
		{RecordTypeTXT, true},
		{RecordTypeSRV, true},
		{RecordTypeCAA, true},
		{RecordTypeNS, false},
		{RecordTypeSOA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.recordType), func(t *testing.T) {
			r := &DnsRecord{Type: tt.recordType}
			assert.Equal(t, tt.editable, r.IsEditable())
		})
	}
}

func TestDnsRecord_IsDeletable(t *testing.T) {
	tests := []struct {
		recordType RecordType
		deletable  bool
	}{
		{RecordTypeA, true},
		{RecordTypeAAAA, true},
		{RecordTypeCNAME, true},
		{RecordTypeMX, true},
		{RecordTypeTXT, true},
		{RecordTypeSRV, true},
		{RecordTypeCAA, true},
		{RecordTypeNS, false},
		{RecordTypeSOA, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.recordType), func(t *testing.T) {
			r := &DnsRecord{Type: tt.recordType}
			assert.Equal(t, tt.deletable, r.IsDeletable())
		})
	}
}

func TestProviderRecord(t *testing.T) {
	priority := 10
	tag := "issue"
	weight := 5
	port := 443
	flags := 0
	comment := "Test comment"
	proxied := true

	pr := ProviderRecord{
		ID:       "record123",
		Type:     RecordTypeMX,
		Name:     "mail",
		Value:    "mail.example.com",
		TTL:      3600,
		Priority: &priority,
		Tag:      &tag,
		Weight:   &weight,
		Port:     &port,
		Flags:    &flags,
		Comment:  &comment,
		Proxied:  &proxied,
	}

	assert.Equal(t, "record123", pr.ID)
	assert.Equal(t, RecordTypeMX, pr.Type)
	assert.Equal(t, "mail", pr.Name)
	assert.Equal(t, "mail.example.com", pr.Value)
	assert.Equal(t, 3600, pr.TTL)
	assert.Equal(t, 10, *pr.Priority)
	assert.Equal(t, "issue", *pr.Tag)
	assert.Equal(t, 5, *pr.Weight)
	assert.Equal(t, 443, *pr.Port)
	assert.Equal(t, 0, *pr.Flags)
	assert.Equal(t, "Test comment", *pr.Comment)
	assert.True(t, *pr.Proxied)
}

func TestDomainProvider_Relations(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	err := db.Create(dp).Error
	require.NoError(t, err)

	domain := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test",
		Address:          "example.com",
	}
	err = db.Create(domain).Error
	require.NoError(t, err)

	// Load provider with domains
	var loadedProvider DomainProvider
	err = db.Preload("Domains").First(&loadedProvider, "id = ?", dp.ID).Error
	require.NoError(t, err)

	assert.Len(t, loadedProvider.Domains, 1)
	assert.Equal(t, domain.ID, loadedProvider.Domains[0].ID)
}

func TestDomain_Relations(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	db.Create(dp)

	domain := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test",
		Address:          "example.com",
	}
	db.Create(domain)

	record := &DnsRecord{
		DomainID:   domain.ID,
		ProviderID: "record123",
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}
	db.Create(record)

	// Load domain with relations
	var loadedDomain Domain
	err := db.Preload("Provider").Preload("Records").First(&loadedDomain, "id = ?", domain.ID).Error
	require.NoError(t, err)

	assert.NotNil(t, loadedDomain.Provider)
	assert.Equal(t, dp.ID, loadedDomain.Provider.ID)
	assert.Len(t, loadedDomain.Records, 1)
	assert.Equal(t, record.ID, loadedDomain.Records[0].ID)
}

func TestDnsRecord_Relations(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}
	db.Create(dp)

	domain := &Domain{
		UserID:           "user123",
		TeamID:           "team123",
		DomainProviderID: dp.ID,
		ProviderID:       "provider123",
		Label:            "Test",
		Address:          "example.com",
	}
	db.Create(domain)

	record := &DnsRecord{
		DomainID:   domain.ID,
		ProviderID: "record123",
		Type:       RecordTypeA,
		Name:       "@",
		Value:      "1.2.3.4",
		TTL:        3600,
	}
	db.Create(record)

	// Load record with domain
	var loadedRecord DnsRecord
	err := db.Preload("Domain").First(&loadedRecord, "id = ?", record.ID).Error
	require.NoError(t, err)

	assert.NotNil(t, loadedRecord.Domain)
	assert.Equal(t, domain.ID, loadedRecord.Domain.ID)
}

func TestDomainProvider_Timestamps(t *testing.T) {
	db := setupTestDB(t)

	dp := &DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test",
		Provider:    DnsProviderCloudflare,
		Credentials: `{"token": "test"}`,
	}

	err := db.Create(dp).Error
	require.NoError(t, err)

	assert.False(t, dp.CreatedAt.IsZero())
	assert.False(t, dp.UpdatedAt.IsZero())

	time.Sleep(time.Millisecond * 10)

	dp.Profile = "Updated"
	err = db.Save(dp).Error
	require.NoError(t, err)

	assert.True(t, dp.UpdatedAt.After(dp.CreatedAt))
}
