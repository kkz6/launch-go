package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGitHubAccountLifecycleDoesNotChangeOtherAccount(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
	teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
	b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "202")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	bRequest := access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo202", Operation: access.Clone}
	grant, err := resolver.Resolve(ctx, bRequest)
	require.NoError(t, err)
	_, err = f.provider.GetInstallationToken(grant.Context(ctx), "42")
	require.NoError(t, err)
	bSnapshot := grant.Snapshot()
	bRequest.Expected = &bSnapshot
	var auth models.GitHubAuthorization
	require.NoError(t, f.db.First(&auth, "id = ?", bSnapshot.AuthorizationID).Error)
	f.revokeFailToken = "b-pending-revocation"
	foreignPending := models.GitHubCredentialLease{TeamID: teamB, SourceControlID: b.id, Token: dbtype.EncryptedString(f.revokeFailToken), ExpiresAt: time.Now().Add(time.Hour), Status: "revoke_pending"}
	require.NoError(t, f.db.Create(&foreignPending).Error)
	assertUnchanged := func() {
		t.Helper()
		_, err := resolver.Resolve(ctx, bRequest)
		require.NoError(t, err, "B's existing jobs must remain authorized")
		require.NoError(t, access.ValidateSnapshot(ctx, f.db, teamB, bSnapshot))
		var savedAuth models.GitHubAuthorization
		require.NoError(t, f.db.First(&savedAuth, "id = ?", auth.ID).Error)
		require.Equal(t, auth, savedAuth, "A cannot change B's authorization")
		var lease models.GitHubCredentialLease
		require.NoError(t, f.db.Where("source_control_id = ? AND status = ?", b.id, "released").First(&lease).Error)
		require.Equal(t, "released", lease.Status, "A cannot revoke B's credentials")
		require.Zero(t, f.revoked, "A's local operations cannot wait on B's pending revocations")
		repos, err := f.service.GetRepositoriesBySourceControlID(ctx, b.id, teamB)
		require.NoError(t, err)
		require.Len(t, repos, 1)
		require.Equal(t, "202", repos[0].GitHubRepositoryID)
	}
	a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
	assertUnchanged()
	for _, ids := range [][]string{{"101", "202", "303"}, {"202"}, {}} {
		f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", a.id), userA, "session-a", ids...)
		assertUnchanged()
	}
	require.NoError(t, f.service.Disconnect(ctx, a.id, teamA, userA))
	assertUnchanged()
	f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", a.id), userA, "session-a", "303")
	assertUnchanged()
	for _, name := range []string{"org/repo101", "org/repo202"} {
		_, err = resolver.Resolve(ctx, access.Request{TeamID: teamA, SourceControlID: a.id, Repository: name, Operation: access.Clone})
		require.Error(t, err, "reconnecting must restore only the new explicit selection")
	}
}

