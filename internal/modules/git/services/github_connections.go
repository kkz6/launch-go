package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

var errAttempt = fiberutil.Forbidden("GitHub connection attempt expired or changed; start connecting again")

type BeginGitHubRequest struct {
	SourceControlID   string `json:"source_control_id"`
	AcknowledgeShared bool   `json:"acknowledge_shared"`
}
type GitHubSelectionRequest struct {
	SourceControlID          json.RawMessage `json:"source_control_id"`
	InstallationID           string          `json:"installation_id"`
	RepositoryIDs            []string        `json:"repository_ids"`
	ExpectedSelectionVersion *uint64         `json:"expected_selection_version"`
	ExpectedGeneration       *uint64         `json:"expected_generation"`
}
type GitHubAttemptResponse struct {
	AttemptID            string                     `json:"attempt_id"`
	Status               string                     `json:"status"`
	TeamID               string                     `json:"team_id"`
	URL                  string                     `json:"url,omitempty"`
	ExpiresAt            time.Time                  `json:"expires_at"`
	SourceControlID      *string                    `json:"source_control_id,omitempty"`
	ConnectionGeneration uint64                     `json:"connection_generation,omitempty"`
	SelectionVersion     uint64                     `json:"selection_version"`
	Installations        []GitHubInstallationChoice `json:"installations"`
	InstallURL           string                     `json:"install_url,omitempty"`
}
type GitHubInstallationChoice struct {
	ID           string `json:"id"`
	AccountLogin string `json:"account_login"`
	AccountType  string `json:"account_type"`
	AvatarURL    string `json:"avatar_url"`
}
type GitHubCandidatesResponse struct {
	Repositories         []GitHubRepositoryChoice `json:"repositories"`
	SourceControlID      *string                  `json:"source_control_id"`
	ConnectionGeneration uint64                   `json:"connection_generation"`
	SelectionVersion     uint64                   `json:"selection_version"`
}
type GitHubRepositoryChoice struct {
	Unavailable        bool                   `json:"unavailable,omitempty"`
	AffectedWorkloads  []GitHubWorkloadImpact `json:"affected_workloads,omitempty"`
	GitHubRepositoryID string                 `json:"github_repository_id"`
	FullName           string                 `json:"full_name"`
	Name               string                 `json:"name"`
	DefaultBranch      string                 `json:"default_branch"`
	Private            bool                   `json:"private"`
	Selected           bool                   `json:"selected"`
}

