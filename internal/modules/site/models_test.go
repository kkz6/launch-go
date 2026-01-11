package site

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Site{}, &models.Deployment{}, &models.Certificate{}, &models.Queue{}, &models.Command{}, &models.Redirect{}, &models.Release{})
	require.NoError(t, err)

	return db
}

// Site tests

func TestSite_TableName(t *testing.T) {
	site := models.Site{}
	assert.Equal(t, "sites", site.TableName())
}

func TestSite_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		site := &models.Site{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
			Address:  "example.com",
		}

		err := db.Create(site).Error
		require.NoError(t, err)

		assert.NotEmpty(t, site.ID)
		assert.Len(t, site.ID, 26)
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existingID := "01ARZ3NDEKTSV4RRFFQ69G5FA1"
		site := &models.Site{
			ID:       existingID,
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
			Address:  "example2.com",
		}

		err := db.Create(site).Error
		require.NoError(t, err)

		assert.Equal(t, existingID, site.ID)
	})

	t.Run("generates deploy token when empty", func(t *testing.T) {
		site := &models.Site{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
			Address:  "example3.com",
		}

		err := db.Create(site).Error
		require.NoError(t, err)

		assert.NotEmpty(t, site.DeployToken)
		assert.Len(t, site.DeployToken, 32)
	})

	t.Run("initializes JSON fields with empty arrays", func(t *testing.T) {
		site := &models.Site{
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
			Address:  "example4.com",
		}

		err := db.Create(site).Error
		require.NoError(t, err)

		assert.Equal(t, "[]", site.SharedDirectories)
		assert.Equal(t, "[]", site.WriteableDirectories)
		assert.Equal(t, "[]", site.SharedFiles)
	})
}

func TestSite_GetURL(t *testing.T) {
	tests := []struct {
		name       string
		tlsSetting enums.TlsSetting
		address    string
		expected   string
	}{
		{
			name:       "HTTPS with auto TLS",
			tlsSetting: enums.TlsSettingAuto,
			address:    "example.com",
			expected:   "https://example.com",
		},
		{
			name:       "HTTPS with custom TLS",
			tlsSetting: enums.TlsSettingCustom,
			address:    "example.com",
			expected:   "https://example.com",
		},
		{
			name:       "HTTPS with internal TLS",
			tlsSetting: enums.TlsSettingInternal,
			address:    "example.com",
			expected:   "https://example.com",
		},
		{
			name:       "HTTP with TLS off",
			tlsSetting: enums.TlsSettingOff,
			address:    "example.com",
			expected:   "http://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &models.Site{
				Address:    tt.address,
				TlsSetting: tt.tlsSetting,
			}
			assert.Equal(t, tt.expected, site.GetURL())
		})
	}
}

func TestSite_GetPort(t *testing.T) {
	tests := []struct {
		name       string
		tlsSetting enums.TlsSetting
		expected   int
	}{
		{
			name:       "port 443 with auto TLS",
			tlsSetting: enums.TlsSettingAuto,
			expected:   443,
		},
		{
			name:       "port 443 with custom TLS",
			tlsSetting: enums.TlsSettingCustom,
			expected:   443,
		},
		{
			name:       "port 80 with TLS off",
			tlsSetting: enums.TlsSettingOff,
			expected:   80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &models.Site{TlsSetting: tt.tlsSetting}
			assert.Equal(t, tt.expected, site.GetPort())
		})
	}
}

func TestSite_StartsWithWww(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{
			name:     "starts with www",
			address:  "www.example.com",
			expected: true,
		},
		{
			name:     "does not start with www",
			address:  "example.com",
			expected: false,
		},
		{
			name:     "www in middle",
			address:  "sub.www.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &models.Site{Address: tt.address}
			assert.Equal(t, tt.expected, site.StartsWithWww())
		})
	}
}

func TestSite_GetLogsDirectory(t *testing.T) {
	site := &models.Site{Path: "/home/user/example.com"}
	assert.Equal(t, "/home/user/example.com/logs", site.GetLogsDirectory())
}

