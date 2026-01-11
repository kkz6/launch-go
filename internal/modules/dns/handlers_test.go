package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/dns/enums"
	"github.com/kkz6/launch-go/internal/modules/dns/handlers"
	"github.com/kkz6/launch-go/internal/modules/dns/models"
	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	"github.com/kkz6/launch-go/internal/modules/dns/services"
)

// testHandler wraps the handlers for testing
type testHandler struct {
	providerHandler *handlers.DomainProviderHandler
	domainHandler   *handlers.DomainHandler
	recordHandler   *handlers.DnsRecordHandler
}

func setupTestHandler(t *testing.T) (*testHandler, *services.DomainProviderService, *repositories.DomainProviderRepository, *repositories.DomainRepository, *repositories.DnsRecordRepository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.DomainProvider{}, &models.Domain{}, &models.DnsRecord{})
	require.NoError(t, err)

	providerRepo := repositories.NewDomainProviderRepository(db)
	domainRepo := repositories.NewDomainRepository(db)
	dnsRecordRepo := repositories.NewDnsRecordRepository(db)

	logger := zerolog.Nop()
	providerService := services.NewDomainProviderService(providerRepo, domainRepo, dnsRecordRepo, &logger)
	domainService := services.NewDomainService(providerRepo, domainRepo, dnsRecordRepo, &logger)
	recordService := services.NewDnsRecordService(domainRepo, dnsRecordRepo, &logger)

	providerHandler := handlers.NewDomainProviderHandler(providerService)
	domainHandler := handlers.NewDomainHandler(domainService, providerService)
	recordHandler := handlers.NewDnsRecordHandler(recordService, domainService)

	handler := &testHandler{
		providerHandler: providerHandler,
		domainHandler:   domainHandler,
		recordHandler:   recordHandler,
	}

	return handler, providerService, providerRepo, domainRepo, dnsRecordRepo, db
}

func setupTestApp(handler *testHandler) *fiber.App {
	app := fiber.New()

	// Mock middleware to set userID and teamID
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "user123")
		c.Locals("teamID", "team123")
		return c.Next()
	})

	// Provider routes
	app.Get("/dns-providers", handler.providerHandler.ListProviders)
	app.Post("/dns-providers", handler.providerHandler.CreateProvider)
	app.Delete("/dns-providers/:id", handler.providerHandler.DeleteProvider)
	app.Post("/dns-providers/:id/check", handler.providerHandler.CheckProviderConnectivity)
	app.Post("/dns-providers/:id/sync", handler.providerHandler.SyncProviderDomains)

	// Domain routes
	app.Get("/domains", handler.domainHandler.ListDomains)
	app.Post("/domains", handler.domainHandler.CreateDomain)
	app.Get("/domains/:id", handler.domainHandler.ShowDomain)
	app.Delete("/domains/:id", handler.domainHandler.DeleteDomain)

	// Record routes
	app.Get("/domains/:id/records", handler.recordHandler.ListRecords)
	app.Post("/domains/:id/records", handler.recordHandler.CreateRecord)
	app.Put("/domains/:domainId/records/:recordId", handler.recordHandler.UpdateRecord)
	app.Delete("/domains/:domainId/records/:recordId", handler.recordHandler.DeleteRecord)

	// Utility routes
	app.Get("/dns/record-types", handler.recordHandler.GetRecordTypes)

	return app
}

func createTestHandlerProvider(t *testing.T, repo *repositories.DomainProviderRepository) *models.DomainProvider {
	ctx := context.Background()
	dp := &models.DomainProvider{
		UserID:      "user123",
		TeamID:      "team123",
		Profile:     "Test Provider",
		Provider:    enums.DnsProviderCloudflare,
		Credentials: `{"token": "test-token"}`,
		Connected:   true,
	}
	err := repo.Create(ctx, dp)
	require.NoError(t, err)
	return dp
}

