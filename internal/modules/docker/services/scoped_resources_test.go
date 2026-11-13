package services

import (
	"context"
	"github.com/kkz6/launch-go/tests/testutil"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

func TestChildResourceListsRequireCompleteOwnershipChain(t *testing.T) {
	db := testutil.SQLite(t,
		&models.Project{}, &models.Application{}, &models.Compose{},
		&models.ApplicationEnvVar{}, &models.ApplicationBuildSecret{},
		&models.ComposeBuildSecret{}, &models.ApplicationSchedule{},
		&models.ApplicationDomain{}, &models.ApplicationVolume{},
	)

	for _, fixture := range []struct{ id, team, server string }{
		{"owned", "team-a", "server-a"},
		{"other-project", "team-a", "server-a"},
		{"other-team", "team-b", "server-a"},
		{"other-server", "team-a", "server-b"},
		{"deleted", "team-a", "server-a"},
	} {
		project := models.Project{Name: fixture.id}
		project.ID, project.TeamID, project.ServerID = fixture.id, fixture.team, fixture.server
		require.NoError(t, db.Create(&project).Error)
		app := models.Application{ProjectID: fixture.id, Name: fixture.id, SourceType: "image"}
		app.ID, app.TeamID, app.ServerID = fixture.id, fixture.team, fixture.server
		require.NoError(t, db.Create(&app).Error)
		compose := models.Compose{ProjectID: fixture.id, Name: fixture.id, ComposeSourceType: "raw"}
		compose.ID, compose.TeamID, compose.ServerID = fixture.id, fixture.team, fixture.server
		require.NoError(t, db.Create(&compose).Error)
		if fixture.id == "deleted" {
			require.NoError(t, db.Delete(&project).Error)
		}
	}

	deps := &ServiceDeps{ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
		Dependencies: pkgservice.Dependencies{DB: db},
		Repos:        repositories.NewRegistry(db),
	}}
	domains := NewDomainService(deps)
	volumes := NewVolumeService(deps)
	lists := []struct {
		name string
		list func(context.Context, string, string, string, string) error
	}{
		{"application env vars", listError(NewEnvVarService(deps).ListEnvVars)},
		{"application build secrets", listError(NewBuildSecretService(deps).ListBuildSecrets)},
		{"application schedules", listError(NewScheduleService(deps).ListSchedules)},
		{"application domains", listError(domains.ListDomains)},
		{"application volumes", listError(volumes.ListVolumes)},
		{"compose build secrets", listError(NewComposeBuildSecretService(deps).ListBuildSecrets)},
		{"compose domains", listError(domains.ListComposeDomains)},
		{"compose volumes", listError(volumes.ListComposeVolumes)},
	}
	cases := []struct {
		name, workload, project, server, team string
		allowed                               bool
	}{
		{"owned workload", "owned", "owned", "server-a", "team-a", true},
		{"other team's own workload", "other-team", "other-team", "server-a", "team-b", true},
		{"wrong project", "owned", "other-project", "server-a", "team-a", false},
		{"missing project", "owned", "missing", "server-a", "team-a", false},
		{"missing workload", "missing", "owned", "server-a", "team-a", false},
		{"wrong request team", "owned", "owned", "server-a", "team-b", false},
		{"wrong request server", "owned", "owned", "server-b", "team-a", false},
		{"foreign workload team", "other-team", "owned", "server-a", "team-a", false},
		{"foreign workload server", "other-server", "owned", "server-a", "team-a", false},
		{"deleted parent", "deleted", "deleted", "server-a", "team-a", false},
	}
	for _, resource := range lists {
		t.Run(resource.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					err := resource.list(context.Background(), tc.workload, tc.project, tc.server, tc.team)
					if tc.allowed {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
						require.True(t, fiberutil.IsNotFound(err), "expected 404, got %v", err)
					}
				})
			}
		})
	}
}

func listError[T any](list func(context.Context, string, string, string, string) ([]T, error)) func(context.Context, string, string, string, string) error {
	return func(ctx context.Context, workloadID, projectID, serverID, teamID string) error {
		_, err := list(ctx, workloadID, projectID, serverID, teamID)
		return err
	}
}
