// Package gitaccess applies the common repository grant to Docker workloads.
package gitaccess

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"gorm.io/gorm"
)

// Resolve only permits a connected source to use a repository selected by its
// workload's team. Async callers must supply the snapshot captured at enqueue.
func Resolve(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, teamID string, config map[string]any, operation access.Operation, expected *access.Snapshot, queued bool) (*access.RepositoryAccess, error) {
	scID, _ := config["source_control_id"].(string)
	repoID, _ := config["repository_id"].(string)
	if strings.TrimSpace(scID) == "" {
		if repoID != "" || expected != nil || config["dependency_repository_ids"] != nil {
			return nil, errors.New("repository requires a selected connection")
		}
		if repository, _ := config["repo"].(string); repository != "" {
			if err := ValidatePublicRepositoryURL(repository); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if db == nil || teamID == "" {
		return nil, errors.New("repository access unavailable")
	}
	var sc gitmodels.SourceControl
	if err := db.WithContext(ctx).Where("id = ? AND team_id = ?", scID, teamID).First(&sc).Error; err != nil {
		return nil, errors.New("repository access unavailable")
	}
	if sc.Provider != gittypes.GitProviderGitHub {
		return nil, errors.New("connected repository provider is not supported for Docker deployments")
	}
	if queued && expected == nil {
		return nil, errors.New("repository authorization missing from queued work; start a new deployment")
	}
	repository, _ := config["repo"].(string)
	// Old GHA config stored owner and repo separately.
	if owner, _ := config["owner"].(string); owner != "" && !strings.Contains(repository, "/") {
		repository = owner + "/" + repository
	}
	resolved, err := access.NewResolver(db, factory).Resolve(ctx, access.Request{TeamID: teamID, SourceControlID: scID, RepositoryID: repoID, Repository: repository, Operation: operation, Expected: expected})
	if err != nil {
		return nil, err
	}
	// GitHub Actions secrets are repository-wide. A CI bootstrap claims the
	// repository through its cleanup manifest before the first remote write.
	// Merely selecting the same repository in another workspace must not
	// interrupt the existing owner's workflows or callbacks.
	if operation == access.ActionsBootstrap || operation == access.ActionsWrite || operation == access.SecretsWrite || operation == access.WorkflowWrite {
		var count int64
		// Local disconnect alone cannot prove remote CI secrets were removed.
		// Failed/expired cleanup continues to block cross-workspace CI reuse.
		if err := db.WithContext(ctx).Model(&gitmodels.GitHubCleanupManifest{}).Where("git_hub_repository_id = ? AND team_id <> ? AND status <> ?", resolved.GitHubRepositoryID, teamID, "completed").Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("GitHub Actions requires completed cleanup of another workspace's repository resources")
		}
	}
	return resolved, nil
}

// Public clones cannot use local checkouts, SSH agents, or external transport
// helpers as an alternate source of private repository authority.
func ValidatePublicRepositoryURL(repository string) error {
	u, err := url.Parse(repository)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(repository, "\r\n\t") {
		return errors.New("public repository requires an HTTPS URL without credentials")
	}
	return nil
}

func Bind(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, teamID string, config map[string]any, operation access.Operation) (*access.Snapshot, error) {
	resolved, err := Resolve(ctx, db, factory, teamID, config, operation, nil, false)
	if err != nil || resolved == nil {
		return nil, err
	}
	dependencies, err := ResolveDependencies(ctx, db, factory, teamID, config, nil, false)
	if err != nil {
		return nil, err
	}
	if operation == access.ActionsBootstrap && len(dependencies) > 0 {
		return nil, errors.New("private dependencies currently require server builds; GitHub Actions cannot refresh their scoped credentials")
	}
	snapshots := make([]access.Snapshot, 0, len(dependencies))
	for _, dependency := range dependencies {
		snapshots = append(snapshots, dependency.Snapshot())
	}
	config["git_dependencies"] = snapshots
	snapshot := resolved.Snapshot()
	config["repository_id"] = strconv.FormatUint(resolved.Repository.ID, 10)
	config["repo"] = resolved.CloneURL
	access.StoreSnapshot(config, snapshot)
	return &snapshot, nil
}

// BootstrapSnapshot is separate from git_access: reconnecting and deploying
// must never revive an old workflow callback bearer.
func BootstrapSnapshot(config map[string]any) (*access.Snapshot, error) {
	return access.SnapshotFromMap(map[string]any{"git_access": config["gha_access"]})
}
func StoreBootstrapSnapshot(config map[string]any, snapshot access.Snapshot) {
	temp := map[string]any{}
	access.StoreSnapshot(temp, snapshot)
	config["gha_access"] = temp["git_access"]
}

// Dependencies are opt-in workload bindings to independently selected grants.
// A dependency may use another connection/installation in the same team.
func ResolveDependencies(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, teamID string, config map[string]any, expected []access.Snapshot, queued bool) ([]*access.RepositoryAccess, error) {
	raw, exists := config["dependency_repository_ids"]
	if !exists {
		if len(expected) > 0 {
			return nil, errors.New("dependency selection changed")
		}
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var ids []string
	if json.Unmarshal(data, &ids) != nil {
		return nil, errors.New("invalid dependency repository selection")
	}
	if len(ids) > 20 {
		return nil, errors.New("at most 20 private dependencies can be selected")
	}
	if queued && len(ids) != len(expected) {
		return nil, errors.New("dependency authorization changed; start a new deployment")
	}
	seen := map[string]bool{}
	out := make([]*access.RepositoryAccess, 0, len(ids))
	for index, id := range ids {
		if seen[id] {
			return nil, errors.New("duplicate dependency repository")
		}
		seen[id] = true
		var repo gitmodels.SourceControlRepository
		if db.WithContext(ctx).Table("source_control_repositories AS r").Select("r.*").Joins("JOIN source_controls AS s ON s.id = r.source_control_id").Where("r.id = ? AND s.team_id = ?", id, teamID).Take(&repo).Error != nil {
			return nil, errors.New("dependency repository is not selected for this workspace")
		}
		request := access.Request{TeamID: teamID, SourceControlID: repo.SourceControlID, RepositoryID: id, Operation: access.Clone}
		if queued {
			request.Expected = &expected[index]
		}
		resolved, err := access.NewResolver(db, factory).Resolve(ctx, request)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

func PinnedSnapshots(config map[string]any, primary *access.Snapshot) []*access.Snapshot {
	out := []*access.Snapshot{primary}
	data, err := json.Marshal(config["git_dependencies"])
	if err != nil {
		return out
	}
	var snapshots []access.Snapshot
	if json.Unmarshal(data, &snapshots) != nil {
		return out
	}
	for i := range snapshots {
		out = append(out, &snapshots[i])
	}
	return out
}
