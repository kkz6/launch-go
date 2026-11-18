// Package access is the authorization boundary for GitHub repository consumers.
package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kkz6/launch-go/internal/modules/git/internal/capability"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

type Operation string

const (
	Clone            Operation = "clone"
	Read             Operation = "read"
	Deployment       Operation = "deployment"
	ActionsRead      Operation = "actions_read"
	ActionsWrite     Operation = "actions_write"
	WorkflowWrite    Operation = "workflow_write"
	SecretsWrite     Operation = "secrets_write"
	ActionsBootstrap Operation = "actions_bootstrap"
)

var ErrAccessDenied = fiberutil.Forbidden("Repository is not selected or its GitHub authorization needs reconnecting")
var ErrStaleAccess = fiberutil.Conflict("Repository access changed; start a new deployment")

type Snapshot struct {
	ConnectionID            string `json:"connection_id"`
	ConnectionGeneration    uint64 `json:"connection_generation"`
	RepositoryID            string `json:"repository_id"`
	GrantVersion            uint64 `json:"grant_version"`
	AuthorizationID         string `json:"authorization_id"`
	AuthorizationGeneration uint64 `json:"authorization_generation"`
	InstallationGeneration  uint64 `json:"installation_generation"`
}

type Request struct {
	TeamID          string
	SourceControlID string
	RepositoryID    string
	Repository      string
	Operation       Operation
	Expected        *Snapshot
}

type Resolver struct {
	db      *gorm.DB
	factory *providers.ProviderFactory
}

func NewResolver(db *gorm.DB, factory *providers.ProviderFactory) *Resolver {
	return &Resolver{db: db, factory: factory}
}

type RepositoryAccess struct {
	repositoryID         uint64
	remoteRepositoryID   string
	remoteInstallationID string
	canonicalFullName    string
	tokenMu              sync.Mutex
	SourceControl        *models.SourceControl
	Repository           *models.SourceControlRepository
	InstallationID       string
	GitHubRepositoryID   string
	CloneURL             string
	resolver             *Resolver
	request              Request
	snapshot             Snapshot
	permissions          map[string]string
	leaseID              string
}

func (a *RepositoryAccess) Snapshot() Snapshot { return a.snapshot }
func (a *RepositoryAccess) Context(ctx context.Context) context.Context {
	return capability.With(ctx, a)
}

func SnapshotFromMap(config map[string]any) (*Snapshot, error) {
	value, ok := config["git_access"]
	if !ok || value == nil {
		return nil, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, ErrStaleAccess
	}
	var snapshot Snapshot
	if json.Unmarshal(b, &snapshot) != nil || snapshot.ConnectionID == "" || snapshot.RepositoryID == "" || snapshot.ConnectionGeneration == 0 || snapshot.GrantVersion == 0 || snapshot.AuthorizationID == "" || snapshot.AuthorizationGeneration == 0 || snapshot.InstallationGeneration == 0 {
		return nil, ErrStaleAccess
	}
	return &snapshot, nil
}
func StoreSnapshot(config map[string]any, snapshot Snapshot) { config["git_access"] = snapshot }

var repositoryComponent = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// CanonicalRepository rejects credential-bearing or foreign-host URLs. It is
// used only to resolve an existing selected grant, never to import a repository.
func CanonicalRepository(value string) (string, error) {
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
			return "", ErrAccessDenied
		}
		value = strings.TrimPrefix(u.Path, "/")
	} else if strings.HasPrefix(value, "git@github.com:") {
		value = strings.TrimPrefix(value, "git@github.com:")
	}
	value = strings.TrimSuffix(value, ".git")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrAccessDenied
	}
	for _, part := range parts {
		if part == "." || part == ".." || !repositoryComponent.MatchString(part) {
			return "", ErrAccessDenied
		}
	}
	return value, nil
}

