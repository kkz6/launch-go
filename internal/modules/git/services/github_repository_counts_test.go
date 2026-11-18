package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/models"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/stretchr/testify/require"
)

func TestGitHubRepositoryCountsMatchAccountPickerDespiteStaleInstallationCount(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "202", "303")

	// Reproduce an old installation-wide count and retained inventory rows.
	// One row is saved selection whose upstream access has been removed.
	require.NoError(t, f.db.Model(&models.SourceControl{}).Where("id IN ?", []string{a.id, b.id}).Update("repository_count", 11).Error)
	for i := range 10 {
		repo := models.SourceControlRepository{SourceControlID: a.id, GitHubRepositoryID: fmt.Sprint(400 + i), Name: fmt.Sprintf("inventory-%d", i), FullName: fmt.Sprintf("org/inventory-%d", i), Selected: i == 0, Available: i != 0, GrantVersion: 1}
		require.NoError(t, f.db.Create(&repo).Error)
	}

	assertCounts := func(teamID, connectionID string, count int) {
		t.Helper()
		picker, err := f.service.GetSourceControlRepositories(ctx, connectionID, teamID)
		require.NoError(t, err)
		require.Len(t, picker, count)
		listed, err := f.service.ListSourceControls(ctx, teamID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, connectionID, listed[0].ID)
		require.Equal(t, len(picker), listed[0].RepositoryCount)
		require.ElementsMatch(t, picker, listed[0].Repositories)
		detail, err := f.service.GetSourceControl(ctx, connectionID, teamID)
		require.NoError(t, err)
		require.Equal(t, len(picker), detail.RepositoryCount)
		require.ElementsMatch(t, picker, detail.Repositories)
		summaries, err := f.service.GetInstallationsWithRepositoryCounts(ctx, teamID)
		require.NoError(t, err)
		require.Len(t, summaries["github"], 1)
		require.Equal(t, len(picker), summaries["github"][0].RepositoryCount)
		cached, err := f.service.GetCachedRepositories(ctx, gittypes.GitProviderGitHub, "42", teamID)
		require.NoError(t, err)
		require.Len(t, cached, count)
	}

	assertCounts(teamA, a.id, 1)
	assertCounts(teamB, b.id, 2)

	// Lifecycle changes need not repair old cached totals to return the right
	// count; every public response derives it from this connection's grants.
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ? AND git_hub_repository_id = ?", a.id, "101").Update("available", false).Error)
	assertCounts(teamA, a.id, 0)
	assertCounts(teamB, b.id, 2)
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ? AND git_hub_repository_id = ?", a.id, "101").Update("available", true).Error)
	for _, status := range []string{"disconnected", "reconnect_required"} {
		require.NoError(t, f.db.Model(&models.SourceControl{}).Where("id = ?", a.id).Update("status", status).Error)
		assertCounts(teamA, a.id, 0)
		assertCounts(teamB, b.id, 2)
	}
}