func TestSite_GetApplicationDirectory(t *testing.T) {
	tests := []struct {
		name                   string
		zeroDowntimeDeployment bool
		path                   string
		expected               string
	}{
		{
			name:                   "with zero downtime deployment",
			zeroDowntimeDeployment: true,
			path:                   "/home/user/example.com",
			expected:               "/home/user/example.com/current",
		},
		{
			name:                   "without zero downtime deployment",
			zeroDowntimeDeployment: false,
			path:                   "/home/user/example.com",
			expected:               "/home/user/example.com/repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &models.Site{
				Path:                   tt.path,
				ZeroDowntimeDeployment: tt.zeroDowntimeDeployment,
			}
			assert.Equal(t, tt.expected, site.GetApplicationDirectory())
		})
	}
}

func TestSite_GetWebDirectory(t *testing.T) {
	tests := []struct {
		name                   string
		zeroDowntimeDeployment bool
		path                   string
		webFolder              string
		expected               string
	}{
		{
			name:                   "with zero downtime and public folder",
			zeroDowntimeDeployment: true,
			path:                   "/home/user/example.com",
			webFolder:              "public",
			expected:               "/home/user/example.com/current/public",
		},
		{
			name:                   "without zero downtime and public folder",
			zeroDowntimeDeployment: false,
			path:                   "/home/user/example.com",
			webFolder:              "public",
			expected:               "/home/user/example.com/repository/public",
		},
		{
			name:                   "with root web folder",
			zeroDowntimeDeployment: true,
			path:                   "/home/user/example.com",
			webFolder:              "/",
			expected:               "/home/user/example.com/current",
		},
		{
			name:                   "with trailing slash",
			zeroDowntimeDeployment: true,
			path:                   "/home/user/example.com",
			webFolder:              "public/",
			expected:               "/home/user/example.com/current/public",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &models.Site{
				Path:                   tt.path,
				WebFolder:              tt.webFolder,
				ZeroDowntimeDeployment: tt.zeroDowntimeDeployment,
			}
			assert.Equal(t, tt.expected, site.GetWebDirectory())
		})
	}
}

func TestSite_GenerateWebDirectory(t *testing.T) {
	site := &models.Site{
		Path:                   "/home/user/example.com",
		ZeroDowntimeDeployment: true,
	}

	assert.Equal(t, "/home/user/example.com/current/public", site.GenerateWebDirectory("public"))
	assert.Equal(t, "/home/user/example.com/current/public/assets", site.GenerateWebDirectory("public/assets"))
}

func TestSite_Aliases(t *testing.T) {
	t.Run("GetAliases returns empty array for empty string", func(t *testing.T) {
		site := &models.Site{Aliases: ""}
		assert.Equal(t, []string{}, site.GetAliases())
	})

	t.Run("GetAliases returns empty array for null string", func(t *testing.T) {
		site := &models.Site{Aliases: "null"}
		assert.Equal(t, []string{}, site.GetAliases())
	})

	t.Run("GetAliases returns aliases from JSON", func(t *testing.T) {
		site := &models.Site{Aliases: `["www.example.com","api.example.com"]`}
		expected := []string{"www.example.com", "api.example.com"}
		assert.Equal(t, expected, site.GetAliases())
	})

	t.Run("GetAliases returns empty array for invalid JSON", func(t *testing.T) {
		site := &models.Site{Aliases: "invalid"}
		assert.Equal(t, []string{}, site.GetAliases())
	})

	t.Run("SetAliases sets aliases as JSON", func(t *testing.T) {
		site := &models.Site{}
		err := site.SetAliases([]string{"www.example.com", "api.example.com"})
		require.NoError(t, err)

		assert.Equal(t, `["www.example.com","api.example.com"]`, site.Aliases)
	})

	t.Run("SetAliases handles empty array", func(t *testing.T) {
		site := &models.Site{}
		err := site.SetAliases([]string{})
		require.NoError(t, err)

		assert.Equal(t, "[]", site.Aliases)
	})
}

func TestSite_SharedDirectories(t *testing.T) {
	t.Run("GetSharedDirectories returns empty array for empty string", func(t *testing.T) {
		site := &models.Site{SharedDirectories: ""}
		assert.Equal(t, []string{}, site.GetSharedDirectories())
	})

	t.Run("GetSharedDirectories returns directories from JSON", func(t *testing.T) {
		site := &models.Site{SharedDirectories: `["storage","vendor"]`}
		expected := []string{"storage", "vendor"}
		assert.Equal(t, expected, site.GetSharedDirectories())
	})

	t.Run("SetSharedDirectories sets directories as JSON", func(t *testing.T) {
		site := &models.Site{}
		err := site.SetSharedDirectories([]string{"storage", "vendor"})
		require.NoError(t, err)

		assert.Equal(t, `["storage","vendor"]`, site.SharedDirectories)
	})
}

