package access

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kkz6/launch-go/internal/modules/git/internal/capability"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Resource struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Branch        string `json:"branch,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
}

var cleanupDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var cleanupName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)
var workflowName = regexp.MustCompile(`^\.github/workflows/[a-zA-Z0-9_-]+\.ya?ml$`)

// RegisterCleanup is called before the first remote write so partial bootstrap
// failures still have a removal manifest. It cannot register arbitrary repos.
func RegisterCleanup(ctx context.Context, db *gorm.DB, sourceControlID, repositoryID string, expected Snapshot, resources []Resource) (string, error) {
	if len(resources) == 0 || len(resources) > 100 || expected.ConnectionID != sourceControlID || expected.RepositoryID != repositoryID {
		return "", ErrAccessDenied
	}
	var manifest models.GitHubCleanupManifest
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sc models.SourceControl
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ?", sourceControlID).Error != nil {
			return ErrAccessDenied
		}
		if err := ValidateSnapshot(ctx, tx, sc.TeamID, expected); err != nil {
			return err
		}
		var repo models.SourceControlRepository
		if tx.First(&repo, "id = ? AND source_control_id = ?", repositoryID, sc.ID).Error != nil {
			return ErrAccessDenied
		}
		// GitHub Actions secrets and variables belong to the repository, even
		// when separate launchctl accounts use different installations. Serialize
		// ownership by immutable GitHub identity before the first remote write.
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "github.com/repository/"+repo.GitHubRepositoryID).Error; err != nil {
				return err
			}
		}
		if err := rejectForeignAutomation(ctx, tx, sc.TeamID, repo.GitHubRepositoryID); err != nil {
			return err
		}
		seen := map[string]bool{}
		for i := range resources {
			r := &resources[i]
			switch r.Kind {
			case "workflow":
				if !workflowName.MatchString(r.Name) || !cleanupDigest.MatchString(r.ContentSHA256) {
					return ErrAccessDenied
				}
				if r.Branch == "" {
					r.Branch = repo.GetDefaultBranchOrMain()
				}
				if strings.ContainsAny(r.Branch, "\r\n") {
					return ErrAccessDenied
				}
			case "secret", "variable":
				if !cleanupName.MatchString(r.Name) || r.Branch != "" {
					return ErrAccessDenied
				}
			default:
				return ErrAccessDenied
			}
			key := r.Kind + "/" + r.Name + "/" + r.Branch
			if seen[key] {
				return ErrAccessDenied
			}
			seen[key] = true
		}
		data, err := json.Marshal(resources)
		if err != nil {
			return err
		}
		// Repeated bootstrap of the same exact capability reuses its manifest.
		err = tx.Where("source_control_id = ? AND repository_id = ? AND git_hub_repository_id = ? AND installation_id = ? AND connection_generation = ? AND grant_version = ? AND authorization_id = ? AND authorization_generation = ? AND installation_generation = ? AND status = ? AND resources = ?", sc.ID, repositoryID, repo.GitHubRepositoryID, sc.GetInstallationID(), expected.ConnectionGeneration, expected.GrantVersion, expected.AuthorizationID, expected.AuthorizationGeneration, expected.InstallationGeneration, "registered", string(data)).First(&manifest).Error
		if err == nil {
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		manifest = models.GitHubCleanupManifest{SourceControlID: sc.ID, TeamID: sc.TeamID, RepositoryID: repositoryID, GitHubRepositoryID: repo.GitHubRepositoryID, RepositoryFullName: repo.FullName, InstallationID: sc.GetInstallationID(), AuthorizationID: expected.AuthorizationID, AuthorizationGeneration: expected.AuthorizationGeneration, InstallationGeneration: expected.InstallationGeneration, ConnectionGeneration: expected.ConnectionGeneration, GrantVersion: expected.GrantVersion, Resources: string(data), Status: "registered"}
		return tx.Create(&manifest).Error
	})
	return manifest.ID, err
}

func rejectForeignAutomation(ctx context.Context, db *gorm.DB, teamID, repositoryID string) error {
	var count int64
	if err := db.WithContext(ctx).Model(&models.GitHubCleanupManifest{}).Where("git_hub_repository_id = ? AND team_id <> ? AND status <> ?", repositoryID, teamID, "completed").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrAccessDenied
	}
	return nil
}

