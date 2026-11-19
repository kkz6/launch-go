package services

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/apperror"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestGitHubCandidateFailuresDistinguishOutageFromInvalidAuthorization(t *testing.T) {
	tests := []struct {
		name, path, body string
		status           int
		rateLimited      bool
		refresh          bool
		expectedCode     string
	}{
		{name: "installation outage", path: "/user/installations", status: 503, expectedCode: "git.github_unavailable"},
		{name: "installation restricted", path: "/user/installations", status: 451, expectedCode: "git.github_access_restricted"},
		{name: "inventory restricted", path: "/user/installations/42/repositories", status: 451, expectedCode: "git.github_access_restricted"},
		{name: "refresh restricted", path: "/login/oauth/access_token", status: 451, refresh: true, expectedCode: "git.github_access_restricted"},
		{name: "installation rate limit", path: "/user/installations", status: 403, rateLimited: true, expectedCode: "git.github_unavailable"},
		{name: "inventory outage", path: "/user/installations/42/repositories", status: 502, expectedCode: "git.github_unavailable"},
		{name: "inventory secondary limit", path: "/user/installations/42/repositories", status: 403, body: "{\"message\":\"You have exceeded a secondary rate limit\"}", expectedCode: "git.github_unavailable"},
		{name: "inventory malformed response", path: "/user/installations/42/repositories", status: 200, body: "{", expectedCode: "git.github_unavailable"},
		{name: "inventory revoked token", path: "/user/installations/42/repositories", status: 401, expectedCode: "git.github_reauthorization_required"},
		{name: "refresh outage", path: "/login/oauth/access_token", status: 503, refresh: true, expectedCode: "git.github_unavailable"},
		{name: "refresh malformed response", path: "/login/oauth/access_token", status: 200, body: "{", refresh: true, expectedCode: "git.github_unavailable"},
		{name: "expired refresh token", path: "/login/oauth/access_token", status: 200, body: "{\"error\":\"bad_refresh_token\"}", refresh: true, expectedCode: "git.github_reauthorization_required"},
		{name: "invalid refresh token status", path: "/login/oauth/access_token", status: 400, body: "{\"error\":\"bad_refresh_token\"}", refresh: true, expectedCode: "git.github_reauthorization_required"},
		{name: "permission lookup outage", path: "/repos/org/repo101/collaborators/sponsor/permission", status: 503, expectedCode: "git.github_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newGitFixture(t)
			team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
			review := f.authorize(t, team, user, session, "")
			var attempt models.GitHubConnectionAttempt
			require.NoError(t, f.db.First(&attempt, "id = ?", review.AttemptID).Error)
			if tt.refresh {
				require.NoError(t, f.db.Model(&models.GitHubAuthorization{}).Where("id = ?", attempt.AuthorizationID).Update("expires_at", time.Now().Add(-time.Minute)).Error)
			}
			var before models.GitHubAuthorization
			require.NoError(t, f.db.First(&before, "id = ?", attempt.AuthorizationID).Error)
			var failing atomic.Bool
			failing.Store(true)
			original := f.server.Config.Handler
			f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if failing.Load() && r.URL.Path == tt.path {
					if tt.rateLimited {
						w.Header().Set("X-RateLimit-Remaining", "0")
					}
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
					return
				}
				original.ServeHTTP(w, r)
			})
			_, err := f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42", 1)
			failure := apperror.As(err)
			require.NotNil(t, failure)
			require.Equal(t, tt.expectedCode, failure.Code)
			expectedStatus := http.StatusServiceUnavailable
			if tt.expectedCode != "git.github_unavailable" {
				expectedStatus = http.StatusForbidden
			}
			require.Equal(t, expectedStatus, failure.HTTPStatus)
			require.NotErrorIs(t, err, errAttempt)
			if tt.status == http.StatusUnavailableForLegalReasons {
				_, saveErr := f.service.SelectGitHubRepositories(context.Background(), review.AttemptID, team, user, session, GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}})
				saveFailure := apperror.As(saveErr)
				require.NotNil(t, saveFailure)
				require.Equal(t, "git.github_access_restricted", saveFailure.Code, "Save must preserve the same permanent restriction classification")
				require.Equal(t, http.StatusForbidden, saveFailure.HTTPStatus)
			}
			var after models.GitHubAuthorization
			require.NoError(t, f.db.First(&after, "id = ?", before.ID).Error)
			require.Equal(t, before.AccessToken, after.AccessToken)
			require.Equal(t, before.RefreshToken, after.RefreshToken)
			require.Equal(t, before.RevokedAt, after.RevokedAt)
			var grants int64
			require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Count(&grants).Error)
			require.Zero(t, grants)
			if tt.expectedCode == "git.github_unavailable" {
				failing.Store(false)
				page, err := f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42", 1)
				require.NoError(t, err, "retry must use the same valid attempt after the outage clears")
				require.Len(t, page.Repositories, 3)
			}
		})
	}
}

func TestGitHubCandidateErrorClassificationKeepsActualDenials(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := newGitFixture(t)
			team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
			review := f.authorize(t, team, user, session, "")
			original := f.server.Config.Handler
			f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/user/installations/42/repositories" {
					http.Error(w, "{\"message\":\"Resource not accessible\"}", status)
					return
				}
				original.ServeHTTP(w, r)
			})
			_, err := f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42", 1)
			require.ErrorIs(t, err, access.ErrAccessDenied)
			require.NoError(t, f.db.Model(&models.GitHubConnectionAttempt{}).Where("id = ?", review.AttemptID).Update("expires_at", time.Now().Add(-time.Minute)).Error)
			_, err = f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42", 1)
			require.ErrorIs(t, err, errAttempt)
		})
	}
}

func TestGitHubReviewDiagnosticsExcludeRawErrors(t *testing.T) {
	f := newGitFixture(t)
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	f.service.Logger = &logger
	err := f.service.githubReviewError("user_token", errors.New("POST https://private.example/oauth?access_token=do-not-log: transport unavailable"))
	require.Equal(t, "git.github_unavailable", apperror.As(err).Code)
	require.NotContains(t, err.Error(), "private.example")
	require.NotContains(t, err.Error(), "do-not-log")
	require.NotContains(t, logs.String(), "private.example")
	require.NotContains(t, logs.String(), "do-not-log")
	require.Contains(t, logs.String(), "\"stage\":\"user_token\"")
	require.Contains(t, logs.String(), "\"retryable\":true")
}