func TestGitHubSelectionCannotOverwriteConcurrentUpstreamRevocation(t *testing.T) {
	for _, revoked := range []string{"installation", "repository"} {
		t.Run(revoked, func(t *testing.T) {
			f := newGitFixture(t)
			ctx := context.Background()
			team, user := "team-a00000000000000000000", "user-a00000000000000000000"
			sc := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101")
			review := f.authorize(t, team, user, "session-a", sc.id)
			f.beforeInstallation = func() {
				if revoked == "installation" {
					require.NoError(t, f.db.Model(&models.GitHubInstallation{}).Where("installation_id = ?", "42").Updates(map[string]any{"status": "suspended", "generation": gorm.Expr("generation + 1")}).Error)
				} else {
					require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", sc.id).Updates(map[string]any{"available": false, "grant_version": gorm.Expr("grant_version + 1")}).Error)
				}
			}
			_, err := f.service.SelectGitHubRepositories(ctx, review.AttemptID, team, user, "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}, ExpectedGeneration: &review.ConnectionGeneration, ExpectedSelectionVersion: &review.SelectionVersion})
			require.ErrorIs(t, err, access.ErrStaleAccess)
			_, err = access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo101", Operation: access.Clone})
			require.Error(t, err, "a stale upstream response must never restore revoked access")
		})
	}
}

func TestGitHubFirstRepositorySelectionCannotMissConcurrentRemoval(t *testing.T) {
	for _, existingConnection := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-connection=%v", existingConnection), func(t *testing.T) {
			f := newGitFixture(t)
			ctx := context.Background()
			teamA, userA := "team-a00000000000000000000", "user-a00000000000000000000"
			teamB, userB := "team-b00000000000000000000", "user-b00000000000000000000"
			b := f.selectRepos(t, f.authorize(t, teamB, userB, "session-b", ""), userB, "session-b", "202", "303")
			resolver := access.NewResolver(f.db, f.service.ProviderFactory())
			unaffected, err := resolver.Resolve(ctx, access.Request{TeamID: teamB, SourceControlID: b.id, Repository: "org/repo303", Operation: access.Clone})
			require.NoError(t, err)
			connectionID := ""
			wanted := []string{"202"}
			if existingConnection {
				a := f.selectRepos(t, f.authorize(t, teamA, userA, "session-a", ""), userA, "session-a", "101")
				connectionID = a.id
				wanted = append(wanted, "101")
			}
			review := f.authorize(t, teamA, userA, "session-a", connectionID)
			f.beforeInstallation = func() {
				// This models a delivered removal after A fetched repository
				// inventory, before it can commit its first grant for repo 202.
				require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Model(&models.GitHubInstallation{}).Where("installation_id = ?", "42").Update("repository_revision", gorm.Expr("repository_revision + 1")).Error; err != nil {
						return err
					}
					return tx.Model(&models.SourceControlRepository{}).Where("source_control_id = ? AND git_hub_repository_id = ?", b.id, "202").Updates(map[string]any{"available": false, "grant_version": gorm.Expr("grant_version + 1")}).Error
				}))
			}
			_, err = f.service.SelectGitHubRepositories(ctx, review.AttemptID, teamA, userA, "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: wanted, ExpectedGeneration: &review.ConnectionGeneration, ExpectedSelectionVersion: &review.SelectionVersion})
			require.ErrorIs(t, err, access.ErrStaleAccess)
			var count int64
			require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Joins("JOIN source_controls sc ON sc.id = source_control_repositories.source_control_id").Where("sc.team_id = ? AND git_hub_repository_id = ? AND selected = ?", teamA, "202", true).Count(&count).Error)
			require.Zero(t, count, "the stale response cannot create an unavailable grant")
			require.NoError(t, unaffected.Validate(ctx), "a repository revision must not invalidate another repository's existing grant")
			if existingConnection {
				_, err = resolver.Resolve(ctx, access.Request{TeamID: teamA, SourceControlID: connectionID, Repository: "org/repo101", Operation: access.Clone})
				require.NoError(t, err)
			}
		})
	}
}

