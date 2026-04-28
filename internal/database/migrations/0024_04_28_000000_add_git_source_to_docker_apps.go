package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0024_04_28_000000_add_git_source_to_docker_apps",
		Name:      "Add git source columns to docker_apps",
		Timestamp: time.Date(2026, 4, 28, 0, 0, 1, 0, time.UTC),
		Up:        addGitSourceToDockerAppsUp,
		Down:      addGitSourceToDockerAppsDown,
	})
}

func addGitSourceToDockerAppsUp(db *gorm.DB) error {
	stmts := []string{
		"ALTER TABLE docker_apps ADD COLUMN git_repo_url VARCHAR(512) NULL",
		"ALTER TABLE docker_apps ADD COLUMN git_branch VARCHAR(255) NULL",
		"ALTER TABLE docker_apps ADD COLUMN git_dockerfile VARCHAR(255) NULL",
		"ALTER TABLE docker_apps ADD COLUMN git_context VARCHAR(255) NULL",
		"ALTER TABLE docker_apps ADD COLUMN git_token LONGTEXT NULL",
		"ALTER TABLE docker_apps ADD COLUMN git_commit_sha VARCHAR(64) NULL",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

func addGitSourceToDockerAppsDown(db *gorm.DB) error {
	stmts := []string{
		"ALTER TABLE docker_apps DROP COLUMN git_commit_sha",
		"ALTER TABLE docker_apps DROP COLUMN git_token",
		"ALTER TABLE docker_apps DROP COLUMN git_context",
		"ALTER TABLE docker_apps DROP COLUMN git_dockerfile",
		"ALTER TABLE docker_apps DROP COLUMN git_branch",
		"ALTER TABLE docker_apps DROP COLUMN git_repo_url",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