func secret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}
func digest(s string) string                  { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func str(m map[string]any, k string) string   { v, _ := m[k].(string); return v }
func boolean(m map[string]any, k string) bool { v, _ := m[k].(bool); return v }

// Membership changes invalidate a selection review, including pending invitations.
func membership(ctx context.Context, db *gorm.DB, teamID string) (string, bool, error) {
	var owner string
	if err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Table("teams").Select("user_id").Where("id = ?", teamID).Scan(&owner).Error; err != nil || owner == "" {
		return "", false, errAttempt
	}
	var members []struct {
		UserID string
		Role   string
	}
	if err := db.WithContext(ctx).Table("team_user").Select("user_id, role").Where("team_id = ?", teamID).Order("user_id").Scan(&members).Error; err != nil {
		return "", false, err
	}
	var invitations []struct {
		Email string
		Role  string
	}
	if err := db.WithContext(ctx).Table("team_invitations").Select("email, role").Where("team_id = ?", teamID).Order("email").Scan(&invitations).Error; err != nil {
		return "", false, err
	}
	shared := len(invitations) > 0
	for _, m := range members {
		if m.UserID != owner {
			shared = true
		}
	}
	b, _ := json.Marshal([]any{owner, members, invitations})
	return digest(string(b)), shared, nil
}
func (s *SourceControlService) BeginGitHub(ctx context.Context, teamID, userID, sessionID string, req BeginGitHubRequest) (GitHubAttemptResponse, error) {
	var out GitHubAttemptResponse
	if sessionID == "" {
		return out, errAttempt
	}
	if err := access.RequireManager(ctx, s.DB(), teamID, userID); err != nil {
		return out, err
	}
	hash, shared, err := membership(ctx, s.DB(), teamID)
	if err != nil {
		return out, err
	}
	if shared && !req.AcknowledgeShared {
		return out, fiberutil.Conflict("shared_workspace_confirmation_required")
	}
	p, err := access.GitHubProvider(s.ProviderFactory())
	if err != nil {
		return out, err
	}
	state, err := secret()
	if err != nil {
		return out, err
	}
	verifier, err := secret()
	if err != nil {
		return out, err
	}
	challenge := sha256.Sum256([]byte(verifier))
	target, err := p.AuthorizationURL(state, base64.RawURLEncoding.EncodeToString(challenge[:]))
	if err != nil {
		return out, err
	}
	a := models.GitHubConnectionAttempt{UserID: userID, TeamID: teamID, SessionHash: digest(sessionID), StateHash: digest(state), PKCEVerifier: dbtype.EncryptedString(verifier), MembershipHash: hash, Status: "authorizing", ExpiresAt: time.Now().Add(15 * time.Minute)}
	if req.SourceControlID != "" {
		var sc models.SourceControl
		if s.DB().WithContext(ctx).First(&sc, "id = ? AND team_id = ? AND provider = ?", req.SourceControlID, teamID, "github").Error != nil {
			return out, errAttempt
		}
		a.SourceControlID = &sc.ID
		a.ConnectionGeneration = sc.Generation
		a.SelectionVersion = sc.SelectionVersion
	}
	if err = s.DB().WithContext(ctx).Create(&a).Error; err != nil {
		return out, err
	}
	out = attemptResponse(&a)
	out.URL = target
	return out, nil
}
func attemptResponse(a *models.GitHubConnectionAttempt) GitHubAttemptResponse {
	return GitHubAttemptResponse{AttemptID: a.ID, Status: a.Status, TeamID: a.TeamID, ExpiresAt: a.ExpiresAt, SourceControlID: a.SourceControlID, ConnectionGeneration: a.ConnectionGeneration, SelectionVersion: a.SelectionVersion, Installations: []GitHubInstallationChoice{}}
}
func validateAttempt(ctx context.Context, db *gorm.DB, a *models.GitHubConnectionAttempt, teamID, userID, sessionID string) error {
	if sessionID == "" || a.UserID != userID || a.TeamID != teamID || a.SessionHash != digest(sessionID) || !a.ExpiresAt.After(time.Now()) {
		return errAttempt
	}
	if err := access.RequireManager(ctx, db, teamID, userID); err != nil {
		return errAttempt
	}
	hash, _, err := membership(ctx, db, teamID)
	if err != nil || hash != a.MembershipHash {
		return errAttempt
	}
	return nil
}
func (s *SourceControlService) loadAttempt(ctx context.Context, id, teamID, userID, sessionID string) (*models.GitHubConnectionAttempt, error) {
	var a models.GitHubConnectionAttempt
	if s.DB().WithContext(ctx).First(&a, "id = ?", id).Error != nil {
		return nil, errAttempt
	}
	if err := validateAttempt(ctx, s.DB(), &a, teamID, userID, sessionID); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *SourceControlService) CompleteGitHubCallback(ctx context.Context, teamID, userID, sessionID, state, code, installationID string) (GitHubAttemptResponse, error) {
	var result GitHubAttemptResponse
	if state == "" || sessionID == "" {
		return result, errAttempt
	}
	p, err := access.GitHubProvider(s.ProviderFactory())
	if err != nil {
		return result, err
	}
	err = s.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var a models.GitHubConnectionAttempt
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"})
		if code != "" {
			q = q.Where("state_hash = ?", digest(state))
		} else {
			q = q.Where("install_state_hash = ?", digest(state))
		}
		if q.First(&a).Error != nil {
			return errAttempt
		}
		if err := validateAttempt(ctx, tx, &a, teamID, userID, sessionID); err != nil {
			return err
		}
		if code == "" {
			if installationID == "" || a.Status != "authorized" || a.AuthorizationID == nil {
				return errAttempt
			}
			// The setup installation ID is only a hint. The authenticated user must see it.
			_, token, err := access.UserToken(ctx, tx, p, *a.AuthorizationID)
			if err != nil {
				return errAttempt
			}
			if _, err = s.verifiedInstallation(ctx, p, token, installationID); err != nil {
				return err
			}
			a.InstallStateHash = nil
		} else {
			if a.Status != "authorizing" || a.ConsumedAt != nil {
				return errAttempt
			}
			tokens, err := p.ExchangeUserCode(ctx, code, string(a.PKCEVerifier))
			if err != nil {
				return errAttempt
			}
			identity, err := p.GetAuthorizedUser(ctx, tokens.AccessToken)
			if err != nil {
				return errAttempt
			}
			remoteID, login := providers.ExtractFloatID(identity, "id"), str(identity, "login")
			if remoteID == "" || login == "" {
				return errAttempt
			}
			var authorization models.GitHubAuthorization
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&authorization, "host = ? AND app_id = ? AND user_id = ? AND git_hub_user_id = ?", "github.com", p.Config().AppID, userID, remoteID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			now := time.Now()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				authorization = models.GitHubAuthorization{Host: "github.com", AppID: p.Config().AppID, UserID: userID, GitHubUserID: remoteID, Generation: 1}
			} else if authorization.RevokedAt != nil {
				authorization.Generation++
			}
			authorization.Login = login
			authorization.AccessToken = dbtype.EncryptedString(tokens.AccessToken)
			authorization.RefreshToken = dbtype.EncryptedString(tokens.RefreshToken)
			authorization.ExpiresAt = now.Add(time.Duration(tokens.ExpiresIn) * time.Second)
			authorization.RefreshExpiresAt = now.Add(time.Duration(tokens.RefreshTokenExpiresIn) * time.Second)
			authorization.RevokedAt = nil
			if err = tx.Save(&authorization).Error; err != nil {
				return err
			}
			a.AuthorizationID = &authorization.ID
			a.Status = "authorized"
			a.ConsumedAt = &now
			a.PKCEVerifier = ""
		}
		if err := tx.Save(&a).Error; err != nil {
			return err
		}
		result = attemptResponse(&a)
		return nil
	})
	return result, err
}
func (s *SourceControlService) verifiedInstallation(ctx context.Context, p *providers.GitHubProvider, token, id string) (map[string]any, error) {
	installs, err := p.GetUserInstallations(ctx, token)
	if err != nil {
		return nil, errAttempt
	}
	for _, i := range installs {
		if providers.ExtractFloatID(i, "id") == id && providers.ExtractFloatID(i, "app_id") == p.Config().AppID && i["suspended_at"] == nil {
			return i, nil
		}
	}
	return nil, access.ErrAccessDenied
}
func (s *SourceControlService) GitHubAttempt(ctx context.Context, id, teamID, userID, sessionID string) (GitHubAttemptResponse, error) {
	a, err := s.loadAttempt(ctx, id, teamID, userID, sessionID)
	if err != nil {
		return GitHubAttemptResponse{}, err
	}
	out := attemptResponse(a)
	if a.Status != "authorized" || a.AuthorizationID == nil {
		return out, nil
	}
	p, err := access.GitHubProvider(s.ProviderFactory())
	if err != nil {
		return out, err
	}
	_, token, err := access.UserToken(ctx, s.DB(), p, *a.AuthorizationID)
	if err != nil {
		return out, errAttempt
	}
	installations, err := p.GetUserInstallations(ctx, token)
	if err != nil {
		return out, errAttempt
	}
	for _, i := range installations {
		if providers.ExtractFloatID(i, "app_id") != p.Config().AppID || i["suspended_at"] != nil {
			continue
		}
		account, _ := i["account"].(map[string]any)
		out.Installations = append(out.Installations, GitHubInstallationChoice{ID: providers.ExtractFloatID(i, "id"), AccountLogin: str(account, "login"), AccountType: str(account, "type"), AvatarURL: str(account, "avatar_url")})
	}
	state, err := secret()
	if err != nil {
		return out, err
	}
	hash := digest(state)
	if err = s.DB().WithContext(ctx).Model(a).Where("status = ?", "authorized").Update("install_state_hash", hash).Error; err != nil {
		return out, err
	}
	installURL, err := p.GetInstallationURL()
	if err != nil {
		return out, err
	}
	u, err := url.Parse(installURL)
	if err != nil {
		return out, err
	}
	q := u.Query()
	q.Set("state", state)
	u.RawQuery = q.Encode()
	out.InstallURL = u.String()
	return out, nil
}