func TestSite_WriteableDirectories(t *testing.T) {
	t.Run("GetWriteableDirectories returns empty array for empty string", func(t *testing.T) {
		site := &models.Site{WriteableDirectories: ""}
		assert.Equal(t, []string{}, site.GetWriteableDirectories())
	})

	t.Run("GetWriteableDirectories returns directories from JSON", func(t *testing.T) {
		site := &models.Site{WriteableDirectories: `["storage/logs"]`}
		expected := []string{"storage/logs"}
		assert.Equal(t, expected, site.GetWriteableDirectories())
	})

	t.Run("SetWriteableDirectories sets directories as JSON", func(t *testing.T) {
		site := &models.Site{}
		err := site.SetWriteableDirectories([]string{"storage/logs"})
		require.NoError(t, err)

		assert.Equal(t, `["storage/logs"]`, site.WriteableDirectories)
	})
}

func TestSite_SharedFiles(t *testing.T) {
	t.Run("GetSharedFiles returns empty array for empty string", func(t *testing.T) {
		site := &models.Site{SharedFiles: ""}
		assert.Equal(t, []string{}, site.GetSharedFiles())
	})

	t.Run("GetSharedFiles returns files from JSON", func(t *testing.T) {
		site := &models.Site{SharedFiles: `[".env"]`}
		expected := []string{".env"}
		assert.Equal(t, expected, site.GetSharedFiles())
	})

	t.Run("SetSharedFiles sets files as JSON", func(t *testing.T) {
		site := &models.Site{}
		err := site.SetSharedFiles([]string{".env"})
		require.NoError(t, err)

		assert.Equal(t, `[".env"]`, site.SharedFiles)
	})
}

func TestSite_IsInstalled(t *testing.T) {
	t.Run("returns false when InstalledAt is nil", func(t *testing.T) {
		site := &models.Site{}
		assert.False(t, site.IsInstalled())
	})

	t.Run("returns true when InstalledAt is set", func(t *testing.T) {
		now := time.Now()
		site := &models.Site{InstalledAt: &now}
		assert.True(t, site.IsInstalled())
	})
}

func TestSite_HasFeature(t *testing.T) {
	t.Run("returns false for empty features", func(t *testing.T) {
		site := &models.Site{Features: ""}
		assert.False(t, site.HasFeature("database"))
	})

	t.Run("returns true when feature exists", func(t *testing.T) {
		site := &models.Site{Features: `["database","queue"]`}
		assert.True(t, site.HasFeature("database"))
		assert.True(t, site.HasFeature("queue"))
	})

	t.Run("returns false when feature does not exist", func(t *testing.T) {
		site := &models.Site{Features: `["database"]`}
		assert.False(t, site.HasFeature("queue"))
	})
}

func TestSite_GetFeatures(t *testing.T) {
	t.Run("returns empty array for empty string", func(t *testing.T) {
		site := &models.Site{Features: ""}
		assert.Equal(t, []string{}, site.GetFeatures())
	})

	t.Run("returns features from JSON", func(t *testing.T) {
		site := &models.Site{Features: `["database","queue"]`}
		expected := []string{"database", "queue"}
		assert.Equal(t, expected, site.GetFeatures())
	})
}

// Deployment tests

func TestDeployment_TableName(t *testing.T) {
	deployment := models.Deployment{}
	assert.Equal(t, "deployments", deployment.TableName())
}

func TestDeployment_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a site
	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}
	err := db.Create(site).Error
	require.NoError(t, err)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		deployment := &models.Deployment{
			SiteID: site.ID,
			Status: enums.DeploymentStatusPending,
		}

		err := db.Create(deployment).Error
		require.NoError(t, err)

		assert.NotEmpty(t, deployment.ID)
		assert.Len(t, deployment.ID, 26)
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existingID := "01ARZ3NDEKTSV4RRFFQ69G5FA2"
		deployment := &models.Deployment{
			ID:     existingID,
			SiteID: site.ID,
			Status: enums.DeploymentStatusPending,
		}

		err := db.Create(deployment).Error
		require.NoError(t, err)

		assert.Equal(t, existingID, deployment.ID)
	})
}

