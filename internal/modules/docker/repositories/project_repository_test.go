package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// setupDB returns an in-memory sqlite DB with just the projects table.
// We don't need the workload tables for these tests — every method on
// ProjectRepository that touches counts already swallows errors when the
// table is absent, which is what fillCounts is documented to do.
func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Project{}))
	return db
}

// makeProject inserts a project with deterministic team/server IDs so the
// scoping tests can assert "wrong team blocks the read".
func makeProject(t *testing.T, db *gorm.DB, teamID, serverID, name string) *models.Project {
	t.Helper()
	p := &models.Project{Name: name}
	p.ID = util.NewULID()
	p.TeamID = teamID
	p.ServerID = serverID
	now := time.Now().UTC()
	p.CreatedAt = &now
	p.UpdatedAt = &now
	require.NoError(t, db.Create(p).Error)
	return p
}

func TestProjectRepository_FindByIDAndTeamServer(t *testing.T) {
	db := setupDB(t)
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p := makeProject(t, db, "team-a", "srv-a", "api")

	t.Run("found", func(t *testing.T) {
		got, err := repo.FindByIDAndTeamServer(ctx, p.ID, "team-a", "srv-a")
		require.NoError(t, err)
		require.Equal(t, "api", got.Name)
	})

	t.Run("wrong team gives 404 not 'forbidden'", func(t *testing.T) {
		// The team scoping intentionally returns NotFound — leaking a 403
		// would tell an attacker that an ID exists somewhere they can't
		// see, which we want to avoid.
		_, err := repo.FindByIDAndTeamServer(ctx, p.ID, "team-b", "srv-a")
		require.Error(t, err)
		require.True(t, fiberutil.IsNotFound(err))
	})

	t.Run("wrong server gives 404", func(t *testing.T) {
		_, err := repo.FindByIDAndTeamServer(ctx, p.ID, "team-a", "srv-b")
		require.Error(t, err)
		require.True(t, fiberutil.IsNotFound(err))
	})
}

func TestProjectRepository_ExistsByName(t *testing.T) {
	db := setupDB(t)
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p := makeProject(t, db, "team-a", "srv-a", "api")

	t.Run("matching name on same server returns true", func(t *testing.T) {
		exists, err := repo.ExistsByName(ctx, "srv-a", "api", "")
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("matching name on different server returns false", func(t *testing.T) {
		exists, err := repo.ExistsByName(ctx, "srv-b", "api", "")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("excludeID lets a rename keep its own name", func(t *testing.T) {
		// During a rename we want "api" to be considered free for project p
		// itself — otherwise the no-op "rename to the same name" case would
		// fail. The excludeID parameter is what makes that work.
		exists, err := repo.ExistsByName(ctx, "srv-a", "api", p.ID)
		require.NoError(t, err)
		require.False(t, exists)
	})
}

func TestProjectRepository_ListForServer(t *testing.T) {
	db := setupDB(t)
	repo := NewProjectRepository(db)
	ctx := context.Background()

	makeProject(t, db, "team-a", "srv-a", "alpha")
	makeProject(t, db, "team-a", "srv-a", "beta")
	makeProject(t, db, "team-a", "srv-b", "gamma") // different server
	makeProject(t, db, "team-b", "srv-a", "delta") // different team

	got, err := repo.ListForServer(ctx, "team-a", "srv-a")
	require.NoError(t, err)
	require.Len(t, got, 2, "should only see this team's projects on this server")
	names := []string{got[0].Name, got[1].Name}
	require.Contains(t, names, "alpha")
	require.Contains(t, names, "beta")
}
