package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func selectionFromReview(t *testing.T, review GitHubCandidatesResponse, installation string, repositories ...string) GitHubSelectionRequest {
	t.Helper()
	id, err := json.Marshal(review.SourceControlID)
	require.NoError(t, err)
	return GitHubSelectionRequest{SourceControlID: id, InstallationID: installation, RepositoryIDs: repositories, ExpectedGeneration: &review.ConnectionGeneration, ExpectedSelectionVersion: &review.SelectionVersion}
}

func TestGitHubPostgresFreshConnectReusesDisconnectedConnection(t *testing.T) {
	for _, mode := range []string{"legacy snapshots", "reviewed connection", "explicit reconnect"} {
		t.Run(mode, func(t *testing.T) {
			f := newGitFixtureDatabase(t, true)
			ctx := context.Background()
			team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
			otherTeam, otherUser := "team-b00000000000000000000", "user-b00000000000000000000"
			first := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
			other := f.selectRepos(t, f.authorize(t, otherTeam, otherUser, "session-b", ""), otherUser, "session-b", "303")
			var before models.SourceControl
			var otherGrants []models.SourceControlRepository
			require.NoError(t, f.db.First(&before, "id = ?", other.id).Error)
			require.NoError(t, f.db.Where("source_control_id = ?", other.id).Find(&otherGrants).Error)
			resolver := access.NewResolver(f.db, f.service.ProviderFactory())
			require.NoError(t, resolver.Disconnect(ctx, first.id, team))
			f.githubUserID = 100 // Reconnect with a freshly verified GitHub sponsor.
			target := ""
			if mode == "explicit reconnect" {
				target = first.id
			}
			attempt := f.authorize(t, team, user, session, target)
			reviewed, err := f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "42")
			require.NoError(t, err)
			require.Equal(t, first.id, *reviewed.SourceControlID)
			require.Equal(t, first.generation+1, reviewed.ConnectionGeneration)
			for _, candidate := range reviewed.Repositories {
				require.Equal(t, candidate.GitHubRepositoryID == "101", candidate.Selected)
			}
			request := selectionFromReview(t, reviewed, "42", "202")
			if mode == "legacy snapshots" {
				request.SourceControlID = nil
			}
			saved, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, request)
			require.NoError(t, err)
			require.Equal(t, first.id, saved.ID)
			require.Equal(t, first.generation+2, saved.Generation)
			var restored models.SourceControl
			require.NoError(t, f.db.First(&restored, "id = ?", first.id).Error)
			require.Equal(t, "active", restored.Status)
			require.Equal(t, 1, *restored.RepositoryCount)
			var sponsor models.GitHubAuthorization
			require.NoError(t, f.db.First(&sponsor, "id = ?", *restored.GitHubAuthorizationID).Error)
			require.Equal(t, "100", sponsor.GitHubUserID)
			_, err = resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: first.id, Repository: "org/repo202", Operation: access.Clone})
			require.NoError(t, err)
			for _, repository := range []string{"org/repo101", "org/repo303"} {
				_, err = resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: first.id, Repository: repository, Operation: access.Clone})
				require.Error(t, err, "only newly selected repositories regain access")
			}
			var localCount int64
			require.NoError(t, f.db.Model(&models.SourceControl{}).Where("team_id = ?", team).Count(&localCount).Error)
			require.EqualValues(t, 1, localCount)
			require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", first.id).Count(&localCount).Error)
			require.EqualValues(t, 2, localCount, "reconnect preserves repository history")
			var after models.SourceControl
			var afterGrants []models.SourceControlRepository
			require.NoError(t, f.db.First(&after, "id = ?", other.id).Error)
			require.NoError(t, f.db.Where("source_control_id = ?", other.id).Find(&afterGrants).Error)
			require.Equal(t, before, after, "the other account connection must not change")
			require.Equal(t, otherGrants, afterGrants)
			_, err = resolver.Resolve(ctx, access.Request{TeamID: otherTeam, SourceControlID: other.id, Repository: "org/repo303", Operation: access.Clone})
			require.NoError(t, err)
		})
	}
}

func TestGitHubFreshConnectReviewsExistingActiveSelection(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	first := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	attempt := f.authorize(t, team, user, session, "")
	reviewed, err := f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "42")
	require.NoError(t, err)
	require.Equal(t, first.id, *reviewed.SourceControlID)
	for _, candidate := range reviewed.Repositories {
		require.Equal(t, candidate.GitHubRepositoryID == "101", candidate.Selected)
	}
	var unchanged models.GitHubConnectionAttempt
	require.NoError(t, f.db.First(&unchanged, "id = ?", attempt.AttemptID).Error)
	require.Nil(t, unchanged.SourceControlID, "listing candidates must not pin the attempt")
	_, err = access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: first.id, Repository: "org/repo202", Operation: access.Clone})
	require.Error(t, err, "review does not grant access")
	saved, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, selectionFromReview(t, reviewed, "42", "202"))
	require.NoError(t, err)
	require.Equal(t, first.id, saved.ID)
	require.Equal(t, first.generation, saved.Generation, "review with the same sponsor preserves the connection generation")
}