func TestDeployment_GetShortGitHash(t *testing.T) {
	t.Run("returns empty string when GitHash is nil", func(t *testing.T) {
		deployment := &models.Deployment{}
		assert.Equal(t, "", deployment.GetShortGitHash())
	})

	t.Run("returns full hash when less than 7 characters", func(t *testing.T) {
		hash := "abc"
		deployment := &models.Deployment{GitHash: &hash}
		assert.Equal(t, "abc", deployment.GetShortGitHash())
	})

	t.Run("returns first 7 characters", func(t *testing.T) {
		hash := "abc1234567890def"
		deployment := &models.Deployment{GitHash: &hash}
		assert.Equal(t, "abc1234", deployment.GetShortGitHash())
	})
}

func TestDeployment_CommitData(t *testing.T) {
	t.Run("GetCommitData returns empty map for empty string", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: ""}
		assert.Equal(t, map[string]interface{}{}, deployment.GetCommitData())
	})

	t.Run("GetCommitData returns empty map for null string", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: "null"}
		assert.Equal(t, map[string]interface{}{}, deployment.GetCommitData())
	})

	t.Run("GetCommitData returns data from JSON", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: `{"author":"John","message":"Initial commit"}`}
		data := deployment.GetCommitData()

		assert.Equal(t, "John", data["author"])
		assert.Equal(t, "Initial commit", data["message"])
	})

	t.Run("GetCommitData returns empty map for invalid JSON", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: "invalid"}
		assert.Equal(t, map[string]interface{}{}, deployment.GetCommitData())
	})

	t.Run("SetCommitData sets data as JSON", func(t *testing.T) {
		deployment := &models.Deployment{}
		err := deployment.SetCommitData(map[string]interface{}{
			"author":  "John",
			"message": "Initial commit",
		})
		require.NoError(t, err)

		// Verify it can be read back
		data := deployment.GetCommitData()
		assert.Equal(t, "John", data["author"])
	})
}

func TestDeployment_IsRollback(t *testing.T) {
	t.Run("returns false for empty commit data", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: ""}
		assert.False(t, deployment.IsRollback())
	})

	t.Run("returns false when only rollback_from exists", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: `{"rollback_from":"abc123"}`}
		assert.False(t, deployment.IsRollback())
	})

	t.Run("returns false when only rollback_to exists", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: `{"rollback_to":"abc123"}`}
		assert.False(t, deployment.IsRollback())
	})

	t.Run("returns true when both rollback fields exist", func(t *testing.T) {
		deployment := &models.Deployment{CommitData: `{"rollback_from":"abc123","rollback_to":"def456"}`}
		assert.True(t, deployment.IsRollback())
	})
}

// Certificate tests

func TestCertificate_TableName(t *testing.T) {
	cert := models.Certificate{}
	assert.Equal(t, "certificates", cert.TableName())
}

func TestCertificate_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a site
	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}
	err := db.Create(site).Error
	require.NoError(t, err)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		cert := &models.Certificate{
			SiteID: site.ID,
			Type:   enums.CertificateTypeAuto,
		}

		err := db.Create(cert).Error
		require.NoError(t, err)

		assert.NotEmpty(t, cert.ID)
		assert.Len(t, cert.ID, 26)
	})
}

