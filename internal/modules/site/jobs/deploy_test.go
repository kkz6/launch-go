package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
)

func TestNewDeployTask(t *testing.T) {
	siteID := "site123"
	deploymentID := "deploy456"
	userID := "user789"

	task, err := NewDeployTask(siteID, deploymentID, userID)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeDeploy, task.Type())
}

func TestNewDeployTask_EmptyUserID(t *testing.T) {
	siteID := "site123"
	deploymentID := "deploy456"

	task, err := NewDeployTask(siteID, deploymentID, "")

	require.NoError(t, err)
	assert.NotNil(t, task)
}

func TestNewDeployZeroDowntimeTask(t *testing.T) {
	siteID := "site123"
	deploymentID := "deploy456"
	userID := "user789"

	task, err := NewDeployZeroDowntimeTask(siteID, deploymentID, userID)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeDeployZeroDowntime, task.Type())
}

func TestDeployPayload(t *testing.T) {
	userID := "user123"
	payload := DeployPayload{
		SiteID:       "site456",
		DeploymentID: "deploy789",
		UserID:       &userID,
	}

	assert.Equal(t, "site456", payload.SiteID)
	assert.Equal(t, "deploy789", payload.DeploymentID)
	assert.NotNil(t, payload.UserID)
	assert.Equal(t, "user123", *payload.UserID)
}

func TestDeployJob_Type(t *testing.T) {
	job := &DeployJob{}
	assert.Equal(t, TypeDeploy, job.Type())
}

func TestDeployZeroDowntimeJob_Type(t *testing.T) {
	job := &DeployZeroDowntimeJob{}
	assert.Equal(t, TypeDeployZeroDowntime, job.Type())
}

func TestDeployJob_BuildDeployConfig(t *testing.T) {
	phpVersion := "8.2"
	branch := "main"
	hookBefore := "composer install"
	hookAfter := "artisan cache:clear"

	site := &models.Site{
		Path:                         "/home/user/site.com",
		Type:                         enums.SiteTypeLaravel,
		Address:                      "site.com",
		User:                         "siteuser",
		PhpVersion:                   &phpVersion,
		RepositoryBranch:             &branch,
		ZeroDowntimeDeployment:       false,
		DeploymentReleasesRetention:  5,
		HookBeforeUpdatingRepository: &hookBefore,
		HookAfterUpdatingRepository:  &hookAfter,
		SharedDirectories:            []string{"storage"},
		SharedFiles:                  []string{".env"},
		WriteableDirectories:         []string{"bootstrap/cache"},
	}

	deployment := &models.Deployment{
		Status: enums.DeploymentStatusPending,
	}
	deployment.ID = "deploy123"

	job := &DeployJob{}
	config := job.buildDeployConfig(site, deployment)

	assert.Equal(t, site.Path, config.SitePath)
	assert.Equal(t, tasks.SiteType(site.Type), config.SiteType)
	assert.Equal(t, "site.com", config.SiteAddress)
	assert.Equal(t, "siteuser", config.Username)
	assert.Equal(t, "php8.2", config.PHPBinary)
	assert.Equal(t, "main", config.RepositoryBranch)
	assert.False(t, config.ZeroDowntimeDeployment)
	assert.Equal(t, "deploy123", config.DeploymentID)
	assert.Equal(t, hookBefore, config.HookBeforeUpdatingRepository)
	assert.Equal(t, hookAfter, config.HookAfterUpdatingRepository)
	assert.Equal(t, []string{"storage"}, config.SharedDirectories)
	assert.Equal(t, []string{".env"}, config.SharedFiles)
	assert.Equal(t, 5, config.RetentionCount)
}

