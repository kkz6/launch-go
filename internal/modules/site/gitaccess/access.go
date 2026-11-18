// Package gitaccess binds site operations to selected GitHub repository grants.
package gitaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"gorm.io/gorm"
)

func Resolve(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, site *models.Site, operation gitaccess.Operation, expected *gitaccess.Snapshot) (*gitaccess.RepositoryAccess, error) {
	if site.SourceControlID == nil && site.SourceControlRepositoriesID == nil {
		return nil, nil
	}
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil || *site.SourceControlID == "" || *site.SourceControlRepositoriesID == 0 {
		return nil, fmt.Errorf("select a connection and a repository together")
	}
	var sc gitmodels.SourceControl
	if err := db.WithContext(ctx).Where("id = ? AND team_id = ?", *site.SourceControlID, site.TeamID).First(&sc).Error; err != nil {
		return nil, fmt.Errorf("selected source connection is unavailable")
	}
	var repo gitmodels.SourceControlRepository
	if err := db.WithContext(ctx).Where("id = ? AND source_control_id = ?", *site.SourceControlRepositoriesID, sc.ID).First(&repo).Error; err != nil {
		return nil, fmt.Errorf("selected repository does not belong to this connection")
	}
	if sc.Provider != gittypes.GitProviderGitHub {
		return nil, nil
	}
	return gitaccess.NewResolver(db, factory).Resolve(ctx, gitaccess.Request{
		TeamID: site.TeamID, SourceControlID: sc.ID, RepositoryID: strconv.FormatUint(repo.ID, 10), Operation: operation, Expected: expected,
	})
}

func Capture(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, site *models.Site) (dbtype.JSONMap, error) {
	grant, err := Resolve(ctx, db, factory, site, gitaccess.Clone, nil)
	if err != nil || grant == nil {
		return nil, err
	}
	data, err := json.Marshal(grant.Snapshot())
	if err != nil {
		return nil, err
	}
	result := dbtype.JSONMap{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	dependencies := map[string]any{}
	for _, id := range site.RepositoryDependencyIDs {
		dep, err := ResolveDependency(ctx, db, factory, site.TeamID, id, nil)
		if err != nil {
			return nil, err
		}
		dependencies[id] = dep.Snapshot()
	}
	if len(dependencies) > 0 {
		result["dependencies"] = dependencies
	}
	return result, nil
}

func Expected(raw map[string]any) (*gitaccess.Snapshot, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("repository authorization is missing; start a new deployment")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var snapshot gitaccess.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil || snapshot.ConnectionID == "" {
		return nil, fmt.Errorf("repository authorization is invalid; start a new deployment")
	}
	return &snapshot, nil
}

func ForDeployment(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, site *models.Site, deployment *models.Deployment, operation gitaccess.Operation) (*gitaccess.RepositoryAccess, error) {
	if deployment.SiteID != site.ID || deployment.TeamID != site.TeamID {
		return nil, fmt.Errorf("deployment does not belong to this site")
	}
	var expected *gitaccess.Snapshot
	if len(deployment.GitAccess) != 0 {
		var err error
		expected, err = Expected(deployment.GitAccess)
		if err != nil {
			return nil, err
		}
	}
	grant, err := Resolve(ctx, db, factory, site, operation, expected)
	if err != nil {
		return nil, err
	}
	if expected != nil && grant == nil {
		return nil, gitaccess.ErrStaleAccess
	}
	if grant != nil && expected == nil {
		return nil, fmt.Errorf("legacy deployment has no repository authorization; start a new deployment")
	}
	if grant != nil {
		if err := ValidateDependencySelection(site, deployment.GitAccess); err != nil {
			return nil, err
		}
	}
	return grant, nil
}

func ValidateDependencySelection(site *models.Site, raw map[string]any) error {
	dependencies, _ := raw["dependencies"].(map[string]any)
	if len(site.RepositoryDependencyIDs) != len(dependencies) {
		return gitaccess.ErrStaleAccess
	}
	seen := map[string]bool{}
	for _, id := range site.RepositoryDependencyIDs {
		if seen[id] {
			return gitaccess.ErrStaleAccess
		}
		seen[id] = true
		if _, ok := dependencies[id]; !ok {
			return gitaccess.ErrStaleAccess
		}
	}
	return nil
}

func ResolveDependency(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, teamID, repositoryID string, expected *gitaccess.Snapshot) (*gitaccess.RepositoryAccess, error) {
	var repo gitmodels.SourceControlRepository
	if err := db.WithContext(ctx).Where("id = ?", repositoryID).First(&repo).Error; err != nil {
		return nil, fmt.Errorf("dependency repository is unavailable")
	}
	return gitaccess.NewResolver(db, factory).Resolve(ctx, gitaccess.Request{
		TeamID: teamID, SourceControlID: repo.SourceControlID, RepositoryID: repositoryID, Operation: gitaccess.Clone, Expected: expected,
	})
}
