package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

func setupDeploymentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Deployment{}))
	return db
}

func makeDeployment(
	t *testing.T, db *gorm.DB, targetType, targetID string,
	status dockertypes.DeploymentStatus, createdAt time.Time,
) *models.Deployment {
	t.Helper()
	d := &models.Deployment{
		TargetType: targetType,
		TargetID:   targetID,
		Status:     status,
	}
	d.ID = util.NewULID()
	d.TeamID = "team-a"
	d.ServerID = "srv-a"
	d.CreatedAt = &createdAt
	d.UpdatedAt = &createdAt
	require.NoError(t, db.Create(d).Error)
	return d
}

func TestSupersedeInProgressForTarget(t *testing.T) {
	db := setupDeploymentDB(t)
	repo := NewDeploymentRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	pending := makeDeployment(t, db, "application", "app-1", dockertypes.DeploymentStatusPending, now)
	building := makeDeployment(t, db, "application", "app-1", dockertypes.DeploymentStatusBuilding, now)
	deploying := makeDeployment(t, db, "application", "app-1", dockertypes.DeploymentStatusDeploying, now)
	success := makeDeployment(t, db, "application", "app-1", dockertypes.DeploymentStatusSuccess, now)
	failed := makeDeployment(t, db, "application", "app-1", dockertypes.DeploymentStatusFailed, now)
	// A different target must be untouched.
	otherTarget := makeDeployment(t, db, "application", "app-2", dockertypes.DeploymentStatusPending, now)

	n, err := repo.SupersedeInProgressForTarget(ctx, "application", "app-1")
	require.NoError(t, err)
	assert.Equal(t, int64(3), n, "pending+building+deploying should be superseded")

	reload := func(id string) models.Deployment {
		var d models.Deployment
		require.NoError(t, db.Where("id = ?", id).First(&d).Error)
		return d
	}

	for _, id := range []string{pending.ID, building.ID, deploying.ID} {
		d := reload(id)
		assert.Equal(t, dockertypes.DeploymentStatusCancelled, d.Status)
		require.NotNil(t, d.FinishedAt)
		require.NotNil(t, d.Error)
		assert.Equal(t, "Superseded by a newer deployment", *d.Error)
	}
	// Terminal rows and other targets stay as they were.
	assert.Equal(t, dockertypes.DeploymentStatusSuccess, reload(success.ID).Status)
	assert.Equal(t, dockertypes.DeploymentStatusFailed, reload(failed.ID).Status)
	assert.Equal(t, dockertypes.DeploymentStatusPending, reload(otherTarget.ID).Status)
}

func TestPruneForTarget_KeepsNewestN(t *testing.T) {
	db := setupDeploymentDB(t)
	repo := NewDeploymentRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// 12 rows, oldest first.
	ids := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		d := makeDeployment(t, db, "application", "app-1",
			dockertypes.DeploymentStatusSuccess, base.Add(time.Duration(i)*time.Minute))
		ids = append(ids, d.ID)
	}
	// A different target shouldn't be pruned by app-1's call.
	other := makeDeployment(t, db, "application", "app-2", dockertypes.DeploymentStatusSuccess, base)

	require.NoError(t, repo.PruneForTarget(ctx, "application", "app-1", 10))

	var remaining []models.Deployment
	require.NoError(t, db.Where("target_type = ? AND target_id = ?", "application", "app-1").
		Order("created_at ASC").Find(&remaining).Error)
	require.Len(t, remaining, 10, "should keep the newest 10")
	// The two oldest (ids[0], ids[1]) must be gone.
	assert.Equal(t, ids[2], remaining[0].ID)
	assert.Equal(t, ids[11], remaining[9].ID)

	// Other target untouched.
	var otherCount int64
	require.NoError(t, db.Model(&models.Deployment{}).
		Where("id = ?", other.ID).Count(&otherCount).Error)
	assert.Equal(t, int64(1), otherCount)
}

