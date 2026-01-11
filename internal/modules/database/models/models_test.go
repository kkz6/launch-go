package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/enums"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
	require.NoError(t, err)

	return db
}

func TestDatabase_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		database := &Database{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:     "test_db",
		}

		err := db.Create(database).Error
		require.NoError(t, err)

		assert.NotEmpty(t, database.ID)
		assert.Len(t, database.ID, 26)
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existingID := "01ARZ3NDEKTSV4RRFFQ69G5FA1"
		database := &Database{
			ID:       existingID,
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:     "test_db_2",
		}

		err := db.Create(database).Error
		require.NoError(t, err)

		assert.Equal(t, existingID, database.ID)
	})
}

func TestDatabase_Status(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		database Database
		expected enums.InstallationStatus
	}{
		{
			name:     "pending when no timestamps set",
			database: Database{},
			expected: enums.StatusPending,
		},
		{
			name: "installed when installed_at is set",
			database: Database{
				InstalledAt: &now,
			},
			expected: enums.StatusInstalled,
		},
		{
			name: "failed when installation_failed_at is set",
			database: Database{
				InstallationFailedAt: &now,
			},
			expected: enums.StatusFailed,
		},
		{
			name: "uninstalling when uninstallation_requested_at is set",
			database: Database{
				InstalledAt:               &now,
				UninstallationRequestedAt: &now,
			},
			expected: enums.StatusUninstalling,
		},
		{
			name: "uninstalling takes priority when both failed and uninstalling",
			database: Database{
				InstallationFailedAt:      &now,
				UninstallationRequestedAt: &now,
			},
			expected: enums.StatusUninstalling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.database.Status())
		})
	}
}

func TestDatabase_IsInstalled(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		database Database
		expected bool
	}{
		{
			name:     "not installed when no timestamps",
			database: Database{},
			expected: false,
		},
		{
			name: "installed when installed_at is set",
			database: Database{
				InstalledAt: &now,
			},
			expected: true,
		},
		{
			name: "not installed when failed",
			database: Database{
				InstalledAt:          &now,
				InstallationFailedAt: &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.database.IsInstalled())
		})
	}
}

func TestDatabase_IsInstalling(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		database Database
		expected bool
	}{
		{
			name:     "installing when no timestamps",
			database: Database{},
			expected: true,
		},
		{
			name: "not installing when installed",
			database: Database{
				InstalledAt: &now,
			},
			expected: false,
		},
		{
			name: "not installing when failed",
			database: Database{
				InstallationFailedAt: &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.database.IsInstalling())
		})
	}
}

func TestDatabase_IsFailed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		database Database
		expected bool
	}{
		{
			name:     "not failed when no timestamps",
			database: Database{},
			expected: false,
		},
		{
			name: "failed when installation_failed_at is set",
			database: Database{
				InstallationFailedAt: &now,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.database.IsFailed())
		})
	}
}

func TestDatabase_IsUninstalling(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		database Database
		expected bool
	}{
		{
			name:     "not uninstalling when no timestamps",
			database: Database{},
			expected: false,
		},
		{
			name: "uninstalling when uninstallation_requested_at is set",
			database: Database{
				UninstallationRequestedAt: &now,
			},
			expected: true,
		},
		{
			name: "not uninstalling when uninstallation failed",
			database: Database{
				UninstallationRequestedAt: &now,
				UninstallationFailedAt:    &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.database.IsUninstalling())
		})
	}
}

func TestDatabase_MarkAsInstalled(t *testing.T) {
	now := time.Now()

	database := &Database{
		InstallationFailedAt: &now,
	}

	database.MarkAsInstalled()

	assert.NotNil(t, database.InstalledAt)
	assert.Nil(t, database.InstallationFailedAt)
}

func TestDatabase_MarkAsFailed(t *testing.T) {
	database := &Database{}

	database.MarkAsFailed()

	assert.NotNil(t, database.InstallationFailedAt)
}

func TestDatabase_MarkAsUninstalling(t *testing.T) {
	database := &Database{}

	database.MarkAsUninstalling()

	assert.NotNil(t, database.UninstallationRequestedAt)
}

func TestDatabase_MarkUninstallationFailed(t *testing.T) {
	now := time.Now()

	database := &Database{
		UninstallationRequestedAt: &now,
	}

	database.MarkUninstallationFailed()

	assert.NotNil(t, database.UninstallationFailedAt)
}

func TestDatabaseUser_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		user := &DatabaseUser{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:     "test_user",
			Password: "secret123",
		}

		err := db.Create(user).Error
		require.NoError(t, err)

		assert.NotEmpty(t, user.ID)
		assert.Len(t, user.ID, 26)
	})

	t.Run("sets default host when empty", func(t *testing.T) {
		user := &DatabaseUser{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:     "test_user_2",
			Password: "secret123",
		}

		err := db.Create(user).Error
		require.NoError(t, err)

		assert.Equal(t, "localhost", user.Host)
	})

	t.Run("preserves existing host", func(t *testing.T) {
		user := &DatabaseUser{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:     "test_user_3",
			Password: "secret123",
			Host:     "192.168.1.1",
		}

		err := db.Create(user).Error
		require.NoError(t, err)

		assert.Equal(t, "192.168.1.1", user.Host)
	})
}

