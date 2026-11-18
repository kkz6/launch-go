package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/stretchr/testify/require"
)

func TestGitHubCandidateReviewPreservesDependencyWorkloadImpact(t *testing.T) {
	for _, postgresDB := range []bool{false, true} {
		t.Run(fmt.Sprint("postgres=", postgresDB), func(t *testing.T) {
			f := newGitFixtureDatabase(t, postgresDB)
			team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
			connection := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
			var repo models.SourceControlRepository
			require.NoError(t, f.db.Where("source_control_id = ?", connection.id).First(&repo).Error)
			require.NoError(t, f.db.Exec("CREATE TABLE sites (id text, address text, team_id text, source_control_id text, source_control_repositories_id bigint, repository_dependency_ids json)").Error)
			for _, table := range []string{"docker_applications", "docker_composes"} {
				require.NoError(t, f.db.Exec("CREATE TABLE "+table+" (id text, name text, team_id text, source_config json, deleted_at timestamp)").Error)
			}
			dependencies := fmt.Sprintf(`["%d"]`, repo.ID)
			require.NoError(t, f.db.Exec("INSERT INTO sites VALUES (?,?,?,?,?,?)", "main-site", "main.example", team, connection.id, repo.ID, "[]").Error)
			require.NoError(t, f.db.Exec("INSERT INTO sites VALUES (?,?,?,?,?,?)", "dependent-site", "dependent.example", team, nil, nil, dependencies).Error)
			require.NoError(t, f.db.Exec("INSERT INTO sites VALUES (?,?,?,?,?,?)", "foreign-site", "foreign.example", "other-team", connection.id, repo.ID, dependencies).Error)
			require.NoError(t, f.db.Exec("INSERT INTO docker_applications VALUES (?,?,?,?,NULL)", "main-application", "main application", team, fmt.Sprintf(`{"source_control_id":%q,"repository_id":"%d"}`, connection.id, repo.ID)).Error)
			require.NoError(t, f.db.Exec("INSERT INTO docker_composes VALUES (?,?,?,?,NULL)", "dependent-compose", "dependent compose", team, fmt.Sprintf(`{"dependency_repository_ids":%s}`, dependencies)).Error)
			review := f.authorize(t, team, user, session, connection.id)
			candidates, err := f.service.GitHubCandidates(context.Background(), review.AttemptID, team, user, session, "42")
			require.NoError(t, err)
			var impacts []GitHubWorkloadImpact
			for _, candidate := range candidates.Repositories {
				if candidate.GitHubRepositoryID == "101" {
					impacts = candidate.AffectedWorkloads
				}
			}
			require.ElementsMatch(t, []GitHubWorkloadImpact{{Kind: "site", ID: "main-site", Name: "main.example"}, {Kind: "site", ID: "dependent-site", Name: "dependent.example"}, {Kind: "application", ID: "main-application", Name: "main application"}, {Kind: "compose", ID: "dependent-compose", Name: "dependent compose"}}, impacts)
		})
	}
}