func TestCertificate_SiteDirectory(t *testing.T) {
	cert := &models.Certificate{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	expected := "/home/user/example.com/certificates/01ARZ3NDEKTSV4RRFFQ69G5FAV"
	assert.Equal(t, expected, cert.SiteDirectory("/home/user/example.com"))
}

func TestCertificate_CertificatePath(t *testing.T) {
	cert := &models.Certificate{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	expected := "/home/user/example.com/certificates/01ARZ3NDEKTSV4RRFFQ69G5FAV/certificate.cert"
	assert.Equal(t, expected, cert.CertificatePath("/home/user/example.com"))
}

func TestCertificate_PrivateKeyPath(t *testing.T) {
	cert := &models.Certificate{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	expected := "/home/user/example.com/certificates/01ARZ3NDEKTSV4RRFFQ69G5FAV/private.key"
	assert.Equal(t, expected, cert.PrivateKeyPath("/home/user/example.com"))
}

func TestCertificate_Domains(t *testing.T) {
	t.Run("GetDomains returns empty array for empty string", func(t *testing.T) {
		cert := &models.Certificate{Domains: ""}
		assert.Equal(t, []string{}, cert.GetDomains())
	})

	t.Run("GetDomains returns domains from JSON", func(t *testing.T) {
		cert := &models.Certificate{Domains: `["example.com","www.example.com"]`}
		expected := []string{"example.com", "www.example.com"}
		assert.Equal(t, expected, cert.GetDomains())
	})

	t.Run("SetDomains sets domains as JSON", func(t *testing.T) {
		cert := &models.Certificate{}
		err := cert.SetDomains([]string{"example.com", "www.example.com"})
		require.NoError(t, err)

		assert.Equal(t, `["example.com","www.example.com"]`, cert.Domains)
	})
}

// Queue tests

func TestQueue_TableName(t *testing.T) {
	queue := models.Queue{}
	assert.Equal(t, "queues", queue.TableName())
}

func TestQueue_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a site
	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}
	err := db.Create(site).Error
	require.NoError(t, err)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		queue := &models.Queue{
			SiteID:   site.ID,
			ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		}

		err := db.Create(queue).Error
		require.NoError(t, err)

		assert.NotEmpty(t, queue.ID)
		assert.Len(t, queue.ID, 26)
	})
}

func TestQueue_GetPath(t *testing.T) {
	queue := &models.Queue{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	expected := "/etc/supervisor/conf.d/daemon-01ARZ3NDEKTSV4RRFFQ69G5FAV.conf"
	assert.Equal(t, expected, queue.GetPath())
}

func TestQueue_ErrorLogPath(t *testing.T) {
	t.Run("returns root path for root user", func(t *testing.T) {
		queue := &models.Queue{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", User: "root"}
		expected := "/root/logs/daemon-01ARZ3NDEKTSV4RRFFQ69G5FAV.err"
		assert.Equal(t, expected, queue.ErrorLogPath("logs"))
	})

	t.Run("returns home path for regular user", func(t *testing.T) {
		queue := &models.Queue{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", User: "deploy"}
		expected := "/home/deploy/logs/daemon-01ARZ3NDEKTSV4RRFFQ69G5FAV.err"
		assert.Equal(t, expected, queue.ErrorLogPath("logs"))
	})
}

func TestQueue_OutputLogPath(t *testing.T) {
	t.Run("returns root path for root user", func(t *testing.T) {
		queue := &models.Queue{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", User: "root"}
		expected := "/root/logs/daemon-01ARZ3NDEKTSV4RRFFQ69G5FAV.log"
		assert.Equal(t, expected, queue.OutputLogPath("logs"))
	})

	t.Run("returns home path for regular user", func(t *testing.T) {
		queue := &models.Queue{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", User: "deploy"}
		expected := "/home/deploy/logs/daemon-01ARZ3NDEKTSV4RRFFQ69G5FAV.log"
		assert.Equal(t, expected, queue.OutputLogPath("logs"))
	})
}

func TestQueue_BuildCommand(t *testing.T) {
	t.Run("builds basic command", func(t *testing.T) {
		queue := &models.Queue{
			QueueConnection:    "database",
			QueueName:          "default",
			RestSecondsOnEmpty: 3,
			MaxSecondsPerJob:   60,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "php artisan queue:work database --queue=default")
		assert.Contains(t, cmd, "--sleep=3")
		assert.Contains(t, cmd, "--timeout=60")
	})

	t.Run("builds command with listen", func(t *testing.T) {
		queue := &models.Queue{
			QueueConnection:    "redis",
			QueueName:          "high",
			RunWithListen:      true,
			RestSecondsOnEmpty: 5,
			MaxSecondsPerJob:   120,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "php artisan queue:listen redis --queue=high")
	})

	t.Run("includes tries option", func(t *testing.T) {
		queue := &models.Queue{
			QueueConnection:    "database",
			QueueName:          "default",
			MaxTries:           3,
			RestSecondsOnEmpty: 3,
			MaxSecondsPerJob:   60,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "--tries=3")
	})

	t.Run("includes environment option", func(t *testing.T) {
		env := "production"
		queue := &models.Queue{
			QueueConnection:    "database",
			QueueName:          "default",
			Environment:        &env,
			RestSecondsOnEmpty: 3,
			MaxSecondsPerJob:   60,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "--env=production")
	})

	t.Run("includes backoff option", func(t *testing.T) {
		queue := &models.Queue{
			QueueConnection:       "database",
			QueueName:             "default",
			FailedJobDelaySeconds: 10,
			RestSecondsOnEmpty:    3,
			MaxSecondsPerJob:      60,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "--backoff=10")
	})

	t.Run("includes force option for maintenance mode", func(t *testing.T) {
		queue := &models.Queue{
			QueueConnection:    "database",
			QueueName:          "default",
			RunOnMaintenance:   true,
			RestSecondsOnEmpty: 3,
			MaxSecondsPerJob:   60,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "--force")
	})

	t.Run("includes memory option", func(t *testing.T) {
		queue := &models.Queue{
			QueueConnection:    "database",
			QueueName:          "default",
			MaxMemory:          256,
			RestSecondsOnEmpty: 3,
			MaxSecondsPerJob:   60,
		}

		cmd := queue.BuildCommand()
		assert.Contains(t, cmd, "--memory=256")
	})
}

func TestQueue_IsInstalled(t *testing.T) {
	t.Run("returns false when InstalledAt is nil", func(t *testing.T) {
		queue := &models.Queue{}
		assert.False(t, queue.IsInstalled())
	})

	t.Run("returns true when InstalledAt is set", func(t *testing.T) {
		now := time.Now()
		queue := &models.Queue{InstalledAt: &now}
		assert.True(t, queue.IsInstalled())
	})
}

// Command tests

func TestCommand_TableName(t *testing.T) {
	cmd := models.Command{}
	assert.Equal(t, "commands", cmd.TableName())
}

func TestCommand_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a site
	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}
	err := db.Create(site).Error
	require.NoError(t, err)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		cmd := &models.Command{
			SiteID:  site.ID,
			UserID:  "01ARZ3NDEKTSV4RRFFQ69G5FAU",
			Command: "php artisan migrate",
		}

		err := db.Create(cmd).Error
		require.NoError(t, err)

		assert.NotEmpty(t, cmd.ID)
		assert.Len(t, cmd.ID, 26)
	})
}

// Redirect tests

func TestRedirect_TableName(t *testing.T) {
	redirect := models.Redirect{}
	assert.Equal(t, "redirects", redirect.TableName())
}

func TestRedirect_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a site
	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}
	err := db.Create(site).Error
	require.NoError(t, err)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		redirect := &models.Redirect{
			SiteID: site.ID,
			UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU",
			Mode:   enums.RedirectModePermanent,
			From:   "/old",
			To:     "/new",
		}

		err := db.Create(redirect).Error
		require.NoError(t, err)

		assert.NotEmpty(t, redirect.ID)
		assert.Len(t, redirect.ID, 26)
	})
}

