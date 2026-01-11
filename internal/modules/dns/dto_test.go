package dns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToDomainProviderResponse(t *testing.T) {
	now := time.Now()
	errMsg := "sync failed"

	dp := &DomainProvider{
		ID:               "provider123",
		Profile:          "My Provider",
		Provider:         DnsProviderCloudflare,
		Connected:        true,
		SyncStatus:       SyncStatusCompleted,
		LastSyncedAt:     &now,
		SyncErrorMessage: &errMsg,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	resp := ToDomainProviderResponse(dp, 5)

	assert.Equal(t, "provider123", resp.ID)
	assert.Equal(t, "My Provider", resp.Profile)
	assert.Equal(t, "cloudflare", resp.Provider)
	assert.Equal(t, "Cloudflare", resp.ProviderLabel)
	assert.True(t, resp.Connected)
	assert.Equal(t, "completed", resp.SyncStatus)
	assert.Equal(t, &now, resp.LastSyncedAt)
	assert.Equal(t, &errMsg, resp.SyncErrorMessage)
	assert.Equal(t, 5, resp.DomainsCount)
	assert.Equal(t, now, resp.CreatedAt)
	assert.Equal(t, now, resp.UpdatedAt)
}

func TestToDomainResponse(t *testing.T) {
	now := time.Now()

	domain := &Domain{
		ID:               "domain123",
		Label:            "My Domain",
		Address:          "example.com",
		ProviderID:       "ext-123",
		DomainProviderID: "provider123",
		CreatedAt:        now,
		UpdatedAt:        now,
		Provider: &DomainProvider{
			ID:       "provider123",
			Profile:  "My Provider",
			Provider: DnsProviderCloudflare,
		},
		Records: []DnsRecord{
			{
				ID:         "record1",
				DomainID:   "domain123",
				ProviderID: "ext-rec1",
				Type:       RecordTypeA,
				Name:       "@",
				Value:      "1.2.3.4",
				TTL:        3600,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
		},
	}

	resp := ToDomainResponse(domain)

	assert.Equal(t, "domain123", resp.ID)
	assert.Equal(t, "My Domain", resp.Label)
	assert.Equal(t, "example.com", resp.Address)
	assert.Equal(t, "ext-123", resp.ProviderID)
	assert.Equal(t, "provider123", resp.DomainProviderID)
	assert.Equal(t, 1, resp.RecordsCount)
	assert.Len(t, resp.Records, 1)

	assert.NotNil(t, resp.Provider)
	assert.Equal(t, "provider123", resp.Provider.ID)
	assert.Equal(t, "My Provider", resp.Provider.Profile)
	assert.Equal(t, "cloudflare", resp.Provider.Provider)
}

func TestToDomainResponse_NoProvider(t *testing.T) {
	domain := &Domain{
		ID:       "domain123",
		Label:    "My Domain",
		Address:  "example.com",
		Provider: nil,
		Records:  []DnsRecord{},
	}

	resp := ToDomainResponse(domain)

	assert.Nil(t, resp.Provider)
	assert.Equal(t, 0, resp.RecordsCount)
	assert.Nil(t, resp.Records)
}

func TestCreateDnsRecordRequest_ToModel(t *testing.T) {
	priority := 10
	weight := 5
	port := 443
	flags := 0
	proxied := true

	req := &CreateDnsRecordRequest{
		Name:     "mail",
		Value:    "mail.example.com",
		Type:     "MX",
		TTL:      7200,
		Priority: &priority,
		Weight:   &weight,
		Port:     &port,
		Flags:    &flags,
		Tag:      "issue",
		Comment:  "Mail server",
		Proxied:  &proxied,
	}

	record := req.ToModel("domain123")

	assert.Equal(t, "domain123", record.DomainID)
	assert.Equal(t, RecordTypeMX, record.Type)
	assert.Equal(t, "mail", record.Name)
	assert.Equal(t, "mail.example.com", record.Value)
	assert.Equal(t, 7200, record.TTL)
	assert.Equal(t, 10, *record.Priority)
	assert.Equal(t, 5, *record.Weight)
	assert.Equal(t, 443, *record.Port)
	assert.Equal(t, 0, *record.Flags)
	assert.Equal(t, "issue", *record.Tag)
	assert.Equal(t, "Mail server", *record.Comment)
	assert.True(t, *record.Proxied)
}

func TestCreateDnsRecordRequest_ToModel_DefaultTTL(t *testing.T) {
	req := &CreateDnsRecordRequest{
		Name:  "@",
		Value: "1.2.3.4",
		Type:  "A",
		TTL:   0,
	}

	record := req.ToModel("domain123")

	assert.Equal(t, 3600, record.TTL)
}

func TestCreateDnsRecordRequest_ToModel_EmptyOptionalFields(t *testing.T) {
	req := &CreateDnsRecordRequest{
		Name:    "@",
		Value:   "1.2.3.4",
		Type:    "A",
		TTL:     3600,
		Tag:     "",
		Comment: "",
	}

	record := req.ToModel("domain123")

	assert.Nil(t, record.Tag)
	assert.Nil(t, record.Comment)
	assert.Nil(t, record.Priority)
	assert.Nil(t, record.Weight)
	assert.Nil(t, record.Port)
	assert.Nil(t, record.Flags)
	assert.Nil(t, record.Proxied)
}

func TestUpdateDnsRecordRequest_ApplyToModel(t *testing.T) {
	priority := 20
	weight := 10
	port := 8443
	flags := 1
	proxied := false

	req := &UpdateDnsRecordRequest{
		Name:     "www",
		Value:    "1.2.3.5",
		Type:     "A",
		TTL:      7200,
		Priority: &priority,
		Weight:   &weight,
		Port:     &port,
		Flags:    &flags,
		Tag:      "updated-tag",
		Comment:  "Updated comment",
		Proxied:  &proxied,
	}

	record := &DnsRecord{
		ID:       "record123",
		DomainID: "domain123",
		Type:     RecordTypeCNAME,
		Name:     "old",
		Value:    "old.example.com",
		TTL:      3600,
	}

	req.ApplyToModel(record)

	assert.Equal(t, "record123", record.ID)
	assert.Equal(t, "domain123", record.DomainID)
	assert.Equal(t, RecordTypeA, record.Type)
	assert.Equal(t, "www", record.Name)
	assert.Equal(t, "1.2.3.5", record.Value)
	assert.Equal(t, 7200, record.TTL)
	assert.Equal(t, 20, *record.Priority)
	assert.Equal(t, 10, *record.Weight)
	assert.Equal(t, 8443, *record.Port)
	assert.Equal(t, 1, *record.Flags)
	assert.Equal(t, "updated-tag", *record.Tag)
	assert.Equal(t, "Updated comment", *record.Comment)
	assert.False(t, *record.Proxied)
}

func TestUpdateDnsRecordRequest_ApplyToModel_ClearOptionalFields(t *testing.T) {
	tag := "old-tag"
	comment := "old-comment"

	record := &DnsRecord{
		ID:      "record123",
		Type:    RecordTypeA,
		Name:    "www",
		Value:   "1.2.3.4",
		TTL:     3600,
		Tag:     &tag,
		Comment: &comment,
	}

	req := &UpdateDnsRecordRequest{
		Name:    "www",
		Value:   "1.2.3.4",
		Type:    "A",
		TTL:     3600,
		Tag:     "",
		Comment: "",
	}

	req.ApplyToModel(record)

	assert.Nil(t, record.Tag)
	assert.Nil(t, record.Comment)
}

func TestUpdateDnsRecordRequest_ApplyToModel_KeepTTL(t *testing.T) {
	record := &DnsRecord{
		ID:    "record123",
		Type:  RecordTypeA,
		Name:  "www",
		Value: "1.2.3.4",
		TTL:   7200,
	}

	req := &UpdateDnsRecordRequest{
		Name:  "www",
		Value: "1.2.3.4",
		Type:  "A",
		TTL:   0,
	}

	req.ApplyToModel(record)

	assert.Equal(t, 7200, record.TTL)
}

func TestToDnsRecordResponse(t *testing.T) {
	now := time.Now()
	priority := 10
	tag := "issue"
	comment := "Test comment"
	proxied := true

	record := &DnsRecord{
		ID:         "record123",
		DomainID:   "domain123",
		ProviderID: "ext-123",
		Type:       RecordTypeMX,
		Name:       "mail",
		Value:      "mail.example.com",
		TTL:        3600,
		Priority:   &priority,
		Tag:        &tag,
		Comment:    &comment,
		Proxied:    &proxied,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	resp := ToDnsRecordResponse(record)

	assert.Equal(t, "record123", resp.ID)
	assert.Equal(t, "domain123", resp.DomainID)
	assert.Equal(t, "ext-123", resp.ProviderID)
	assert.Equal(t, "MX", resp.Type)
	assert.Equal(t, "mail", resp.Name)
	assert.Equal(t, "mail.example.com", resp.Value)
	assert.Equal(t, 3600, resp.TTL)
	assert.Equal(t, 10, *resp.Priority)
	assert.Equal(t, "issue", *resp.Tag)
	assert.Equal(t, "Test comment", *resp.Comment)
	assert.True(t, *resp.Proxied)
	assert.True(t, resp.IsEditable)
	assert.Equal(t, now.Format(time.RFC3339), resp.CreatedAt)
	assert.Equal(t, now.Format(time.RFC3339), resp.UpdatedAt)
}

func TestToDnsRecordResponse_NSRecord(t *testing.T) {
	record := &DnsRecord{
		ID:       "record123",
		Type:     RecordTypeNS,
		Name:     "@",
		Value:    "ns1.example.com",
		TTL:      86400,
	}

	resp := ToDnsRecordResponse(record)

	assert.Equal(t, "NS", resp.Type)
	assert.False(t, resp.IsEditable)
}

func TestDeleteDomainRequest(t *testing.T) {
	req := DeleteDomainRequest{
		DeleteFromProvider: true,
	}

	assert.True(t, req.DeleteFromProvider)

	req2 := DeleteDomainRequest{}
	assert.False(t, req2.DeleteFromProvider)
}

func TestSyncDomainsRequest(t *testing.T) {
	req := SyncDomainsRequest{
		ProviderID: "provider123",
	}

	assert.Equal(t, "provider123", req.ProviderID)
}

func TestCreateDomainProviderRequest(t *testing.T) {
	req := CreateDomainProviderRequest{
		Profile:   "My Provider",
		Provider:  "cloudflare",
		Token:     "test-token",
		AccountID: "account123",
	}

	assert.Equal(t, "My Provider", req.Profile)
	assert.Equal(t, "cloudflare", req.Provider)
	assert.Equal(t, "test-token", req.Token)
	assert.Equal(t, "account123", req.AccountID)
}

func TestUpdateDomainProviderRequest(t *testing.T) {
	req := UpdateDomainProviderRequest{
		Profile: "Updated Provider",
	}

	assert.Equal(t, "Updated Provider", req.Profile)
}

func TestCreateDomainRequest(t *testing.T) {
	req := CreateDomainRequest{
		Label:    "My Domain",
		Address:  "example.com",
		Provider: "provider123",
	}

	assert.Equal(t, "My Domain", req.Label)
	assert.Equal(t, "example.com", req.Address)
	assert.Equal(t, "provider123", req.Provider)
}

func TestProviderSummary(t *testing.T) {
	ps := ProviderSummary{
		ID:       "provider123",
		Profile:  "My Provider",
		Provider: "cloudflare",
	}

	assert.Equal(t, "provider123", ps.ID)
	assert.Equal(t, "My Provider", ps.Profile)
	assert.Equal(t, "cloudflare", ps.Provider)
}