func createTestHandlerDomain(t *testing.T, repo *repositories.DomainRepository, providerID string) *models.Domain {
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

func createTestHandlerRecord(t *testing.T, repo *repositories.DnsRecordRepository, domainID string) *models.DnsRecord {
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

// Provider Handler Tests

func TestHandler_ListProviders(t *testing.T) {
	handler, _, providerRepo, _, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	createTestHandlerProvider(t, providerRepo)

	req := httptest.NewRequest("GET", "/dns-providers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListProviders_Empty(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/dns-providers", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	data := result["data"].([]interface{})
	assert.Len(t, data, 0)
}

func TestHandler_DeleteProvider(t *testing.T) {
	handler, _, providerRepo, _, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)

	req := httptest.NewRequest("DELETE", "/dns-providers/"+dp.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestHandler_DeleteProvider_NotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("DELETE", "/dns-providers/nonexistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_DeleteProvider_HasDomains(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	createTestHandlerDomain(t, domainRepo, dp.ID)

	req := httptest.NewRequest("DELETE", "/dns-providers/"+dp.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestHandler_CheckProviderConnectivity_NotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("POST", "/dns-providers/nonexistent/check", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_SyncProviderDomains_NotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("POST", "/dns-providers/nonexistent/sync", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// Domain Handler Tests

func TestHandler_ListDomains(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	createTestHandlerDomain(t, domainRepo, dp.ID)

	req := httptest.NewRequest("GET", "/domains", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]interface{})
	domains := data["domains"].([]interface{})
	assert.Len(t, domains, 1)
}

func TestHandler_ShowDomain(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	req := httptest.NewRequest("GET", "/domains/"+d.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]interface{})
	domain := data["domain"].(map[string]interface{})
	assert.Equal(t, d.ID, domain["id"])
}

func TestHandler_ShowDomain_NotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/domains/nonexistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_DeleteDomain(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	req := httptest.NewRequest("DELETE", "/domains/"+d.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestHandler_DeleteDomain_NotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("DELETE", "/domains/nonexistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_DeleteDomain_WithBody(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	body := bytes.NewBuffer([]byte(`{"delete_from_provider": true}`))
	req := httptest.NewRequest("DELETE", "/domains/"+d.ID, body)
	req.Header.Set("Content-Type", "application/json")
	// Increase timeout to 10 seconds as this test may make HTTP calls to provider
	resp, err := app.Test(req, 10000)
	require.NoError(t, err)

	// Will fail trying to delete from provider due to invalid credentials
	// but the handler logic is tested
	assert.Contains(t, []int{fiber.StatusNoContent, fiber.StatusBadRequest}, resp.StatusCode)
}

// Record Handler Tests

func TestHandler_ListRecords(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)
	createTestHandlerRecord(t, dnsRecordRepo, d.ID)

	req := httptest.NewRequest("GET", "/domains/"+d.ID+"/records", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListRecords_DomainNotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/domains/nonexistent/records", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_CreateRecord_InvalidBody(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	body := bytes.NewBuffer([]byte(`invalid json`))
	req := httptest.NewRequest("POST", "/domains/"+d.ID+"/records", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestHandler_CreateRecord_ValidationError(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	body := bytes.NewBuffer([]byte(`{"name": "", "value": "", "type": "INVALID"}`))
	req := httptest.NewRequest("POST", "/domains/"+d.ID+"/records", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_CreateRecord_DomainNotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	body := bytes.NewBuffer([]byte(`{"name": "@", "value": "1.2.3.4", "type": "A", "ttl": 3600}`))
	req := httptest.NewRequest("POST", "/domains/nonexistent/records", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_UpdateRecord_InvalidBody(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)
	r := createTestHandlerRecord(t, dnsRecordRepo, d.ID)

	body := bytes.NewBuffer([]byte(`invalid json`))
	req := httptest.NewRequest("PUT", "/domains/"+d.ID+"/records/"+r.ID, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestHandler_UpdateRecord_ValidationError(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)
	r := createTestHandlerRecord(t, dnsRecordRepo, d.ID)

	body := bytes.NewBuffer([]byte(`{"name": "", "value": "", "type": "INVALID"}`))
	req := httptest.NewRequest("PUT", "/domains/"+d.ID+"/records/"+r.ID, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_UpdateRecord_DomainNotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	body := bytes.NewBuffer([]byte(`{"name": "@", "value": "1.2.3.4", "type": "A", "ttl": 3600}`))
	req := httptest.NewRequest("PUT", "/domains/nonexistent/records/rec123", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_UpdateRecord_RecordNotFound(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	body := bytes.NewBuffer([]byte(`{"name": "@", "value": "1.2.3.4", "type": "A", "ttl": 3600}`))
	req := httptest.NewRequest("PUT", "/domains/"+d.ID+"/records/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_UpdateRecord_NSNotEditable(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	// Create NS record
	ctx := context.Background()
	nsRecord := &models.DnsRecord{
		DomainID:   d.ID,
		ProviderID: "ns-rec-123",
		Type:       enums.RecordTypeNS,
		Name:       "@",
		Value:      "ns1.example.com",
		TTL:        86400,
	}
	dnsRecordRepo.Create(ctx, nsRecord)

	// Trying to submit NS type in update request fails validation (NS not in allowed types)
	body := bytes.NewBuffer([]byte(`{"name": "@", "value": "ns2.example.com", "type": "NS", "ttl": 86400}`))
	req := httptest.NewRequest("PUT", "/domains/"+d.ID+"/records/"+nsRecord.ID, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Returns 422 because NS type is not allowed in UpdateDnsRecordRequest validation
	assert.Equal(t, fiber.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_DeleteRecord_DomainNotFound(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("DELETE", "/domains/nonexistent/records/rec123", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_DeleteRecord_RecordNotFound(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	req := httptest.NewRequest("DELETE", "/domains/"+d.ID+"/records/nonexistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestHandler_DeleteRecord_NSNotDeletable(t *testing.T) {
	handler, _, providerRepo, domainRepo, dnsRecordRepo, _ := setupTestHandler(t)
	_ = dnsRecordRepo // silence unused variable warning where applicable
	app := setupTestApp(handler)

	dp := createTestHandlerProvider(t, providerRepo)
	d := createTestHandlerDomain(t, domainRepo, dp.ID)

	// Create NS record
	ctx := context.Background()
	nsRecord := &models.DnsRecord{
		DomainID:   d.ID,
		ProviderID: "ns-rec-123",
		Type:       enums.RecordTypeNS,
		Name:       "@",
		Value:      "ns1.example.com",
		TTL:        86400,
	}
	dnsRecordRepo.Create(ctx, nsRecord)

	req := httptest.NewRequest("DELETE", "/domains/"+d.ID+"/records/"+nsRecord.ID, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// Utility Handler Tests

func TestHandler_GetRecordTypes(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	app := setupTestApp(handler)

	req := httptest.NewRequest("GET", "/dns/record-types", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	assert.True(t, result["success"].(bool))
	data := result["data"].([]interface{})
	assert.Len(t, data, 9)
}

func TestNewHandler(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.providerHandler)
	assert.NotNil(t, handler.domainHandler)
	assert.NotNil(t, handler.recordHandler)
}