func (r *Resolver) Resolve(ctx context.Context, req Request) (*RepositoryAccess, error) {
	if r.db == nil || req.TeamID == "" || req.SourceControlID == "" {
		return nil, ErrAccessDenied
	}
	permissions, err := operationPermissions(req.Operation)
	if err != nil {
		return nil, err
	}
	var sc models.SourceControl
	if err = r.db.WithContext(ctx).Where("id = ? AND team_id = ?", req.SourceControlID, req.TeamID).First(&sc).Error; err != nil {
		return nil, ErrAccessDenied
	}
	if string(sc.Provider) != "github" || sc.Status != "active" || sc.GitHubAuthorizationID == nil || sc.GitHubInstallationID == nil || sc.GetInstallationID() == "" {
		return nil, ErrAccessDenied
	}
	q := r.db.WithContext(ctx).Where("source_control_id = ? AND selected = ? AND available = ?", sc.ID, true, true)
	if req.RepositoryID != "" {
		id, err := strconv.ParseUint(req.RepositoryID, 10, 64)
		if err != nil || id == 0 {
			return nil, ErrAccessDenied
		}
		q = q.Where("id = ?", id)
	} else {
		name, err := CanonicalRepository(req.Repository)
		if err != nil {
			return nil, err
		}
		q = q.Where("LOWER(full_name) = ?", strings.ToLower(name))
	}
	var repo models.SourceControlRepository
	if q.First(&repo).Error != nil || repo.GitHubRepositoryID == "" {
		return nil, ErrAccessDenied
	}
	if req.Repository != "" {
		name, err := CanonicalRepository(req.Repository)
		if err != nil || !strings.EqualFold(name, repo.FullName) {
			return nil, ErrAccessDenied
		}
	}
	var authorization models.GitHubAuthorization
	if r.db.WithContext(ctx).First(&authorization, "id = ?", *sc.GitHubAuthorizationID).Error != nil || authorization.RevokedAt != nil {
		return nil, ErrAccessDenied
	}
	var installation models.GitHubInstallation
	if r.db.WithContext(ctx).First(&installation, "id = ?", *sc.GitHubInstallationID).Error != nil || installation.Status != "active" || installation.InstallationID != sc.GetInstallationID() {
		return nil, ErrAccessDenied
	}
	if !permissionsAllowed(installation.Permissions, permissions) {
		return nil, ErrAccessDenied
	}
	if err := RequireManager(ctx, r.db, sc.TeamID, authorization.UserID); err != nil {
		return nil, ErrAccessDenied
	}
	snapshot := Snapshot{ConnectionID: sc.ID, ConnectionGeneration: sc.Generation, RepositoryID: strconv.FormatUint(repo.ID, 10), GrantVersion: repo.GrantVersion, AuthorizationID: authorization.ID, AuthorizationGeneration: authorization.Generation, InstallationGeneration: installation.Generation}
	if req.Expected != nil && *req.Expected != snapshot {
		return nil, ErrStaleAccess
	}
	a := &RepositoryAccess{SourceControl: &sc, Repository: &repo, InstallationID: sc.GetInstallationID(), GitHubRepositoryID: repo.GitHubRepositoryID, CloneURL: "https://github.com/" + repo.FullName + ".git", repositoryID: repo.ID, remoteRepositoryID: repo.GitHubRepositoryID, remoteInstallationID: sc.GetInstallationID(), canonicalFullName: repo.FullName, resolver: r, request: req, snapshot: snapshot, permissions: permissions}
	if err := a.verifySponsor(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *RepositoryAccess) Validate(ctx context.Context) error {
	ctx = providers.WithoutRepositoryAuthorization(ctx)
	return a.resolver.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return a.validateLocal(ctx, tx) })
}

func (a *RepositoryAccess) validateLocal(ctx context.Context, db *gorm.DB) error {
	var sc models.SourceControl
	if db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ? AND team_id = ?", a.snapshot.ConnectionID, a.request.TeamID).Error != nil || sc.Status != "active" || sc.Generation != a.snapshot.ConnectionGeneration || sc.GitHubAuthorizationID == nil || *sc.GitHubAuthorizationID != a.snapshot.AuthorizationID {
		return ErrStaleAccess
	}
	var repo models.SourceControlRepository
	if db.First(&repo, "id = ? AND source_control_id = ?", a.snapshot.RepositoryID, sc.ID).Error != nil || !repo.Selected || !repo.Available || repo.GrantVersion != a.snapshot.GrantVersion {
		return ErrStaleAccess
	}
	var authorization models.GitHubAuthorization
	if db.First(&authorization, "id = ?", a.snapshot.AuthorizationID).Error != nil || authorization.RevokedAt != nil || authorization.Generation != a.snapshot.AuthorizationGeneration {
		return ErrStaleAccess
	}
	var installation models.GitHubInstallation
	if sc.GitHubInstallationID == nil || db.First(&installation, "id = ?", *sc.GitHubInstallationID).Error != nil || installation.Status != "active" || installation.Generation != a.snapshot.InstallationGeneration {
		return ErrStaleAccess
	}
	if err := RequireManager(ctx, db, sc.TeamID, authorization.UserID); err != nil {
		return ErrAccessDenied
	}
	if repo.VerifiedAt == nil || repo.VerifiedAt.Before(time.Now().Add(-5*time.Minute)) {
		return ErrAccessDenied
	}
	return a.validateAutomationOwnership(ctx, db, false)
}

