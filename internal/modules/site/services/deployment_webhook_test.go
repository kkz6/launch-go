package services

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func setupWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Site{}, &models.Deployment{})
	require.NoError(t, err)

	return db
}

func newTestDeploymentService(db *gorm.DB) *DeploymentService {
	logger := zerolog.Nop()
	repos := repositories.NewRegistry(db)

	deps := &ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: service.Dependencies{
				Logger: &logger,
			},
			Repos: repos,
		},
	}

	return NewDeploymentService(deps)
}

func createTestSite(t *testing.T, db *gorm.DB, deployToken *string, branch *string) *models.Site {
	t.Helper()

	site := &models.Site{
		Address:          "example.com",
		Type:             sitetypes.SiteTypeLaravel,
		TLSSetting:       sitetypes.TLSSettingAuto,
		DeployToken:      deployToken,
		RepositoryBranch: branch,
		User:             "launch",
		Path:             "/home/launch/example.com",
		WebFolder:        "/public",
	}
	site.TeamID = "team-1"
	site.ServerID = "server-1"
	site.UserID = "user-1"

	err := db.Create(site).Error
	require.NoError(t, err)

	return site
}

func TestDeployFromWebhook_ValidToken(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	token := "abc123token"
	site := createTestSite(t, db, &token, nil)

	err := svc.DeployFromWebhook(context.Background(), site.ID, token, nil)
	assert.NoError(t, err)

	// Verify a deployment was created
	var count int64
	db.Model(&models.Deployment{}).Where("site_id = ?", site.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestDeployFromWebhook_NilDeployToken(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	site := createTestSite(t, db, nil, nil)

	err := svc.DeployFromWebhook(context.Background(), site.ID, "anytoken", nil)
	require.Error(t, err)

	var fiberErr *fiber.Error
	require.True(t, errors.As(err, &fiberErr))
	assert.Equal(t, fiber.StatusUnauthorized, fiberErr.Code)
}

func TestDeployFromWebhook_WrongToken(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	token := "correct-token"
	site := createTestSite(t, db, &token, nil)

	err := svc.DeployFromWebhook(context.Background(), site.ID, "wrong-token", nil)
	require.Error(t, err)

	var fiberErr *fiber.Error
	require.True(t, errors.As(err, &fiberErr))
	assert.Equal(t, fiber.StatusUnauthorized, fiberErr.Code)
}

func TestDeployFromWebhook_EmptyToken(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	token := "valid-token"
	site := createTestSite(t, db, &token, nil)

	err := svc.DeployFromWebhook(context.Background(), site.ID, "", nil)
	require.Error(t, err)

	var fiberErr *fiber.Error
	require.True(t, errors.As(err, &fiberErr))
	assert.Equal(t, fiber.StatusUnauthorized, fiberErr.Code)
}

func TestDeployFromWebhook_BranchMismatch(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	token := "valid-token"
	branch := "production"
	site := createTestSite(t, db, &token, &branch)

	payload := map[string]any{
		"ref": "refs/heads/develop",
		"head_commit": map[string]any{
			"id":      "abc123def456",
			"message": "test commit",
		},
	}

	err := svc.DeployFromWebhook(context.Background(), site.ID, token, payload)
	assert.ErrorIs(t, err, ErrBranchMismatch)
}

func TestDeployFromWebhook_BranchMatch(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	token := "valid-token"
	branch := "main"
	site := createTestSite(t, db, &token, &branch)

	payload := map[string]any{
		"ref": "refs/heads/main",
		"head_commit": map[string]any{
			"id":      "abc123def456",
			"message": "test commit",
		},
	}

	err := svc.DeployFromWebhook(context.Background(), site.ID, token, payload)
	assert.NoError(t, err)
}

func TestDeployFromWebhook_SiteNotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	err := svc.DeployFromWebhook(context.Background(), "nonexistent-id", "token", nil)
	assert.Error(t, err)
}

func TestParseWebhookPayload_GitHub(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	site := &models.Site{}
	payload := map[string]any{
		"ref": "refs/heads/main",
		"head_commit": map[string]any{
			"id":      "abc123def456789",
			"message": "feat: add new feature",
			"url":     "https://github.com/user/repo/commit/abc123",
			"author": map[string]any{
				"name":  "Test Author",
				"email": "test@example.com",
			},
		},
	}

	result := svc.parseWebhookPayload(payload, site)

	require.NotNil(t, result)
	assert.Equal(t, "abc123def456789", result["commit_id"])
	assert.Equal(t, "abc123d", result["sha"])
	assert.Equal(t, "Test Author", result["name"])
	assert.Equal(t, "test@example.com", result["email"])
	assert.Equal(t, "feat: add new feature", result["message"])
	assert.Equal(t, "https://github.com/user/repo/commit/abc123", result["url"])
	assert.Equal(t, "main", result["branch"])
}

func TestParseWebhookPayload_GitLab(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	site := &models.Site{}
	payload := map[string]any{
		"ref": "refs/heads/develop",
		"commits": []any{
			map[string]any{
				"id":      "def456abc789012",
				"message": "fix: resolve bug",
				"url":     "https://gitlab.com/user/repo/-/commit/def456",
				"author": map[string]any{
					"name":  "GitLab User",
					"email": "gitlab@example.com",
				},
			},
		},
	}

	result := svc.parseWebhookPayload(payload, site)

	require.NotNil(t, result)
	assert.Equal(t, "def456abc789012", result["commit_id"])
	assert.Equal(t, "def456a", result["sha"])
	assert.Equal(t, "GitLab User", result["name"])
	assert.Equal(t, "gitlab@example.com", result["email"])
	assert.Equal(t, "fix: resolve bug", result["message"])
	assert.Equal(t, "develop", result["branch"])
}

func TestParseWebhookPayload_Bitbucket(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	site := &models.Site{}
	payload := map[string]any{
		"push": map[string]any{
			"changes": []any{
				map[string]any{
					"new": map[string]any{
						"name": "feature-branch",
					},
					"commits": []any{
						map[string]any{
							"hash":    "789abc012def345",
							"message": "chore: update deps",
							"author": map[string]any{
								"raw": "BB User <bb@example.com>",
							},
							"links": map[string]any{
								"html": map[string]any{
									"href": "https://bitbucket.org/user/repo/commits/789abc",
								},
							},
						},
					},
				},
			},
		},
	}

	result := svc.parseWebhookPayload(payload, site)

	require.NotNil(t, result)
	assert.Equal(t, "789abc012def345", result["commit_id"])
	assert.Equal(t, "789abc0", result["sha"])
	assert.Equal(t, "BB User <bb@example.com>", result["name"])
	assert.Equal(t, "chore: update deps", result["message"])
	assert.Equal(t, "https://bitbucket.org/user/repo/commits/789abc", result["url"])
	assert.Equal(t, "feature-branch", result["branch"])
}

func TestParseWebhookPayload_NilPayload(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	result := svc.parseWebhookPayload(nil, &models.Site{})
	assert.Nil(t, result)
}

func TestParseWebhookPayload_EmptyPayload(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	result := svc.parseWebhookPayload(map[string]any{}, &models.Site{})
	assert.Nil(t, result)
}

func TestDeployFromWebhook_DefaultBranchIsMain(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)

	token := "valid-token"
	// No branch set — should default to "main"
	site := createTestSite(t, db, &token, nil)

	payload := map[string]any{
		"ref": "refs/heads/develop",
		"head_commit": map[string]any{
			"id": "abc123",
		},
	}

	err := svc.DeployFromWebhook(context.Background(), site.ID, token, payload)
	assert.ErrorIs(t, err, ErrBranchMismatch)
}