func TestDatabaseUser_Status(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		user     DatabaseUser
		expected enums.InstallationStatus
	}{
		{
			name:     "pending when no timestamps set",
			user:     DatabaseUser{},
			expected: enums.StatusPending,
		},
		{
			name: "installed when installed_at is set",
			user: DatabaseUser{
				InstalledAt: &now,
			},
			expected: enums.StatusInstalled,
		},
		{
			name: "failed when installation_failed_at is set",
			user: DatabaseUser{
				InstallationFailedAt: &now,
			},
			expected: enums.StatusFailed,
		},
		{
			name: "uninstalling when uninstallation_requested_at is set",
			user: DatabaseUser{
				InstalledAt:               &now,
				UninstallationRequestedAt: &now,
			},
			expected: enums.StatusUninstalling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.user.Status())
		})
	}
}

func TestDatabaseUser_IsInstalled(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		user     DatabaseUser
		expected bool
	}{
		{
			name:     "not installed when no timestamps",
			user:     DatabaseUser{},
			expected: false,
		},
		{
			name: "installed when installed_at is set",
			user: DatabaseUser{
				InstalledAt: &now,
			},
			expected: true,
		},
		{
			name: "not installed when failed",
			user: DatabaseUser{
				InstalledAt:          &now,
				InstallationFailedAt: &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.user.IsInstalled())
		})
	}
}

func TestDatabaseUser_IsInstalling(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		user     DatabaseUser
		expected bool
	}{
		{
			name:     "installing when no timestamps",
			user:     DatabaseUser{},
			expected: true,
		},
		{
			name: "not installing when installed",
			user: DatabaseUser{
				InstalledAt: &now,
			},
			expected: false,
		},
		{
			name: "not installing when failed",
			user: DatabaseUser{
				InstallationFailedAt: &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.user.IsInstalling())
		})
	}
}

func TestDatabaseUser_IsFailed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		user     DatabaseUser
		expected bool
	}{
		{
			name:     "not failed when no timestamps",
			user:     DatabaseUser{},
			expected: false,
		},
		{
			name: "failed when installation_failed_at is set",
			user: DatabaseUser{
				InstallationFailedAt: &now,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.user.IsFailed())
		})
	}
}

func TestDatabaseUser_IsUninstalling(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		user     DatabaseUser
		expected bool
	}{
		{
			name:     "not uninstalling when no timestamps",
			user:     DatabaseUser{},
			expected: false,
		},
		{
			name: "uninstalling when uninstallation_requested_at is set",
			user: DatabaseUser{
				UninstallationRequestedAt: &now,
			},
			expected: true,
		},
		{
			name: "not uninstalling when uninstallation failed",
			user: DatabaseUser{
				UninstallationRequestedAt: &now,
				UninstallationFailedAt:    &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.user.IsUninstalling())
		})
	}
}

func TestDatabaseUser_MarkAsInstalled(t *testing.T) {
	now := time.Now()

	user := &DatabaseUser{
		InstallationFailedAt: &now,
	}

	user.MarkAsInstalled()

	assert.NotNil(t, user.InstalledAt)
	assert.Nil(t, user.InstallationFailedAt)
}

func TestDatabaseUser_MarkAsFailed(t *testing.T) {
	user := &DatabaseUser{}

	user.MarkAsFailed()

	assert.NotNil(t, user.InstallationFailedAt)
}

func TestDatabaseUser_MarkAsUninstalling(t *testing.T) {
	user := &DatabaseUser{}

	user.MarkAsUninstalling()

	assert.NotNil(t, user.UninstallationRequestedAt)
}

func TestDatabaseUser_MarkUninstallationFailed(t *testing.T) {
	now := time.Now()

	user := &DatabaseUser{
		UninstallationRequestedAt: &now,
	}

	user.MarkUninstallationFailed()

	assert.NotNil(t, user.UninstallationFailedAt)
}

func TestDatabaseDatabaseUser_TableName(t *testing.T) {
	pivot := DatabaseDatabaseUser{}
	assert.Equal(t, "database_database_user", pivot.TableName())
}

func TestInstallationStatus_Values(t *testing.T) {
	assert.Equal(t, enums.InstallationStatus("pending"), enums.StatusPending)
	assert.Equal(t, enums.InstallationStatus("installing"), enums.StatusInstalling)
	assert.Equal(t, enums.InstallationStatus("installed"), enums.StatusInstalled)
	assert.Equal(t, enums.InstallationStatus("failed"), enums.StatusFailed)
	assert.Equal(t, enums.InstallationStatus("uninstalling"), enums.StatusUninstalling)
}

func TestDatabase_ManyToManyRelationship(t *testing.T) {
	db := setupTestDB(t)

	// Create a database
	database := &Database{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:     "test_db",
	}
	err := db.Create(database).Error
	require.NoError(t, err)

	// Create a database user
	user := &DatabaseUser{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:     "test_user",
		Password: "secret",
	}
	err = db.Create(user).Error
	require.NoError(t, err)

	// Create association
	pivot := &DatabaseDatabaseUser{
		DatabaseID:     database.ID,
		DatabaseUserID: user.ID,
	}
	err = db.Create(pivot).Error
	require.NoError(t, err)

	// Verify database can load users
	var loadedDB Database
	err = db.Preload("Users").First(&loadedDB, "id = ?", database.ID).Error
	require.NoError(t, err)
	assert.Len(t, loadedDB.Users, 1)
	assert.Equal(t, user.ID, loadedDB.Users[0].ID)

	// Verify user can load databases
	var loadedUser DatabaseUser
	err = db.Preload("Databases").First(&loadedUser, "id = ?", user.ID).Error
	require.NoError(t, err)
	assert.Len(t, loadedUser.Databases, 1)
	assert.Equal(t, database.ID, loadedUser.Databases[0].ID)
}