// Repository-wide automation requests require an owned cleanup registration.
// Resolve may precede registration during bootstrap, but no credential or
// remote operation is permitted until that claim is persisted.
func (a *RepositoryAccess) ValidateRequest(ctx context.Context, _ string, _ string, _ any) error {
	return a.validateAutomationOwnership(ctx, a.resolver.db, true)
}

func (a *RepositoryAccess) verifySponsor(ctx context.Context) error {
	if a.Repository.VerifiedAt != nil && a.Repository.VerifiedAt.After(time.Now().Add(-5*time.Minute)) {
		return a.Validate(ctx)
	}
	ctx = providers.WithoutRepositoryAuthorization(ctx)
	provider, err := GitHubProvider(a.resolver.factory)
	if err != nil {
		return err
	}
	user, token, err := UserToken(ctx, a.resolver.db, provider, a.snapshot.AuthorizationID)
	if err != nil {
		return err
	}
	repositories, err := provider.GetUserInstallationRepositories(ctx, token, a.remoteInstallationID)
	if err != nil {
		return err
	}
	found := false
	for _, repo := range repositories {
		if providers.ExtractFloatID(repo, "id") == a.remoteRepositoryID && repo["full_name"] == a.canonicalFullName {
			found = true
			break
		}
	}
	if !found {
		a.invalidateGrant(ctx)
		return ErrAccessDenied
	}
	admin, err := provider.UserRepositoryAdmin(ctx, token, a.canonicalFullName, user.Login)
	if err != nil {
		return err
	}
	if !admin {
		a.invalidateGrant(ctx)
		return ErrAccessDenied
	}
	installation, err := provider.GetInstallation(ctx, a.remoteInstallationID)
	if err != nil {
		return err
	}
	if installation.IsSuspended() {
		return ErrAccessDenied
	}
	now := time.Now()
	err = a.resolver.db.WithContext(ctx).Model(&models.SourceControlRepository{}).Where("id = ? AND grant_version = ? AND selected = ? AND available = ?", a.repositoryID, a.snapshot.GrantVersion, true, true).Update("verified_at", now).Error
	if err != nil {
		return err
	}
	a.Repository.VerifiedAt = &now
	return a.Validate(ctx)
}

func (a *RepositoryAccess) TokenScope(ctx context.Context, installationID string) ([]int64, map[string]string, error) {
	if installationID != a.remoteInstallationID {
		return nil, nil, ErrAccessDenied
	}
	if err := a.verifySponsor(ctx); err != nil {
		return nil, nil, err
	}
	id, err := strconv.ParseInt(a.remoteRepositoryID, 10, 64)
	if err != nil || id <= 0 {
		return nil, nil, ErrAccessDenied
	}
	lease := models.GitHubCredentialLease{TeamID: a.request.TeamID, SourceControlID: a.snapshot.ConnectionID, ConnectionGeneration: a.snapshot.ConnectionGeneration, RepositoryID: a.repositoryID, GrantVersion: a.snapshot.GrantVersion, AuthorizationID: a.snapshot.AuthorizationID, AuthorizationGeneration: a.snapshot.AuthorizationGeneration, InstallationID: a.remoteInstallationID, Status: "issuing"}
	err = a.resolver.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := a.validateLocal(ctx, tx); err != nil {
			return err
		}
		if err := a.validateAutomationOwnership(ctx, tx, true); err != nil {
			return err
		}
		return tx.Create(&lease).Error
	})
	if err != nil {
		return nil, nil, err
	}
	a.leaseID = lease.ID
	return []int64{id}, a.permissions, nil
}

func (a *RepositoryAccess) RecordToken(ctx context.Context, token string, expires time.Time) error {
	if a.leaseID == "" {
		return ErrAccessDenied
	}
	err := a.resolver.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := a.validateLocal(ctx, tx); err != nil {
			return err
		}
		if err := a.validateAutomationOwnership(ctx, tx, true); err != nil {
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
		_ = a.resolver.db.WithContext(context.WithoutCancel(ctx)).Model(&models.GitHubCredentialLease{}).Where("id = ?", a.leaseID).Updates(map[string]any{"token": dbtype.EncryptedString(token), "expires_at": expires, "status": "revoke_pending"}).Error
	}
	return err
}

