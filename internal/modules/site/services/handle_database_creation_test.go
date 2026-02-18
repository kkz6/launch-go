package services

import (
	"context"
	"testing"

	databasedto "github.com/kkz6/launch-go/internal/modules/database/dto"
	databasemodels "github.com/kkz6/launch-go/internal/modules/database/models"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// mockDatabaseManager implements contracts.DatabaseManager for testing
type mockDatabaseManager struct {
	database *databasemodels.Database
	dbUser   *databasemodels.DatabaseUser
	err      error
}

func (m *mockDatabaseManager) CreateDatabase(_ context.Context, _, _ string, _ *databasedto.CreateDatabaseRequest, _ *string) (*databasemodels.Database, error) {
	return m.database, m.err
}

func (m *mockDatabaseManager) GetDatabase(_ context.Context, _, _, _ string) (*databasemodels.Database, error) {
	return m.database, m.err
}

func (m *mockDatabaseManager) GetDatabaseUser(_ context.Context, _, _ string) (*databasemodels.DatabaseUser, error) {
	return m.dbUser, m.err
}

// mockServerReader implements contracts.ServerReader for testing
type mockServerReader struct {
	services []servermodels.InstalledService
}

func (m *mockServerReader) FindServerByID(_ context.Context, _ string) (*servermodels.Server, error) {
	return nil, nil
}

func (m *mockServerReader) FindServicesByServer(_ context.Context, _ string) ([]servermodels.InstalledService, error) {
	return m.services, nil
}

func newTestSiteService(dbManager contracts.DatabaseManager, serverReader contracts.ServerReader) *SiteService {
	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: service.Dependencies{
				Logger: &logger,
			},
		},
	}

	svc := NewSiteService(deps)
	svc.SetDatabaseManager(dbManager)
	if serverReader != nil {
		svc.SetServerReader(serverReader)
	}

	return svc
}

func TestHandleDatabaseCreation_ExistingDB_ExistingUser(t *testing.T) {
	dbUser := &databasemodels.DatabaseUser{
		BaseModel: basemodels.BaseModel{ID: "user-123"},
		Name:      "app_user",
		Password: &dbtype.EncryptedNullableString{
			String: "secret_password",
			Valid:  true,
		},
	}

	database := &databasemodels.Database{
		BaseModel: basemodels.BaseModel{ID: "db-123"},
		Name:      "my_database",
	}

	dbManager := &mockDatabaseManager{
		database: database,
		dbUser:   dbUser,
	}

	serverReader := &mockServerReader{
		services: []servermodels.InstalledService{
			{Type: servertypes.ServiceTypeMySQL},
		},
	}

	svc := newTestSiteService(dbManager, serverReader)

	site := &sitemodels.Site{
		Type: sitetypes.SiteTypeLaravel,
	}

	dbUserID := "user-123"
	dbID := "db-123"
	req := &dto.CreateSiteRequest{
		CreateDatabase:     true,
		DatabaseOption:     "existing",
		DatabaseID:         &dbID,
		DatabaseUserOption: "existing",
		DatabaseUserID:     &dbUserID,
	}

	envVars := svc.handleDatabaseCreation(context.Background(), site, "server-1", "team-1", "user-1", req)

	assert.Equal(t, "my_database", envVars["DB_DATABASE"])
	assert.Equal(t, "127.0.0.1", envVars["DB_HOST"])
	assert.Equal(t, "app_user", envVars["DB_USERNAME"])
	assert.Equal(t, "secret_password", envVars["DB_PASSWORD"])
}

func TestHandleDatabaseCreation_NewDB_ExistingUser(t *testing.T) {
	dbUser := &databasemodels.DatabaseUser{
		BaseModel: basemodels.BaseModel{ID: "user-456"},
		Name:      "existing_user",
		Password: &dbtype.EncryptedNullableString{
			String: "existing_pass",
			Valid:  true,
		},
	}

	database := &databasemodels.Database{
		BaseModel: basemodels.BaseModel{ID: "db-456"},
		Name:      "new_database",
	}

	dbManager := &mockDatabaseManager{
		database: database,
		dbUser:   dbUser,
	}

	serverReader := &mockServerReader{
		services: []servermodels.InstalledService{
			{Type: servertypes.ServiceTypeMySQL},
		},
	}

	svc := newTestSiteService(dbManager, serverReader)

	site := &sitemodels.Site{
		Type: sitetypes.SiteTypeLaravel,
	}

	dbName := "new_database"
	dbUserID := "user-456"
	req := &dto.CreateSiteRequest{
		CreateDatabase:     true,
		DatabaseOption:     "new",
		DatabaseName:       &dbName,
		DatabaseUserOption: "existing",
		DatabaseUserID:     &dbUserID,
	}

	envVars := svc.handleDatabaseCreation(context.Background(), site, "server-1", "team-1", "user-1", req)

	assert.Equal(t, "new_database", envVars["DB_DATABASE"])
	assert.Equal(t, "127.0.0.1", envVars["DB_HOST"])
	assert.Equal(t, "mysql", envVars["DB_CONNECTION"])
	assert.Equal(t, "3306", envVars["DB_PORT"])
	assert.Equal(t, "existing_user", envVars["DB_USERNAME"])
	assert.Equal(t, "existing_pass", envVars["DB_PASSWORD"])
}