func TestPruneForTarget_NoOpWhenUnderCap(t *testing.T) {
	db := setupDeploymentDB(t)
	repo := NewDeploymentRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		makeDeployment(t, db, "application", "app-1",
			dockertypes.DeploymentStatusSuccess, base.Add(time.Duration(i)*time.Minute))
	}
	require.NoError(t, repo.PruneForTarget(ctx, "application", "app-1", 10))
	require.NoError(t, repo.PruneForTarget(ctx, "application", "app-1", 0)) // no-op guard

	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).
		Where("target_id = ?", "app-1").Count(&count).Error)
	assert.Equal(t, int64(5), count)
}

func TestCreate_EnforcesHistoryLimitPerTarget(t *testing.T) {
	db := setupDeploymentDB(t)
	repo := NewDeploymentRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	ids := make([]string, 0, DeploymentHistoryLimit+1)
	for i := 0; i < DeploymentHistoryLimit+1; i++ {
		d := &models.Deployment{
			TargetType: "application",
			TargetID:   "app-1",
			Status:     dockertypes.DeploymentStatusSuccess,
		}
		d.ID = util.NewULID()
		d.TeamID = "team-a"
		d.ServerID = "srv-a"
		createdAt := base.Add(time.Duration(i) * time.Minute)
		d.CreatedAt = &createdAt
		d.UpdatedAt = &createdAt
		require.NoError(t, repo.Create(ctx, d))
		ids = append(ids, d.ID)
	}

	other := &models.Deployment{
		TargetType: "application",
		TargetID:   "app-2",
		Status:     dockertypes.DeploymentStatusSuccess,
	}
	other.ID = util.NewULID()
	other.TeamID = "team-a"
	other.ServerID = "srv-a"
	require.NoError(t, repo.Create(ctx, other))

	var remaining []models.Deployment
	require.NoError(t, db.Where("target_type = ? AND target_id = ?", "application", "app-1").
		Order("created_at ASC").Find(&remaining).Error)
	require.Len(t, remaining, DeploymentHistoryLimit)
	assert.Equal(t, ids[1], remaining[0].ID, "the oldest deployment should be removed")
	assert.Equal(t, ids[DeploymentHistoryLimit], remaining[DeploymentHistoryLimit-1].ID)

	var otherCount int64
	require.NoError(t, db.Model(&models.Deployment{}).Where("id = ?", other.ID).Count(&otherCount).Error)
	assert.Equal(t, int64(1), otherCount, "another workload's history must be untouched")
}

func TestCreate_RollsBackWhenInsertFails(t *testing.T) {
	db := setupDeploymentDB(t)
	repo := NewDeploymentRepository(db)
	ctx := context.Background()

	existing := makeDeployment(t, db, "application", "app-1", dockertypes.DeploymentStatusSuccess, time.Now())
	duplicate := &models.Deployment{
		TargetType: "application",
		TargetID:   "app-1",
		Status:     dockertypes.DeploymentStatusPending,
	}
	duplicate.ID = existing.ID
	duplicate.TeamID = "team-a"
	duplicate.ServerID = "srv-a"

	require.Error(t, repo.Create(ctx, duplicate))

	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCreate_RollsBackWhenPruningFails(t *testing.T) {
	db := setupDeploymentDB(t)
	repo := NewDeploymentRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	for i := 0; i < DeploymentHistoryLimit; i++ {
		makeDeployment(t, db, "application", "app-1",
			dockertypes.DeploymentStatusSuccess, base.Add(time.Duration(i)*time.Minute))
	}
	require.NoError(t, db.Exec(`
		CREATE TRIGGER prevent_deployment_prune
		BEFORE DELETE ON docker_deployments
		BEGIN
			SELECT RAISE(FAIL, 'pruning disabled');
		END
	`).Error)

	next := &models.Deployment{
		TargetType: "application",
		TargetID:   "app-1",
		Status:     dockertypes.DeploymentStatusPending,
	}
	next.ID = util.NewULID()
	next.TeamID = "team-a"
	next.ServerID = "srv-a"
	createdAt := base.Add(DeploymentHistoryLimit * time.Minute)
	next.CreatedAt = &createdAt
	next.UpdatedAt = &createdAt

	require.ErrorContains(t, repo.Create(ctx, next), "pruning disabled")

	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).Count(&count).Error)
	assert.Equal(t, int64(DeploymentHistoryLimit), count, "the insert must roll back with the failed prune")
}
