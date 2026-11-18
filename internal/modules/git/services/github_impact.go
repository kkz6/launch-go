package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

type GitHubWorkloadImpact struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RepositoryImpact shows only workloads owned by the same workspace, including
// explicitly declared dependencies. It never exposes other installation users.
func (s *SourceControlService) repositoryImpact(ctx context.Context, teamID, sourceControlID, remoteID string) ([]GitHubWorkloadImpact, error) {
	out := []GitHubWorkloadImpact{}
	var repo models.SourceControlRepository
	if err := s.DB().WithContext(ctx).Where("source_control_id = ? AND git_hub_repository_id = ?", sourceControlID, remoteID).First(&repo).Error; err != nil {
		return out, err
	}
	localID := fmt.Sprint(repo.ID)
	contains := func(raw any) bool {
		b, err := json.Marshal(raw)
		if err != nil {
			return false
		}
		var ids []string
		if json.Unmarshal(b, &ids) != nil {
			return false
		}
		for _, id := range ids {
			if id == localID {
				return true
			}
		}
		return false
	}
	if s.DB().Migrator().HasTable("sites") {
		var rows []struct {
			ID                          string
			Address                     string
			SourceControlID             *string
			SourceControlRepositoriesID *uint64
			RepositoryDependencyIDs     dbtype.JSONStringSlice
		}
		if err := s.DB().WithContext(ctx).Table("sites").Select("id,address,source_control_id,source_control_repositories_id,repository_dependency_ids").Where("team_id = ?", teamID).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if (r.SourceControlID != nil && *r.SourceControlID == sourceControlID && r.SourceControlRepositoriesID != nil && *r.SourceControlRepositoriesID == repo.ID) || contains(r.RepositoryDependencyIDs) {
				out = append(out, GitHubWorkloadImpact{Kind: "site", ID: r.ID, Name: r.Address})
			}
		}
	}
	for _, table := range []struct{ name, kind string }{{"docker_applications", "application"}, {"docker_composes", "compose"}} {
		if !s.DB().Migrator().HasTable(table.name) {
			continue
		}
		var rows []struct {
			ID           string
			Name         string
			SourceConfig dbtype.JSONMap
		}
		if err := s.DB().WithContext(ctx).Table(table.name).Select("id,name,source_config").Where("team_id = ? AND deleted_at IS NULL", teamID).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if (r.SourceConfig.GetString("source_control_id") == sourceControlID && r.SourceConfig.GetString("repository_id") == localID) || contains(r.SourceConfig["dependency_repository_ids"]) {
				out = append(out, GitHubWorkloadImpact{Kind: table.kind, ID: r.ID, Name: r.Name})
			}
		}
	}
	return out, nil
}