func TestGitHubFirstConnectionHonorsLifecycleRecordCreatedDuringVerification(t *testing.T) {
	for _, event := range []string{"repository_removed", "installation_suspended"} {
		t.Run(event, func(t *testing.T) {
			f := newGitFixture(t)
			ctx := context.Background()
			team, user := "team-a00000000000000000000", "user-a00000000000000000000"
			review := f.authorize(t, team, user, "session-a", "")
			f.beforeInstallation = func() {
				installation := models.GitHubInstallation{Host: "github.com", AppID: "7", InstallationID: "42", Status: "active", Generation: 1, RepositoryRevision: 2}
				if event == "installation_suspended" {
					installation.Status = "suspended"
					installation.Generation = 2
					installation.RepositoryRevision = 1
				}
				require.NoError(t, f.db.Create(&installation).Error)
			}
			_, err := f.service.SelectGitHubRepositories(ctx, review.AttemptID, team, user, "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}})
			require.ErrorIs(t, err, access.ErrStaleAccess)
			var count int64
			require.NoError(t, f.db.Model(&models.SourceControl{}).Count(&count).Error)
			require.Zero(t, count, "a first connection must not ignore an already delivered revocation")
		})
	}
}

func TestGitHubInventoryRevokesOnlyRemovedRepositoryCredentials(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	sc := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101", "202")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	grants := map[string]*access.RepositoryAccess{}
	for _, remoteID := range []string{"101", "202"} {
		grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo" + remoteID, Operation: access.Clone})
		require.NoError(t, err)
		_, err = f.provider.GetInstallationToken(grant.Context(ctx), "42")
		require.NoError(t, err)
		grants[remoteID] = grant
	}
	var connection models.SourceControl
	require.NoError(t, f.db.First(&connection, "id = ?", sc.id).Error)
	require.NoError(t, f.service.SyncInstallationRepositories(ctx, &connection, []map[string]any{{"id": "202", "full_name": "org/repo202", "default_branch": "main", "private": true}}))
	for _, remoteID := range []string{"101", "202"} {
		var lease models.GitHubCredentialLease
		require.NoError(t, f.db.Where("source_control_id = ? AND repository_id = ?", sc.id, grants[remoteID].Snapshot().RepositoryID).First(&lease).Error)
		expected := "released"
		if remoteID == "101" {
			expected = "revoke_pending"
		}
		require.Equal(t, expected, lease.Status, fmt.Sprintf("credential for repository %s", remoteID))
	}
	require.NoError(t, grants["202"].Validate(ctx))
	require.Error(t, grants["101"].Validate(ctx))
}

func TestGitHubStaleInventoryCannotInvalidateNewSelection(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	sc := f.selectRepos(t, f.authorize(t, team, user, "session-a", ""), user, "session-a", "101")
	var beforeFetch models.SourceControl
	require.NoError(t, f.db.First(&beforeFetch, "id = ?", sc.id).Error)
	f.selectRepos(t, f.authorize(t, team, user, "session-a", sc.id), user, "session-a", "101", "202")
	err := f.service.SyncInstallationRepositories(ctx, &beforeFetch, []map[string]any{{"id": "101", "full_name": "org/repo101", "default_branch": "main", "private": true}})
	require.ErrorIs(t, err, access.ErrStaleAccess)
	_, err = access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo202", Operation: access.Clone})
	require.NoError(t, err, "an inventory response fetched before selection cannot revoke the new grant")
}

func TestGitHubPostgresSeparateAccountsCanConnectFirstInstallationConcurrently(t *testing.T) {
	f := newGitFixtureDatabase(t, true)
	ctx := context.Background()
	type account struct {
		team, user, session, repository string
		attempt                         GitHubAttemptResponse
	}
	accounts := []account{}
	for i, suffix := range []string{"a", "b"} {
		a := account{team: "team-" + suffix + "00000000000000000000", user: "user-" + suffix + "00000000000000000000", session: "session-" + suffix, repository: []string{"101", "202"}[i]}
		a.attempt = f.authorize(t, a.team, a.user, a.session, "")
		accounts = append(accounts, a)
	}
	var firstReads sync.WaitGroup
	firstReads.Add(2)
	require.NoError(t, f.db.Callback().Query().After("gorm:query").Register("test:simultaneous-first-installation", func(tx *gorm.DB) {
		_, locked := tx.Statement.Clauses["FOR"]
		if tx.Statement.Table == "git_hub_installations" && locked && errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			firstReads.Done()
			firstReads.Wait()
		}
	}))
	type result struct {
		account
		connectionID string
		err          error
	}
	results := make(chan result, len(accounts))
	for _, a := range accounts {
		go func(a account) {
			connection, err := f.service.SelectGitHubRepositories(ctx, a.attempt.AttemptID, a.team, a.user, a.session, GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{a.repository}})
			results <- result{account: a, connectionID: connection.ID, err: err}
		}(a)
	}
	for range accounts {
		r := <-results
		require.NoError(t, r.err, "the other account's first connection must not fail this connection")
		repos, err := f.service.GetRepositoriesBySourceControlID(ctx, r.connectionID, r.team)
		require.NoError(t, err)
		require.Len(t, repos, 1)
		require.Equal(t, r.repository, repos[0].GitHubRepositoryID)
	}
	var count int64
	require.NoError(t, f.db.Model(&models.GitHubInstallation{}).Count(&count).Error)
	require.Equal(t, int64(1), count, "connections must reuse the same shared installation metadata")
}
