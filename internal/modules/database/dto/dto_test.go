package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/database/models"
)

func TestToDatabaseResponse(t *testing.T) {
	now := time.Now()

	t.Run("converts database with all fields", func(t *testing.T) {
		database := &models.Database{
			ID:                        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:                  "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:                      "test_db",
			InstalledAt:               &now,
			InstallationFailedAt:      nil,
			UninstallationRequestedAt: nil,
			CreatedAt:                 now,
			UpdatedAt:                 now,
			Users: []models.DatabaseUser{
				{ID: "user1", Name: "test_user"},
			},
		}

		resp := ToDatabaseResponse(database)

		assert.Equal(t, database.ID, resp.ID)
		assert.Equal(t, database.ServerID, resp.ServerID)
		assert.Equal(t, database.Name, resp.Name)
		assert.Equal(t, "installed", resp.Status)
		assert.NotNil(t, resp.InstalledAt)
		assert.Nil(t, resp.InstallationFailedAt)
		assert.Nil(t, resp.UninstallationRequestedAt)
		assert.Len(t, resp.Users, 1)
		assert.Equal(t, "user1", resp.Users[0].ID)
	})

	t.Run("converts database with failed installation", func(t *testing.T) {
		database := &models.Database{
			ID:                   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:             "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:                 "failed_db",
			InstallationFailedAt: &now,
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		resp := ToDatabaseResponse(database)

		assert.Equal(t, "failed", resp.Status)
		assert.NotNil(t, resp.InstallationFailedAt)
	})

	t.Run("converts database being uninstalled", func(t *testing.T) {
		database := &models.Database{
			ID:                        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:                  "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:                      "uninstalling_db",
			InstalledAt:               &now,
			UninstallationRequestedAt: &now,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		}

		resp := ToDatabaseResponse(database)

		assert.Equal(t, "uninstalling", resp.Status)
		assert.NotNil(t, resp.UninstallationRequestedAt)
	})

	t.Run("converts database with no users", func(t *testing.T) {
		database := &models.Database{
			ID:        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:  "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:      "no_users_db",
			CreatedAt: now,
			UpdatedAt: now,
		}

		resp := ToDatabaseResponse(database)

		assert.Nil(t, resp.Users)
	})
}

func TestToDatabaseUserResponse(t *testing.T) {
	now := time.Now()

	t.Run("converts user with all fields", func(t *testing.T) {
		user := &models.DatabaseUser{
			ID:                        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:                  "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:                      "test_user",
			Host:                      "localhost",
			InstalledAt:               &now,
			InstallationFailedAt:      nil,
			UninstallationRequestedAt: nil,
			CreatedAt:                 now,
			UpdatedAt:                 now,
			Databases: []models.Database{
				{ID: "db1", Name: "test_db"},
			},
		}

		resp := ToDatabaseUserResponse(user)

		assert.Equal(t, user.ID, resp.ID)
		assert.Equal(t, user.ServerID, resp.ServerID)
		assert.Equal(t, user.Name, resp.Name)
		assert.Equal(t, user.Host, resp.Host)
		assert.Equal(t, "installed", resp.Status)
		assert.NotNil(t, resp.InstalledAt)
		assert.Nil(t, resp.InstallationFailedAt)
		assert.Nil(t, resp.UninstallationRequestedAt)
		assert.Len(t, resp.Databases, 1)
		assert.Len(t, resp.DatabaseIDs, 1)
		assert.Equal(t, "db1", resp.Databases[0].ID)
		assert.Equal(t, "db1", resp.DatabaseIDs[0])
	})

	t.Run("converts user with failed installation", func(t *testing.T) {
		user := &models.DatabaseUser{
			ID:                   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:             "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:                 "failed_user",
			Host:                 "localhost",
			InstallationFailedAt: &now,
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		resp := ToDatabaseUserResponse(user)

		assert.Equal(t, "failed", resp.Status)
		assert.NotNil(t, resp.InstallationFailedAt)
	})

	t.Run("converts user being uninstalled", func(t *testing.T) {
		user := &models.DatabaseUser{
			ID:                        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:                  "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:                      "uninstalling_user",
			Host:                      "localhost",
			InstalledAt:               &now,
			UninstallationRequestedAt: &now,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		}

		resp := ToDatabaseUserResponse(user)

		assert.Equal(t, "uninstalling", resp.Status)
		assert.NotNil(t, resp.UninstallationRequestedAt)
	})

	t.Run("converts user with no databases", func(t *testing.T) {
		user := &models.DatabaseUser{
			ID:        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ServerID:  "01ARZ3NDEKTSV4RRFFQ69G5FA1",
			Name:      "no_db_user",
			Host:      "localhost",
			CreatedAt: now,
			UpdatedAt: now,
		}

		resp := ToDatabaseUserResponse(user)

		assert.Nil(t, resp.Databases)
		assert.Nil(t, resp.DatabaseIDs)
	})
}

func TestToDatabaseResponseList(t *testing.T) {
	now := time.Now()

	t.Run("converts list of databases", func(t *testing.T) {
		databases := []models.Database{
			{ID: "db1", ServerID: "server1", Name: "database1", CreatedAt: now, UpdatedAt: now},
			{ID: "db2", ServerID: "server1", Name: "database2", CreatedAt: now, UpdatedAt: now},
		}

		resp := ToDatabaseResponseList(databases)

		assert.Len(t, resp, 2)
		assert.Equal(t, "db1", resp[0].ID)
		assert.Equal(t, "db2", resp[1].ID)
	})

	t.Run("returns empty list for empty input", func(t *testing.T) {
		databases := []models.Database{}

		resp := ToDatabaseResponseList(databases)

		assert.Len(t, resp, 0)
	})
}

func TestToDatabaseUserResponseList(t *testing.T) {
	now := time.Now()

	t.Run("converts list of users", func(t *testing.T) {
		users := []models.DatabaseUser{
			{ID: "user1", ServerID: "server1", Name: "user1", Host: "localhost", CreatedAt: now, UpdatedAt: now},
			{ID: "user2", ServerID: "server1", Name: "user2", Host: "localhost", CreatedAt: now, UpdatedAt: now},
		}

		resp := ToDatabaseUserResponseList(users)

		assert.Len(t, resp, 2)
		assert.Equal(t, "user1", resp[0].ID)
		assert.Equal(t, "user2", resp[1].ID)
	})

	t.Run("returns empty list for empty input", func(t *testing.T) {
		users := []models.DatabaseUser{}

		resp := ToDatabaseUserResponseList(users)

		assert.Len(t, resp, 0)
	})
}