func operationPermissions(op Operation) (map[string]string, error) {
	switch op {
	case Clone, Read:
		return map[string]string{"contents": "read"}, nil
	case Deployment:
		return map[string]string{"deployments": "write"}, nil
	case ActionsRead:
		return map[string]string{"actions": "read"}, nil
	case ActionsWrite:
		return map[string]string{"actions": "write"}, nil
	case WorkflowWrite:
		return map[string]string{"contents": "write", "workflows": "write"}, nil
	case SecretsWrite:
		return map[string]string{"secrets": "write", "variables": "write"}, nil
	case ActionsBootstrap:
		return map[string]string{"contents": "write", "workflows": "write", "actions": "write", "secrets": "write", "variables": "write"}, nil
	default:
		return nil, ErrAccessDenied
	}
}
func permissionsAllowed(raw string, wanted map[string]string) bool {
	var actual map[string]string
	if json.Unmarshal([]byte(raw), &actual) != nil {
		return false
	}
	for key, value := range wanted {
		if actual[key] != "write" && actual[key] != value {
			return false
		}
	}
	return true
}

func RequireManager(ctx context.Context, db *gorm.DB, teamID, userID string) error {
	if teamID == "" || userID == "" {
		return ErrAccessDenied
	}
	var userCount int64
	if db.WithContext(ctx).Table("users").Where("id = ? AND status = ?", userID, "active").Count(&userCount).Error != nil || userCount != 1 {
		return ErrAccessDenied
	}
	var team struct{ UserID string }
	if db.WithContext(ctx).Table("teams").Select("user_id").Where("id = ?", teamID).Take(&team).Error != nil {
		return ErrAccessDenied
	}
	if team.UserID == userID {
		return nil
	}
	var count int64
	if db.WithContext(ctx).Table("team_user").Where("team_id = ? AND user_id = ? AND role = ?", teamID, userID, "admin").Count(&count).Error != nil || count == 0 {
		return ErrAccessDenied
	}
	return nil
}

func GitHubProvider(factory *providers.ProviderFactory) (*providers.GitHubProvider, error) {
	if factory == nil {
		return nil, ErrAccessDenied
	}
	provider, err := factory.GetProvider(providers.GitProviderGitHub)
	if err != nil {
		return nil, err
	}
	gh, ok := provider.(*providers.GitHubProvider)
	if !ok {
		return nil, ErrAccessDenied
	}
	return gh, nil
}

// UserToken rotates one local authorization under a database lock. Refresh is
// deliberately not shared across separate launchctl identities.
func UserToken(ctx context.Context, db *gorm.DB, provider *providers.GitHubProvider, id string) (*models.GitHubAuthorization, string, error) {
	var authorization models.GitHubAuthorization
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&authorization, "id = ?", id).Error; err != nil {
			return ErrAccessDenied
		}
		if authorization.RevokedAt != nil || authorization.AccessToken == "" {
			return ErrAccessDenied
		}
		if authorization.ExpiresAt.After(time.Now().Add(time.Minute)) {
			return nil
		}
		if authorization.RefreshToken == "" || !authorization.RefreshExpiresAt.After(time.Now()) {
			return ErrAccessDenied
		}
		token, err := provider.RefreshUserToken(providers.WithoutRepositoryAuthorization(ctx), authorization.RefreshToken.String())
		if err != nil {
			return err
		}
		authorization.AccessToken = dbtype.EncryptedString(token.AccessToken)
		authorization.RefreshToken = dbtype.EncryptedString(token.RefreshToken)
		authorization.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		authorization.RefreshExpiresAt = time.Now().Add(time.Duration(token.RefreshTokenExpiresIn) * time.Second)
		return tx.Save(&authorization).Error
	})
	return &authorization, authorization.AccessToken.String(), err
}

// Disconnect invalidates access before attempting provider token revocations.
func (r *Resolver) Disconnect(ctx context.Context, sourceControlID, teamID string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sc models.SourceControl
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ? AND team_id = ?", sourceControlID, teamID).Error != nil {
			return ErrAccessDenied
		}
		if err := tx.Model(&sc).Updates(map[string]any{"status": "disconnected", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Updates(map[string]any{"grant_version": gorm.Expr("grant_version + 1"), "verified_at": nil}).Error; err != nil {
			return err
		}
		if err := ActivateCleanup(ctx, tx, sc.ID, ""); err != nil {
			return err
		}
		return tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ? AND status IN ?", sc.ID, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
	})
	if err != nil {
		return err
	}
	_ = r.RevokeConnectionPending(ctx, sourceControlID)
	return nil
}