// Release tests

func TestRelease_TableName(t *testing.T) {
	release := models.Release{}
	assert.Equal(t, "releases", release.TableName())
}

func TestRelease_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a site
	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}
	err := db.Create(site).Error
	require.NoError(t, err)

	t.Run("generates ULID when ID is empty", func(t *testing.T) {
		release := &models.Release{
			SiteID: site.ID,
			Path:   "/home/user/example.com/releases/20240101120000",
		}

		err := db.Create(release).Error
		require.NoError(t, err)

		assert.NotEmpty(t, release.ID)
		assert.Len(t, release.ID, 26)
	})
}

// Helper function tests

func TestGenerateRandomToken(t *testing.T) {
	t.Run("generates token of specified length", func(t *testing.T) {
		token := models.GenerateRandomToken(32)
		assert.Len(t, token, 32)
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		token1 := models.GenerateRandomToken(32)
		token2 := models.GenerateRandomToken(32)
		assert.NotEqual(t, token1, token2)
	})
}

func TestGenerateAppKey(t *testing.T) {
	t.Run("generates key with base64 prefix", func(t *testing.T) {
		key := models.GenerateAppKey()
		assert.True(t, len(key) > 0)
		assert.Contains(t, key, "base64:")
	})

	t.Run("generates unique keys", func(t *testing.T) {
		key1 := models.GenerateAppKey()
		key2 := models.GenerateAppKey()
		assert.NotEqual(t, key1, key2)
	})
}