// Resolve the local connection for the installation being reviewed without
// binding a fresh attempt: the user may still switch installations before saving.
func (s *SourceControlService) reviewGitHubConnection(ctx context.Context, attempt *models.GitHubConnectionAttempt, installationID string) (*models.SourceControl, error) {
	var connection models.SourceControl
	err := s.DB().WithContext(ctx).Where("team_id = ? AND provider = ? AND provider_id = ?", attempt.TeamID, gittypes.GitProviderGitHub, installationID).First(&connection).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && attempt.SourceControlID == nil {
		return nil, nil
	}
	if err != nil {
		return nil, errAttempt
	}
	if connection.GetInstallationID() != installationID || (attempt.SourceControlID != nil && connection.ID != *attempt.SourceControlID) {
		return nil, fiberutil.Conflict("Reconnect the same GitHub installation for this connection")
	}
	if attempt.SourceControlID != nil && (connection.Generation != attempt.ConnectionGeneration || connection.SelectionVersion != attempt.SelectionVersion) {
		return nil, access.ErrStaleAccess
	}
	return &connection, nil
}
func (s *SourceControlService) GitHubCandidates(ctx context.Context, id, teamID, userID, sessionID, installationID string) (GitHubCandidatesResponse, error) {
	out := GitHubCandidatesResponse{Repositories: []GitHubRepositoryChoice{}}
	a, err := s.loadAttempt(ctx, id, teamID, userID, sessionID)
	if err != nil {
		return out, err
	}
	if a.Status != "authorized" || a.AuthorizationID == nil {
		return out, errAttempt
	}
	connection, err := s.reviewGitHubConnection(ctx, a, installationID)
	if err != nil {
		return out, err
	}
	if connection != nil {
		out.SourceControlID = &connection.ID
		out.ConnectionGeneration = connection.Generation
		out.SelectionVersion = connection.SelectionVersion
	}
	p, err := access.GitHubProvider(s.ProviderFactory())
	if err != nil {
		return out, err
	}
	authorization, token, err := access.UserToken(ctx, s.DB(), p, *a.AuthorizationID)
	if err != nil {
		return out, errAttempt
	}
	if _, err = s.verifiedInstallation(ctx, p, token, installationID); err != nil {
		return out, err
	}
	repos, err := p.GetUserInstallationRepositories(ctx, token, installationID)
	if err != nil {
		return out, err
	}
	selected := map[string]bool{}
	saved := map[string]models.SourceControlRepository{}
	emitted := map[string]bool{}
	if out.SourceControlID != nil {
		var existing []models.SourceControlRepository
		if err = s.DB().WithContext(ctx).Where("source_control_id = ? AND selected = ?", *out.SourceControlID, true).Find(&existing).Error; err != nil {
			return out, err
		}
		for _, r := range existing {
			selected[r.GitHubRepositoryID] = true
			saved[r.GitHubRepositoryID] = r
		}
	}
	for _, r := range repos {
		full := str(r, "full_name")
		admin, err := p.UserRepositoryAdmin(ctx, token, full, authorization.Login)
		if err != nil {
			return out, err
		}
		if !admin {
			continue
		}
		remoteID := providers.ExtractFloatID(r, "id")
		var impact []GitHubWorkloadImpact
		if selected[remoteID] && out.SourceControlID != nil {
			impact, err = s.repositoryImpact(ctx, teamID, *out.SourceControlID, remoteID)
			if err != nil {
				return out, err
			}
		}
		emitted[remoteID] = true
		out.Repositories = append(out.Repositories, GitHubRepositoryChoice{AffectedWorkloads: impact, GitHubRepositoryID: remoteID, FullName: full, Name: str(r, "name"), DefaultBranch: str(r, "default_branch"), Private: boolean(r, "private"), Selected: selected[remoteID]})
	}
	for remoteID, repo := range saved {
		if emitted[remoteID] {
			continue
		}
		impact, err := s.repositoryImpact(ctx, teamID, *out.SourceControlID, remoteID)
		if err != nil {
			return out, err
		}
		out.Repositories = append(out.Repositories, GitHubRepositoryChoice{GitHubRepositoryID: remoteID, FullName: repo.FullName, Name: repo.Name, DefaultBranch: repo.DefaultBranch, Private: !repo.Public, Selected: true, Unavailable: true, AffectedWorkloads: impact})
	}
	sort.Slice(out.Repositories, func(i, j int) bool { return out.Repositories[i].FullName < out.Repositories[j].FullName })
	return out, nil
}
func (s *SourceControlService) SelectGitHubRepositories(ctx context.Context, id, teamID, userID, sessionID string, req GitHubSelectionRequest) (dto.SourceControlResponse, error) {
	var out dto.SourceControlResponse
	a, err := s.loadAttempt(ctx, id, teamID, userID, sessionID)
	if err != nil {
		return out, err
	}
	if a.Status != "authorized" || a.AuthorizationID == nil {
		return out, errAttempt
	}
	if len(req.RepositoryIDs) > 500 {
		return out, fiberutil.BadRequest("Select at most 500 repositories")
	}
	wanted := map[string]bool{}
	for _, id := range req.RepositoryIDs {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil || n <= 0 || wanted[id] {
			return out, fiberutil.BadRequest("Invalid repository selection")
		}
		wanted[id] = true
	}
	observedConnection, err := s.reviewGitHubConnection(ctx, a, req.InstallationID)
	if err != nil {
		return out, err
	}
	// Omitted IDs support older clients; explicit null means the review found
	// no connection, and must never overwrite one created in the meantime.
	if len(req.SourceControlID) > 0 {
		var reviewedID *string
		if err := json.Unmarshal(req.SourceControlID, &reviewedID); err != nil {
			return out, fiberutil.BadRequest("Invalid connection selection")
		}
		if (reviewedID == nil && observedConnection != nil) || (reviewedID != nil && (observedConnection == nil || *reviewedID != observedConnection.ID)) {
			return out, access.ErrStaleAccess
		}
	}
	if observedConnection != nil {
		if req.ExpectedGeneration == nil || *req.ExpectedGeneration != observedConnection.Generation || req.ExpectedSelectionVersion == nil || *req.ExpectedSelectionVersion != observedConnection.SelectionVersion {
			return out, access.ErrStaleAccess
		}
	} else if (req.ExpectedGeneration != nil && *req.ExpectedGeneration != 0) || (req.ExpectedSelectionVersion != nil && *req.ExpectedSelectionVersion != 0) {
		return out, access.ErrStaleAccess
	}
	p, err := access.GitHubProvider(s.ProviderFactory())
	if err != nil {
		return out, err
	}
	// Remember local revocation state before any upstream checks. A webhook
	// committed while those requests are in flight must win over their response.
	var observedInstallation models.GitHubInstallation
	if err = s.DB().WithContext(ctx).Where("host = ? AND app_id = ? AND installation_id = ?", "github.com", p.Config().AppID, req.InstallationID).First(&observedInstallation).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	observedGrants := map[uint64]uint64{}
	if observedConnection != nil {
		var grants []models.SourceControlRepository
		if err = s.DB().WithContext(ctx).Where("source_control_id = ?", observedConnection.ID).Find(&grants).Error; err != nil {
			return out, err
		}
		for _, grant := range grants {
			observedGrants[grant.ID] = grant.GrantVersion
		}
	}
	authorization, token, err := access.UserToken(ctx, s.DB(), p, *a.AuthorizationID)
	if err != nil {
		return out, errAttempt
	}
	if _, err = s.verifiedInstallation(ctx, p, token, req.InstallationID); err != nil {
		return out, err
	}
	inventory, err := p.GetUserInstallationRepositories(ctx, token, req.InstallationID)
	if err != nil {
		return out, err
	}
	chosen := map[string]map[string]any{}
	for _, repo := range inventory {
		remoteID := providers.ExtractFloatID(repo, "id")
		if !wanted[remoteID] {
			continue
		}
		full := str(repo, "full_name")
		if _, err = access.CanonicalRepository(full); err != nil {
			return out, err
		}
		admin, err := p.UserRepositoryAdmin(ctx, token, full, authorization.Login)
		if err != nil || !admin {
			return out, access.ErrAccessDenied
		}
		chosen[remoteID] = repo
	}
	if len(chosen) != len(wanted) {
		return out, access.ErrAccessDenied
	}
	installation, err := p.GetInstallation(ctx, req.InstallationID)
	if err != nil || installation.IsSuspended() || strconv.FormatInt(installation.AppID, 10) != p.Config().AppID {
		return out, access.ErrAccessDenied
	}
	var sc models.SourceControl
	err = s.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attempt models.GitHubConnectionAttempt
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "id = ?", a.ID).Error != nil || attempt.Status != "authorized" {
			return errAttempt
		}
		if err := validateAttempt(ctx, tx, &attempt, teamID, userID, sessionID); err != nil {
			return err
		}
		if attempt.AuthorizationID == nil || *attempt.AuthorizationID != authorization.ID || (attempt.SourceControlID == nil) != (a.SourceControlID == nil) {
			return errAttempt
		}
		if attempt.SourceControlID != nil && (*attempt.SourceControlID != *a.SourceControlID || attempt.ConnectionGeneration != a.ConnectionGeneration || attempt.SelectionVersion != a.SelectionVersion) {
			return errAttempt
		}
		var currentAuth models.GitHubAuthorization
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&currentAuth, "id = ?", authorization.ID).Error != nil || currentAuth.RevokedAt != nil || currentAuth.Generation != authorization.Generation {
			return errAttempt
		}
		var existingInstallation models.GitHubInstallation
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existingInstallation, "host = ? AND app_id = ? AND installation_id = ?", "github.com", p.Config().AppID, req.InstallationID).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if errors.Is(e, gorm.ErrRecordNotFound) {
			if observedInstallation.ID != "" {
				return access.ErrStaleAccess
			}
			// Separate accounts can discover the installation at the same time.
			// Reuse the winning metadata row, then check its revocation state.
			candidate := models.GitHubInstallation{Host: "github.com", AppID: p.Config().AppID, InstallationID: req.InstallationID, Generation: 1, RepositoryRevision: 1, Status: "active"}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "host"}, {Name: "app_id"}, {Name: "installation_id"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existingInstallation, "host = ? AND app_id = ? AND installation_id = ?", "github.com", p.Config().AppID, req.InstallationID).Error; err != nil {
				return err
			}
		}
		if observedInstallation.ID != "" {
			if existingInstallation.ID != observedInstallation.ID || existingInstallation.Generation != observedInstallation.Generation || existingInstallation.RepositoryRevision != observedInstallation.RepositoryRevision || existingInstallation.Status != observedInstallation.Status || existingInstallation.Permissions != observedInstallation.Permissions {
				return access.ErrStaleAccess
			}
		} else if existingInstallation.Generation != 1 || existingInstallation.RepositoryRevision != 1 || existingInstallation.Status != "active" {
			return access.ErrStaleAccess
		}
		if existingInstallation.Status != "" && existingInstallation.Status != "active" {
			existingInstallation.Generation++
		}
		now := time.Now()
		perms, _ := json.Marshal(installation.Permissions)
		existingInstallation.AccountID = installation.AccountID
		existingInstallation.AccountLogin = installation.AccountLogin
		existingInstallation.AccountType = installation.AccountType
		existingInstallation.Permissions = string(perms)
		existingInstallation.Status = "active"
		existingInstallation.LastSyncedAt = &now
		if err := tx.Save(&existingInstallation).Error; err != nil {
			return err
		}
		replacingAuthorization := false
		removedRepositories := []string{}
		if observedConnection != nil {
			if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sc, "id = ? AND team_id = ? AND provider = ? AND provider_id = ?", observedConnection.ID, teamID, gittypes.GitProviderGitHub, req.InstallationID).Error != nil {
				return errAttempt
			}
			if req.ExpectedGeneration == nil || *req.ExpectedGeneration != sc.Generation || sc.Generation != observedConnection.Generation || req.ExpectedSelectionVersion == nil || *req.ExpectedSelectionVersion != sc.SelectionVersion || sc.SelectionVersion != observedConnection.SelectionVersion {
				return access.ErrStaleAccess
			}
			if sc.GetInstallationID() != req.InstallationID {
				return fiberutil.Conflict("Reconnect the same GitHub installation for this connection")
			}
			replacingAuthorization = sc.Status != "active" || sc.GitHubAuthorizationID == nil || *sc.GitHubAuthorizationID != authorization.ID
			if replacingAuthorization {
				sc.Generation++
			}
			sc.SelectionVersion++
		} else {
			var count int64
			if err := tx.Model(&models.SourceControl{}).Where("team_id = ? AND provider = ? AND provider_id = ?", teamID, "github", req.InstallationID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return access.ErrStaleAccess
			}
			sc = models.SourceControl{Provider: gittypes.GitProviderGitHub, ProviderID: req.InstallationID, Generation: 1, SelectionVersion: 1}
			sc.TeamID = teamID
			sc.UserID = userID
		}
		count := len(chosen)
		selection := "selected"
		permissions := string(perms)
		sc.Status = "active"
		sc.GitHubAuthorizationID = &authorization.ID
		sc.GitHubInstallationID = &existingInstallation.ID
		sc.InstallationID = &req.InstallationID
		sc.Login = &installation.AccountLogin
		sc.Name = &installation.AccountLogin
		sc.Type = &installation.AccountType
		sc.ProviderAccountID = &installation.AccountID
		sc.AvatarURL = &installation.AccountAvatarURL
		sc.HTMLURL = &installation.HTMLURL
		sc.Permissions = &permissions
		sc.RepositorySelection = &selection
		sc.HasMultipleRepositories = true
		sc.RepositoryCount = &count
		sc.LastSyncedAt = &now
		if sc.ConnectedAt == nil {
			sc.ConnectedAt = &now
		}
		if err := tx.Save(&sc).Error; err != nil {
			return err
		}
		var existing []models.SourceControlRepository
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_control_id = ?", sc.ID).Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) != len(observedGrants) {
			return access.ErrStaleAccess
		}
		for _, grant := range existing {
			if version, ok := observedGrants[grant.ID]; !ok || version != grant.GrantVersion {
				return access.ErrStaleAccess
			}
		}
		byRemote := map[string]*models.SourceControlRepository{}
		byName := map[string]*models.SourceControlRepository{}
		for i := range existing {
			r := &existing[i]
			byRemote[r.GitHubRepositoryID] = r
			byName[r.FullName] = r
		}
		for _, r := range existing {
			if r.Selected && !wanted[r.GitHubRepositoryID] {
				removedRepositories = append(removedRepositories, strconv.FormatUint(r.ID, 10))
				if err := tx.Model(&models.SourceControlRepository{}).Where("id = ?", r.ID).Updates(map[string]any{"selected": false, "grant_version": gorm.Expr("grant_version + 1"), "verified_at": nil}).Error; err != nil {
					return err
				}
			}
		}
		ids := make([]string, 0, len(chosen))
		for remoteID := range chosen {
			ids = append(ids, remoteID)
		}
		sort.Strings(ids)
		for _, remoteID := range ids {
			data := chosen[remoteID]
			r := byRemote[remoteID]
			if r == nil {
				r = byName[str(data, "full_name")]
				if r != nil && r.GitHubRepositoryID != "" && r.GitHubRepositoryID != remoteID {
					return fiberutil.Conflict("Repository identity changed; contact support to preserve its history")
				}
			}
			if r == nil {
				r = &models.SourceControlRepository{SourceControlID: sc.ID, GrantVersion: 1}
			} else if replacingAuthorization || !r.Selected || !r.Available || r.GitHubRepositoryID != remoteID || r.FullName != str(data, "full_name") {
				r.GrantVersion++
			}
			html := "https://github.com/" + str(data, "full_name")
			r.GitHubRepositoryID = remoteID
			r.Name = str(data, "name")
			r.FullName = str(data, "full_name")
			r.Public = !boolean(data, "private")
			r.DefaultBranch = str(data, "default_branch")
			r.HTMLURL = &html
			r.SSHURL = "git@github.com:" + r.FullName + ".git"
			r.Selected = true
			r.Available = true
			r.VerifiedAt = &now
			if err := tx.Save(r).Error; err != nil {
				return err
			}
		}
		if replacingAuthorization {
			if err := access.ActivateCleanup(ctx, tx, sc.ID, ""); err != nil {
				return err
			}
			if err := tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ? AND status IN ?", sc.ID, []string{"issuing", "released"}).Update("status", "revoke_pending").Error; err != nil {
				return err
			}
		} else {
			for _, repositoryID := range removedRepositories {
				if err := access.ActivateCleanup(ctx, tx, sc.ID, repositoryID); err != nil {
					return err
				}
				if err := tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ? AND repository_id = ? AND status IN ?", sc.ID, repositoryID, []string{"issuing", "released"}).Update("status", "revoke_pending").Error; err != nil {
					return err
				}
			}
		}
		attempt.Status = "completed"
		attempt.SourceControlID = &sc.ID
		attempt.ConnectionGeneration = sc.Generation
		attempt.SelectionVersion = sc.SelectionVersion
		attempt.InstallStateHash = nil
		return tx.Save(&attempt).Error
	})
	if err != nil {
		return out, err
	}
	_ = access.NewResolver(s.DB(), s.ProviderFactory()).RevokeConnectionPending(ctx, sc.ID)
	if err = s.DB().WithContext(ctx).Where("source_control_id = ? AND selected = ? AND available = ?", sc.ID, true, true).Find(&sc.Repositories).Error; err != nil {
		return out, err
	}
	s.decorateCleanup(ctx, &sc)
	return dto.ToSourceControlResponse(&sc), nil
}