func TestHandleDatabaseCreation_NewDB_NewUser(t *testing.T) {
	database := &databasemodels.Database{
		BaseModel: basemodels.BaseModel{ID: "db-789"},
		Name:      "fresh_database",
	}

	dbManager := &mockDatabaseManager{
		database: database,
	}

	serverReader := &mockServerReader{
		services: []servermodels.InstalledService{
			{Type: servertypes.ServiceTypeMySQL},
		},
	}

	svc := newTestSiteService(dbManager, serverReader)

	site := &sitemodels.Site{
		Type: sitetypes.SiteTypeLaravel,
	}

	dbName := "fresh_database"
	userName := "new_user"
	userPass := "new_pass"
	req := &dto.CreateSiteRequest{
		CreateDatabase:       true,
		DatabaseOption:       "new",
		DatabaseName:         &dbName,
		DatabaseUserOption:   "new",
		DatabaseUserName:     &userName,
		DatabaseUserPassword: &userPass,
	}

	envVars := svc.handleDatabaseCreation(context.Background(), site, "server-1", "team-1", "user-1", req)

	assert.Equal(t, "fresh_database", envVars["DB_DATABASE"])
	assert.Equal(t, "127.0.0.1", envVars["DB_HOST"])
	assert.Equal(t, "mysql", envVars["DB_CONNECTION"])
	assert.Equal(t, "3306", envVars["DB_PORT"])
	assert.Equal(t, "new_user", envVars["DB_USERNAME"])
	assert.Equal(t, "new_pass", envVars["DB_PASSWORD"])
}

func TestHandleDatabaseCreation_ExistingDB_NoUser(t *testing.T) {
	database := &databasemodels.Database{
		BaseModel: basemodels.BaseModel{ID: "db-abc"},
		Name:      "some_database",
	}

	dbManager := &mockDatabaseManager{
		database: database,
	}

	serverReader := &mockServerReader{
		services: []servermodels.InstalledService{
			{Type: servertypes.ServiceTypeMySQL},
		},
	}

	svc := newTestSiteService(dbManager, serverReader)

	site := &sitemodels.Site{
		Type: sitetypes.SiteTypeLaravel,
	}

	dbID := "db-abc"
	req := &dto.CreateSiteRequest{
		CreateDatabase: true,
		DatabaseOption: "existing",
		DatabaseID:     &dbID,
	}

	envVars := svc.handleDatabaseCreation(context.Background(), site, "server-1", "team-1", "user-1", req)

	assert.Equal(t, "some_database", envVars["DB_DATABASE"])
	assert.Equal(t, "127.0.0.1", envVars["DB_HOST"])
	assert.Empty(t, envVars["DB_USERNAME"])
	assert.Empty(t, envVars["DB_PASSWORD"])
}

func TestHandleDatabaseCreation_WordPress_ExistingUser(t *testing.T) {
	dbUser := &databasemodels.DatabaseUser{
		BaseModel: basemodels.BaseModel{ID: "user-wp"},
		Name:      "wp_user",
		Password: &dbtype.EncryptedNullableString{
			String: "wp_pass",
			Valid:  true,
		},
	}

	database := &databasemodels.Database{
		BaseModel: basemodels.BaseModel{ID: "db-wp"},
		Name:      "wp_database",
	}

	dbManager := &mockDatabaseManager{
		database: database,
		dbUser:   dbUser,
	}

	serverReader := &mockServerReader{
		services: []servermodels.InstalledService{
			{Type: servertypes.ServiceTypeMySQL},
		},
	}

	svc := newTestSiteService(dbManager, serverReader)

	site := &sitemodels.Site{
		Type: sitetypes.SiteTypeWordpress,
	}

	dbID := "db-wp"
	dbUserID := "user-wp"
	req := &dto.CreateSiteRequest{
		CreateDatabase:     true,
		DatabaseOption:     "existing",
		DatabaseID:         &dbID,
		DatabaseUserOption: "existing",
		DatabaseUserID:     &dbUserID,
	}

	envVars := svc.handleDatabaseCreation(context.Background(), site, "server-1", "team-1", "user-1", req)

	// WordPress uses different env var names
	assert.Equal(t, "wp_database", envVars["DB_NAME"])
	assert.Equal(t, "127.0.0.1", envVars["DB_HOST"])
	assert.Equal(t, "wp_user", envVars["DB_USER"])
	assert.Equal(t, "wp_pass", envVars["DB_PASSWORD"])
}