func (a *RepositoryAccess) validateAutomationOwnership(ctx context.Context, db *gorm.DB, requireOwned bool) error {
	switch a.request.Operation {
	case ActionsBootstrap, ActionsRead, ActionsWrite, WorkflowWrite, SecretsWrite:
	default:
		return nil
	}
	if err := rejectForeignAutomation(ctx, db, a.request.TeamID, a.remoteRepositoryID); err != nil {
		return err
	}
	if !requireOwned {
		return nil
	}
	var count int64
	err := db.WithContext(ctx).Model(&models.GitHubCleanupManifest{}).Where("source_control_id = ? AND repository_id = ? AND git_hub_repository_id = ? AND team_id = ? AND status = ? AND connection_generation = ? AND grant_version = ? AND authorization_id = ? AND authorization_generation = ? AND installation_generation = ?", a.snapshot.ConnectionID, a.snapshot.RepositoryID, a.remoteRepositoryID, a.request.TeamID, "registered", a.snapshot.ConnectionGeneration, a.snapshot.GrantVersion, a.snapshot.AuthorizationID, a.snapshot.AuthorizationGeneration, a.snapshot.InstallationGeneration).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrAccessDenied
	}
	return nil
}

// ActivateCleanup runs in the revocation transaction after the authority update.
// Empty repositoryID activates all manifests for the connection. Explicit
// workload disable may activate its manifest before grant revocation as well.
func ActivateCleanup(ctx context.Context, db *gorm.DB, sourceControlID, repositoryID string) error {
	var sc models.SourceControl
	if db.WithContext(ctx).First(&sc, "id = ?", sourceControlID).Error != nil {
		return ErrAccessDenied
	}
	query := db.WithContext(ctx).Where("source_control_id = ? AND status = ?", sc.ID, "registered")
	if repositoryID != "" {
		query = query.Where("repository_id = ?", repositoryID)
	}
	var manifests []models.GitHubCleanupManifest
	if err := query.Find(&manifests).Error; err != nil {
		return err
	}
	expires := time.Now().Add(15 * time.Minute)
	for _, m := range manifests {
		var repo models.SourceControlRepository
		if db.WithContext(ctx).First(&repo, "id = ? AND source_control_id = ?", m.RepositoryID, sc.ID).Error != nil {
			return ErrAccessDenied
		}
		if err := db.WithContext(ctx).Model(&m).Where("status = ?", "registered").Updates(map[string]any{"status": "pending", "connection_generation": sc.Generation, "grant_version": repo.GrantVersion, "expires_at": expires}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ActivateCleanupManifest is for explicitly disabling one workload. It does not
// affect resources belonging to other workloads using the same repository.
func ActivateCleanupManifest(ctx context.Context, db *gorm.DB, id string) error {
	var m models.GitHubCleanupManifest
	if db.WithContext(ctx).First(&m, "id = ?", id).Error != nil {
		return ErrAccessDenied
	}
	if m.Status != "registered" {
		return nil
	}
	var sc models.SourceControl
	var repo models.SourceControlRepository
	if db.WithContext(ctx).First(&sc, "id = ? AND team_id = ?", m.SourceControlID, m.TeamID).Error != nil || db.WithContext(ctx).First(&repo, "id = ? AND source_control_id = ?", m.RepositoryID, sc.ID).Error != nil {
		return ErrAccessDenied
	}
	return db.WithContext(ctx).Model(&m).Where("status = ?", "registered").Updates(map[string]any{"status": "pending", "connection_generation": sc.Generation, "grant_version": repo.GrantVersion, "expires_at": time.Now().Add(15 * time.Minute)}).Error
}

type CleanupAccess struct {
	InstallationID     string
	RepositoryFullName string
	Resources          []Resource
	db                 *gorm.DB
	factory            *providers.ProviderFactory
	manifest           models.GitHubCleanupManifest
	tokenMu            sync.Mutex
	leaseID            string
}

func ResolveCleanup(ctx context.Context, db *gorm.DB, factory *providers.ProviderFactory, id string) (*CleanupAccess, error) {
	var manifest models.GitHubCleanupManifest
	if db.WithContext(ctx).First(&manifest, "id = ?", id).Error != nil {
		return nil, ErrAccessDenied
	}
	a := &CleanupAccess{db: db, factory: factory, manifest: manifest, InstallationID: manifest.InstallationID, RepositoryFullName: manifest.RepositoryFullName}
	if err := json.Unmarshal([]byte(manifest.Resources), &a.Resources); err != nil {
		return nil, err
	}
	if err := a.Validate(ctx); err != nil {
		return nil, err
	}
	return a, nil
}
func (a *CleanupAccess) Context(ctx context.Context) context.Context {
	return capability.With(ctx, a)
}
func (a *CleanupAccess) Lock()   { a.tokenMu.Lock() }
func (a *CleanupAccess) Unlock() { a.tokenMu.Unlock() }
func (a *CleanupAccess) Validate(ctx context.Context) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return a.validate(ctx, tx) })
}
func (a *CleanupAccess) validate(ctx context.Context, db *gorm.DB) error {
	m := a.manifest
	var stored models.GitHubCleanupManifest
	if db.First(&stored, "id = ?", m.ID).Error != nil || (stored.Status != "pending" && stored.Status != "failed_cleanup") || stored.ExpiresAt == nil || !stored.ExpiresAt.After(time.Now()) || stored.ConnectionGeneration != m.ConnectionGeneration || stored.GrantVersion != m.GrantVersion {
		return ErrStaleAccess
	}
	var sc models.SourceControl
	if db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ? AND team_id = ?", m.SourceControlID, m.TeamID).Error != nil || sc.Generation != m.ConnectionGeneration || sc.GitHubAuthorizationID == nil || *sc.GitHubAuthorizationID != m.AuthorizationID {
		return ErrStaleAccess
	}
	var repo models.SourceControlRepository
	if db.First(&repo, "id = ? AND source_control_id = ?", m.RepositoryID, sc.ID).Error != nil || repo.GrantVersion != m.GrantVersion || repo.GitHubRepositoryID != m.GitHubRepositoryID || repo.FullName != m.RepositoryFullName {
		return ErrStaleAccess
	}
	var authorization models.GitHubAuthorization
	if db.First(&authorization, "id = ?", m.AuthorizationID).Error != nil || authorization.RevokedAt != nil || authorization.Generation != m.AuthorizationGeneration {
		return ErrStaleAccess
	}
	var installation models.GitHubInstallation
	if sc.GitHubInstallationID == nil || db.First(&installation, "id = ?", *sc.GitHubInstallationID).Error != nil || installation.Status != "active" || installation.Generation != m.InstallationGeneration {
		return ErrStaleAccess
	}
	return RequireManager(ctx, db, sc.TeamID, authorization.UserID)
}
func (a *CleanupAccess) ValidateRequest(ctx context.Context, method, path string, body any) error {
	if err := a.Validate(ctx); err != nil {
		return err
	}
	u, err := url.Parse(path)
	if err != nil || u.IsAbs() || u.Host != "" || u.RawPath != "" || u.Fragment != "" {
		return ErrAccessDenied
	}
	if method == http.MethodPost && u.Path == "/app/installations/"+a.manifest.InstallationID+"/access_tokens" && u.RawQuery == "" {
		return nil
	}
	prefix := "/repos/" + a.manifest.RepositoryFullName
	var resources []Resource
	if json.Unmarshal([]byte(a.manifest.Resources), &resources) != nil {
		return ErrAccessDenied
	}
	for _, r := range resources {
		switch r.Kind {
		case "secret", "variable":
			suffix := "/actions/secrets/"
			if r.Kind == "variable" {
				suffix = "/actions/variables/"
			}
			if method == http.MethodDelete && u.Path == prefix+suffix+r.Name && u.RawQuery == "" {
				return nil
			}
		case "workflow":
			if u.Path != prefix+"/contents/"+r.Name {
				continue
			}
			if method == http.MethodGet && u.Query().Get("ref") == r.Branch && len(u.Query()) == 1 && len(u.Query()["ref"]) == 1 {
				return nil
			}
			if method == http.MethodDelete && u.RawQuery == "" {
				data, ok := body.(map[string]interface{})
				if ok && data["branch"] == r.Branch && data["sha"] != "" {
					return nil
				}
			}
		}
	}
	return ErrAccessDenied
}
func (a *CleanupAccess) TokenScope(ctx context.Context, installationID string) ([]int64, map[string]string, error) {
	if installationID != a.manifest.InstallationID {
		return nil, nil, ErrAccessDenied
	}
	if err := a.Validate(ctx); err != nil {
		return nil, nil, err
	}
	p, err := GitHubProvider(a.factory)
	if err != nil {
		return nil, nil, err
	}
	authorization, token, err := UserToken(providers.WithoutRepositoryAuthorization(ctx), a.db, p, a.manifest.AuthorizationID)
	if err != nil {
		return nil, nil, err
	}
	admin, err := p.UserRepositoryAdmin(providers.WithoutRepositoryAuthorization(ctx), token, a.manifest.RepositoryFullName, authorization.Login)
	if err != nil {
		return nil, nil, err
	}
	if !admin {
		return nil, nil, ErrAccessDenied
	}
	id, err := strconv.ParseInt(a.manifest.GitHubRepositoryID, 10, 64)
	if err != nil || id <= 0 {
		return nil, nil, ErrAccessDenied
	}
	permissions := map[string]string{}
	var resources []Resource
	if json.Unmarshal([]byte(a.manifest.Resources), &resources) != nil {
		return nil, nil, ErrAccessDenied
	}
	for _, r := range resources {
		switch r.Kind {
		case "workflow":
			permissions["contents"] = "write"
			permissions["workflows"] = "write"
		case "secret":
			permissions["secrets"] = "write"
		case "variable":
			permissions["variables"] = "write"
		}
	}
	lease := models.GitHubCredentialLease{CleanupManifestID: &a.manifest.ID, TeamID: a.manifest.TeamID, SourceControlID: a.manifest.SourceControlID, ConnectionGeneration: a.manifest.ConnectionGeneration, GrantVersion: a.manifest.GrantVersion, AuthorizationID: a.manifest.AuthorizationID, AuthorizationGeneration: a.manifest.AuthorizationGeneration, InstallationID: a.manifest.InstallationID, Status: "issuing"}
	lease.RepositoryID, _ = strconv.ParseUint(a.manifest.RepositoryID, 10, 64)
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := a.validate(ctx, tx); err != nil {
			return err
		}
		return tx.Create(&lease).Error
	})
	if err != nil {
		return nil, nil, err
	}
	a.leaseID = lease.ID
	return []int64{id}, permissions, nil
}
func (a *CleanupAccess) RecordToken(ctx context.Context, token string, expires time.Time) error {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := a.validate(ctx, tx); err != nil {
			return err
		}
		result := tx.Model(&models.GitHubCredentialLease{}).Where("id = ? AND status = ?", a.leaseID, "issuing").Updates(map[string]any{"token": dbtype.EncryptedString(token), "expires_at": expires, "status": "released"})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrStaleAccess
		}
		return nil
	})
	if err != nil {
		_ = a.db.WithContext(context.WithoutCancel(ctx)).Model(&models.GitHubCredentialLease{}).Where("id = ?", a.leaseID).Updates(map[string]any{"token": dbtype.EncryptedString(token), "expires_at": expires, "status": "revoke_pending"}).Error
	}
	return err
}
func (a *CleanupAccess) close(ctx context.Context, status string) error {
	if err := a.db.WithContext(ctx).Model(&models.GitHubCredentialLease{}).Where("cleanup_manifest_id = ? AND status IN ?", a.manifest.ID, []string{"issuing", "released"}).Update("status", "revoke_pending").Error; err != nil {
		return err
	}
	revokeErr := NewResolver(a.db, a.factory).revokePending(ctx, map[string]any{"cleanup_manifest_id": a.manifest.ID})
	if revokeErr != nil {
		status = "failed_cleanup"
	}
	return errors.Join(revokeErr, a.db.WithContext(ctx).Model(&models.GitHubCleanupManifest{}).Where("id = ? AND status IN ? AND connection_generation = ? AND grant_version = ?", a.manifest.ID, []string{"pending", "failed_cleanup"}, a.manifest.ConnectionGeneration, a.manifest.GrantVersion).Update("status", status).Error)
}
func (a *CleanupAccess) Complete(ctx context.Context) error { return a.close(ctx, "completed") }
func (a *CleanupAccess) Fail(ctx context.Context) error     { return a.close(ctx, "failed_cleanup") }
