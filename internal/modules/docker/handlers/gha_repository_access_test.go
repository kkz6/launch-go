package handlers

import (
	"context"
	"fmt"
	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/testutil"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestGHACallbackCannotReviveAfterReconnectOrRepositoryRemoval(t *testing.T) {
	for _, scenario := range []string{"unselected", "reconnected", "new config snapshot", "missing bootstrap snapshot", "cross account"} {
		t.Run(scenario, func(t *testing.T) {
			app, db, workload := setupHandler(t)
			scID := workload.SourceConfig["source_control_id"].(string)
			switch scenario {
			case "unselected":
				require.NoError(t, db.Model(&gitmodels.SourceControlRepository{}).Where("source_control_id = ?", scID).Update("selected", false).Error)
			case "reconnected", "new config snapshot":
				require.NoError(t, db.Model(&gitmodels.SourceControl{}).Where("id = ?", scID).Update("generation", 2).Error)
				if scenario == "new config snapshot" {
					workload.SourceConfig["git_access"] = map[string]any{"connection_generation": 2}
					require.NoError(t, db.Model(workload).Update("source_config", workload.SourceConfig).Error)
				}
			case "missing bootstrap snapshot":
				delete(workload.SourceConfig, "gha_access")
				require.NoError(t, db.Model(workload).Update("source_config", workload.SourceConfig).Error)
			case "cross account":
				require.NoError(t, db.Model(workload).Update("team_id", "another-account").Error)
			}
			resp, _ := post(t, app, "/api/webhooks/docker/applications/"+workload.ID+"/status", rawToken, statusPayload{Status: "failed", RunID: "old-generation"})
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			var count int64
			require.NoError(t, db.Model(&dockermodels.Deployment{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestGHACallbackUnaffectedByAnotherAccountsSharedSelection(t *testing.T) {
	app, db, workload := setupHandler(t)
	var primary gitmodels.SourceControlRepository
	require.NoError(t, db.First(&primary, "id = ?", workload.SourceConfig["repository_id"]).Error)
	snapshot, err := access.SnapshotFromMap(workload.SourceConfig)
	require.NoError(t, err)
	_, err = access.RegisterCleanup(context.Background(), db, snapshot.ConnectionID, snapshot.RepositoryID, *snapshot, []access.Resource{{Kind: "secret", Name: "LAUNCH_A"}})
	require.NoError(t, err)
	other := testutil.SelectedRepository(t, db, "account-b", primary.FullName, map[string]any{})
	require.NoError(t, db.Model(other.Repository).Update("git_hub_repository_id", primary.GitHubRepositoryID).Error)
	for index, change := range []map[string]any{{"selection_version": 2}, {"generation": 2}, {"status": "disconnected"}} {
		require.NoError(t, db.Model(other.SourceControl).Updates(change).Error)
		resp, _ := post(t, app, "/api/webhooks/docker/applications/"+workload.ID+"/status", rawToken, statusPayload{Status: "failed", RunID: fmt.Sprintf("owner-run-%d", index)})
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
}
