package registryaccess

import (
	"context"
	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strconv"
	"testing"
	"time"
)

func TestCredentialReferenceIsEncryptedAndBoundToDeployment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RegistryLease{}))
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	dep := &models.Deployment{TargetType: "application", TargetID: "app"}
	dep.ID = "deployment"
	dep.TeamID = "account-a"
	snapshot := &access.Snapshot{ConnectionID: "connection", ConnectionGeneration: 1, RepositoryID: "1", GrantVersion: 1, AuthorizationID: "auth", AuthorizationGeneration: 1, InstallationGeneration: 1}
	now := time.Now()
	id, err := Create(context.Background(), db, dep, "ghcr.io/acme/app", "very-secret-token", strconv.FormatInt(now.Unix(), 10), strconv.FormatInt(now.Add(5*time.Minute).Unix(), 10), snapshot)
	require.NoError(t, err)
	var stored string
	require.NoError(t, db.Table("docker_registry_leases").Select("token").Where("id = ?", id).Scan(&stored).Error)
	require.NotContains(t, stored, "very-secret-token")
	token, err := Redeem(context.Background(), db, id, dep, "ghcr.io/acme/app", snapshot)
	require.NoError(t, err)
	require.Equal(t, "very-secret-token", token)
	for _, scenario := range []string{"team", "deployment", "package", "generation", "expiry"} {
		t.Run(scenario, func(t *testing.T) {
			changed := *dep
			pinned := *snapshot
			pkg := "ghcr.io/acme/app"
			switch scenario {
			case "team":
				changed.TeamID = "account-b"
			case "deployment":
				changed.ID = "other"
			case "package":
				pkg = "ghcr.io/acme/other"
			case "generation":
				pinned.ConnectionGeneration++
			case "expiry":
				require.NoError(t, db.Model(&models.RegistryLease{}).Where("id = ?", id).Update("expires_at", time.Now()).Error)
			}
			_, err := Redeem(context.Background(), db, id, &changed, pkg, &pinned)
			require.Error(t, err)
		})
	}
	_, err = Create(context.Background(), db, dep, "ghcr.io/acme/app", "token", strconv.FormatInt(now.Unix(), 10), "", snapshot)
	require.Error(t, err)
}