func (r *Resolver) RevokePending(ctx context.Context) error {
	return r.revokePending(ctx, nil)
}

func (r *Resolver) RevokeConnectionPending(ctx context.Context, sourceControlID string) error {
	if sourceControlID == "" {
		return ErrAccessDenied
	}
	return r.revokePending(ctx, map[string]any{"source_control_id": sourceControlID})
}

func (r *Resolver) revokePending(ctx context.Context, scope map[string]any) error {
	provider, err := GitHubProvider(r.factory)
	if err != nil {
		return err
	}
	var leases []models.GitHubCredentialLease
	query := r.db.WithContext(ctx).Where("status = ?", "revoke_pending")
	if len(scope) > 0 {
		query = query.Where(scope)
	}
	if err = query.Find(&leases).Error; err != nil {
		return err
	}
	var failures []error
	for _, lease := range leases {
		if lease.Token == "" {
			continue
		}
		if lease.ExpiresAt.After(time.Now()) {
			if err := provider.RevokeInstallationToken(ctx, lease.Token.String()); err != nil {
				failures = append(failures, err)
				continue
			}
		}
		if err := r.db.WithContext(ctx).Model(&lease).Updates(map[string]any{"token": nil, "status": "revoked"}).Error; err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s Snapshot) String() string {
	return fmt.Sprintf("%s/%d/%s/%d", s.ConnectionID, s.ConnectionGeneration, s.RepositoryID, s.GrantVersion)
}

// ValidateSnapshot fences completion callbacks against persisted local authority.
// It cannot mint credentials and does not replace Resolve for upstream work.
func ValidateSnapshot(ctx context.Context, db *gorm.DB, teamID string, snapshot Snapshot) error {
	if db == nil || teamID == "" || snapshot.ConnectionID == "" || snapshot.RepositoryID == "" || snapshot.ConnectionGeneration == 0 || snapshot.GrantVersion == 0 || snapshot.AuthorizationID == "" || snapshot.AuthorizationGeneration == 0 || snapshot.InstallationGeneration == 0 {
		return ErrStaleAccess
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sc models.SourceControl
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ? AND team_id = ?", snapshot.ConnectionID, teamID).Error != nil || sc.Status != "active" || sc.Generation != snapshot.ConnectionGeneration || sc.GitHubAuthorizationID == nil || *sc.GitHubAuthorizationID != snapshot.AuthorizationID {
			return ErrStaleAccess
		}
		var repo models.SourceControlRepository
		if tx.First(&repo, "id = ? AND source_control_id = ?", snapshot.RepositoryID, sc.ID).Error != nil || !repo.Selected || !repo.Available || repo.GrantVersion != snapshot.GrantVersion {
			return ErrStaleAccess
		}
		var a models.GitHubAuthorization
		if tx.First(&a, "id = ?", snapshot.AuthorizationID).Error != nil || a.RevokedAt != nil || a.Generation != snapshot.AuthorizationGeneration {
			return ErrStaleAccess
		}
		var i models.GitHubInstallation
		if sc.GitHubInstallationID == nil || tx.First(&i, "id = ?", *sc.GitHubInstallationID).Error != nil || i.Status != "active" || i.Generation != snapshot.InstallationGeneration {
			return ErrStaleAccess
		}
		return RequireManager(ctx, tx, teamID, a.UserID)
	})
}

func (a *RepositoryAccess) Lock()   { a.tokenMu.Lock() }
func (a *RepositoryAccess) Unlock() { a.tokenMu.Unlock() }

func (a *RepositoryAccess) invalidateGrant(ctx context.Context) {
	err := a.resolver.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sc models.SourceControl
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ?", a.snapshot.ConnectionID).Error != nil || sc.Generation != a.snapshot.ConnectionGeneration {
			return ErrStaleAccess
		}
		result := tx.Model(&models.SourceControlRepository{}).Where("id = ? AND grant_version = ?", a.snapshot.RepositoryID, a.snapshot.GrantVersion).Updates(map[string]any{"available": false, "verified_at": nil, "grant_version": gorm.Expr("grant_version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrStaleAccess
		}
		if err := ActivateCleanup(ctx, tx, sc.ID, a.snapshot.RepositoryID); err != nil {
			return err
		}
		return tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ? AND repository_id = ? AND status IN ?", sc.ID, a.repositoryID, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
	})
	if err == nil {
		_ = a.resolver.revokePending(ctx, map[string]any{"source_control_id": a.snapshot.ConnectionID, "repository_id": a.repositoryID})
	}
}