func TestDeployZeroDowntimeJob_BuildDeployConfig(t *testing.T) {
	phpVersion := "8.2"
	branch := "main"

	site := &models.Site{
		Path:                        "/home/user/site.com",
		Type:                        enums.SiteTypeLaravel,
		Address:                     "site.com",
		User:                        "siteuser",
		PhpVersion:                  &phpVersion,
		RepositoryBranch:            &branch,
		ZeroDowntimeDeployment:      true,
		DeploymentReleasesRetention: 10,
		SharedDirectories:           []string{"storage"},
		SharedFiles:                 []string{".env"},
		WriteableDirectories:        []string{"bootstrap/cache"},
	}

	deployment := &models.Deployment{
		Status: enums.DeploymentStatusPending,
	}
	deployment.ID = "deploy123"

	job := &DeployZeroDowntimeJob{}
	config := job.buildDeployConfig(site, deployment)

	assert.Equal(t, site.Path, config.SitePath)
	assert.True(t, config.ZeroDowntimeDeployment)
	assert.Contains(t, config.SharedDirectory, "/shared")
	assert.Contains(t, config.ReleasesDirectory, "/releases")
	assert.Contains(t, config.ReleaseDirectory, "/releases/")
	assert.Contains(t, config.CurrentDirectory, "/current")
	assert.Equal(t, 10, config.RetentionCount)
}

func TestDeployJob_BuildDeployConfig_DefaultBranch(t *testing.T) {
	site := &models.Site{
		Path:    "/home/user/site.com",
		Type:    enums.SiteTypeLaravel,
		Address: "site.com",
		User:    "siteuser",
	}

	deployment := &models.Deployment{}
	deployment.ID = "deploy123"

	job := &DeployJob{}
	config := job.buildDeployConfig(site, deployment)

	// When RepositoryBranch is nil, should default to "main"
	assert.Equal(t, "main", config.RepositoryBranch)
}

func TestDeployJob_BuildDeployConfig_DefaultPhpBinary(t *testing.T) {
	site := &models.Site{
		Path:    "/home/user/site.com",
		Type:    enums.SiteTypeLaravel,
		Address: "site.com",
		User:    "siteuser",
	}

	deployment := &models.Deployment{}
	deployment.ID = "deploy123"

	job := &DeployJob{}
	config := job.buildDeployConfig(site, deployment)

	// When PhpVersion is nil, should default to "php"
	assert.Equal(t, "php", config.PHPBinary)
}

func TestDeployJob_BuildDeployConfig_InstalledAt(t *testing.T) {
	now := time.Now()
	site := &models.Site{
		Path:    "/home/user/site.com",
		Type:    enums.SiteTypeLaravel,
		Address: "site.com",
		User:    "siteuser",
	}
	site.InstalledAt = &now

	deployment := &models.Deployment{}
	deployment.ID = "deploy123"

	job := &DeployJob{}
	config := job.buildDeployConfig(site, deployment)

	// InstalledAt should be true
	assert.True(t, config.InstalledAt)
}

func TestNewRollbackTask(t *testing.T) {
	siteID := "site123"
	deploymentID := "deploy456"
	targetDeploymentID := "deploy123"
	userID := "user789"

	task, err := NewRollbackTask(siteID, deploymentID, targetDeploymentID, userID)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, TypeRollback, task.Type())
}

func TestRollbackPayload(t *testing.T) {
	userID := "user123"
	payload := RollbackPayload{
		SiteID:             "site456",
		DeploymentID:       "deploy789",
		TargetDeploymentID: "target123",
		UserID:             &userID,
	}

	assert.Equal(t, "site456", payload.SiteID)
	assert.Equal(t, "deploy789", payload.DeploymentID)
	assert.Equal(t, "target123", payload.TargetDeploymentID)
	assert.Equal(t, "user123", *payload.UserID)
}

func TestStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: "",
		},
		{
			name: "non-empty string",
			input: func() *string {
				s := "hello"
				return &s
			}(),
			expected: "hello",
		},
		{
			name: "empty string",
			input: func() *string {
				s := ""
				return &s
			}(),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