func TestGitHubFreshConnectRejectsUnreviewedOrForeignConnection(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	first := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	other := f.selectRepos(t, f.authorize(t, "team-b00000000000000000000", "user-b00000000000000000000", "session-b", ""), "user-b00000000000000000000", "session-b", "303")
	attempt := f.authorize(t, team, user, session, "")
	reviewed, err := f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "42")
	require.NoError(t, err)
	foreignID, err := json.Marshal(other.id)
	require.NoError(t, err)
	for _, kind := range []string{"missing snapshots", "zero snapshots", "explicit new", "foreign account", "invalid ID type"} {
		t.Run(kind, func(t *testing.T) {
			req := selectionFromReview(t, reviewed, "42", "202")
			switch kind {
			case "missing snapshots":
				req.SourceControlID, req.ExpectedGeneration, req.ExpectedSelectionVersion = nil, nil, nil
			case "zero snapshots":
				var zero uint64
				req.SourceControlID, req.ExpectedGeneration, req.ExpectedSelectionVersion = nil, &zero, &zero
			case "explicit new":
				req.SourceControlID = json.RawMessage("null")
			case "foreign account":
				req.SourceControlID = foreignID
			case "invalid ID type":
				req.SourceControlID = json.RawMessage("42")
			}
			_, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, req)
			require.Error(t, err)
		})
	}
	var row models.SourceControl
	require.NoError(t, f.db.First(&row, "id = ?", first.id).Error)
	require.Equal(t, reviewed.SelectionVersion, row.SelectionVersion)
}

func TestGitHubFreshConnectCanSwitchInstallationWithoutReusingSnapshot(t *testing.T) {
	f := newGitFixture(t)
	f.otherInstallation = true
	ctx := context.Background()
	team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
	first := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
	newAttempt := f.authorize(t, team, user, session, "")
	firstReview, err := f.service.GitHubCandidates(ctx, newAttempt.AttemptID, team, user, session, "42")
	require.NoError(t, err)
	newReview, err := f.service.GitHubCandidates(ctx, newAttempt.AttemptID, team, user, session, "43")
	require.NoError(t, err)
	require.Nil(t, newReview.SourceControlID)
	require.Zero(t, newReview.ConnectionGeneration)
	require.Zero(t, newReview.SelectionVersion)
	for _, repo := range newReview.Repositories {
		require.False(t, repo.Selected)
	}
	_, err = f.service.SelectGitHubRepositories(ctx, newAttempt.AttemptID, team, user, session, selectionFromReview(t, firstReview, "43", "404"))
	require.Error(t, err)
	second, err := f.service.SelectGitHubRepositories(ctx, newAttempt.AttemptID, team, user, session, selectionFromReview(t, newReview, "43", "404"))
	require.NoError(t, err)
	require.NotEqual(t, first.id, second.ID)

	attempt := f.authorize(t, team, user, session, "")
	firstReview, err = f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "42")
	require.NoError(t, err)
	secondReview, err := f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "43")
	require.NoError(t, err)
	require.Equal(t, firstReview.ConnectionGeneration, secondReview.ConnectionGeneration)
	require.Equal(t, firstReview.SelectionVersion, secondReview.SelectionVersion)
	_, err = f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, selectionFromReview(t, firstReview, "43", "505"))
	require.Error(t, err, "equal versions cannot make another installation's reviewed ID valid")
	saved, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, selectionFromReview(t, secondReview, "43", "505"))
	require.NoError(t, err)
	require.Equal(t, second.ID, saved.ID)
	var firstAfter models.SourceControl
	require.NoError(t, f.db.First(&firstAfter, "id = ?", first.id).Error)
	require.Equal(t, firstReview.SelectionVersion, firstAfter.SelectionVersion)

	explicit := f.authorize(t, team, user, session, first.id)
	_, err = f.service.GitHubCandidates(ctx, explicit.AttemptID, team, user, session, "43")
	require.Error(t, err, "explicit reconnect remains bound to its original installation")
}

func TestGitHubPostgresFreshConnectRejectsConcurrentRevocation(t *testing.T) {
	for _, change := range []string{"disconnect", "disconnect and reconnect", "grant revocation", "attempt target changed"} {
		t.Run(change, func(t *testing.T) {
			f := newGitFixtureDatabase(t, true)
			ctx := context.Background()
			team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
			first := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
			attempt := f.authorize(t, team, user, session, "")
			reviewed, err := f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "42")
			require.NoError(t, err)
			f.beforeInstallation = func() {
				switch change {
				case "grant revocation":
					require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", first.id).Update("grant_version", gorm.Expr("grant_version + 1")).Error)
				case "attempt target changed":
					require.NoError(t, f.db.Model(&models.GitHubConnectionAttempt{}).Where("id = ?", attempt.AttemptID).Update("source_control_id", first.id).Error)
				default:
					require.NoError(t, access.NewResolver(f.db, f.service.ProviderFactory()).Disconnect(ctx, first.id, team))
					if change == "disconnect and reconnect" {
						f.selectRepos(t, f.authorize(t, team, user, session, first.id), user, session, "303")
					}
				}
			}
			_, err = f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, selectionFromReview(t, reviewed, "42", "202"))
			require.Error(t, err)
			var count int64
			require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ? AND git_hub_repository_id = ? AND selected = ?", first.id, "202", true).Count(&count).Error)
			require.Zero(t, count, "a stale save must not grant its requested repository")
			var row models.SourceControl
			require.NoError(t, f.db.First(&row, "id = ?", first.id).Error)
			if change == "disconnect" {
				require.Equal(t, "disconnected", row.Status)
			}
			if change == "disconnect and reconnect" {
				require.Equal(t, "active", row.Status)
				_, err = access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: first.id, Repository: "org/repo303", Operation: access.Clone})
				require.NoError(t, err)
			}
		})
	}
}
