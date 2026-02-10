package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ============================================================================
// Mock repositories
// ============================================================================

type mockUserRepo struct {
	contracts.UserRepository
	users map[string]*models.User
}

func (m *mockUserRepo) Create(_ context.Context, user *models.User) error {
	if user.ID == "" {
		user.ID = "generated_user_id"
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) Update(_ context.Context, user *models.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Delete(_ context.Context, id string) error {
	delete(m.users, id)
	return nil
}

func (m *mockUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	for _, u := range m.users {
		if u.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockUserRepo) SetCurrentTeam(_ context.Context, userID, teamID string) error {
	if u, ok := m.users[userID]; ok {
		u.CurrentTeamID = &teamID
	}
	return nil
}

func (m *mockUserRepo) MarkEmailAsVerified(_ context.Context, userID string) error {
	if u, ok := m.users[userID]; ok {
		now := time.Now()
		u.EmailVerifiedAt = &now
	}
	return nil
}

func (m *mockUserRepo) SetOnboarded(_ context.Context, userID string, onboarded bool) error {
	if u, ok := m.users[userID]; ok {
		u.Onboarded = onboarded
	}
	return nil
}

type mockTeamRepo struct {
	contracts.TeamRepository
	teams   map[string]*models.Team
	members map[string][]models.TeamMember
}

func (m *mockTeamRepo) Create(_ context.Context, team *models.Team) error {
	if team.ID == "" {
		team.ID = "generated_team_id"
	}
	m.teams[team.ID] = team
	return nil
}

func (m *mockTeamRepo) FindByID(_ context.Context, id string) (*models.Team, error) {
	if t, ok := m.teams[id]; ok {
		return t, nil
	}
	return nil, nil
}

func (m *mockTeamRepo) Update(_ context.Context, team *models.Team) error {
	m.teams[team.ID] = team
	return nil
}

func (m *mockTeamRepo) Delete(_ context.Context, id string) error {
	delete(m.teams, id)
	return nil
}

func (m *mockTeamRepo) GetUserTeams(_ context.Context, userID string) ([]models.Team, error) {
	var result []models.Team
	for _, t := range m.teams {
		if t.UserID == userID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockTeamRepo) GetMembers(_ context.Context, teamID string) ([]models.TeamMember, error) {
	return m.members[teamID], nil
}

type mockTeamMemberRepo struct {
	contracts.TeamMemberRepository
	members map[string]map[string]*models.TeamMember
}

func (m *mockTeamMemberRepo) AddUser(_ context.Context, teamID, userID, role string) error {
	if m.members == nil {
		m.members = make(map[string]map[string]*models.TeamMember)
	}
	if m.members[teamID] == nil {
		m.members[teamID] = make(map[string]*models.TeamMember)
	}
	m.members[teamID][userID] = &models.TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   &role,
	}
	return nil
}

func (m *mockTeamMemberRepo) RemoveUser(_ context.Context, teamID, userID string) error {
	if team, ok := m.members[teamID]; ok {
		delete(team, userID)
	}
	return nil
}

func (m *mockTeamMemberRepo) UpdateRole(_ context.Context, teamID, userID, role string) error {
	if team, ok := m.members[teamID]; ok {
		if member, ok := team[userID]; ok {
			member.Role = &role
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (m *mockTeamMemberRepo) Get(_ context.Context, teamID, userID string) (*models.TeamMember, error) {
	if team, ok := m.members[teamID]; ok {
		if member, ok := team[userID]; ok {
			return member, nil
		}
	}
	return nil, nil
}

func (m *mockTeamMemberRepo) IsMember(_ context.Context, teamID, userID string) (bool, error) {
	if team, ok := m.members[teamID]; ok {
		_, ok := team[userID]
		return ok, nil
	}
	return false, nil
}

type mockTeamInvitationRepo struct {
	contracts.TeamInvitationRepository
	invitations map[string]*models.TeamInvitation
}

func (m *mockTeamInvitationRepo) Create(_ context.Context, invitation *models.TeamInvitation) error {
	if invitation.ID == "" {
		invitation.ID = "generated_invitation_id"
	}
	if m.invitations == nil {
		m.invitations = make(map[string]*models.TeamInvitation)
	}
	m.invitations[invitation.ID] = invitation
	return nil
}

func (m *mockTeamInvitationRepo) FindByID(_ context.Context, id string) (*models.TeamInvitation, error) {
	if inv, ok := m.invitations[id]; ok {
		return inv, nil
	}
	return nil, nil
}

func (m *mockTeamInvitationRepo) FindByEmail(_ context.Context, teamID, email string) (*models.TeamInvitation, error) {
	for _, inv := range m.invitations {
		if inv.TeamID == teamID && inv.Email == email {
			return inv, nil
		}
	}
	return nil, nil
}

func (m *mockTeamInvitationRepo) GetByTeam(_ context.Context, teamID string) ([]models.TeamInvitation, error) {
	var result []models.TeamInvitation
	for _, inv := range m.invitations {
		if inv.TeamID == teamID {
			result = append(result, *inv)
		}
	}
	return result, nil
}

func (m *mockTeamInvitationRepo) Delete(_ context.Context, id string) error {
	delete(m.invitations, id)
	return nil
}

type mockPasswordResetTokenRepo struct {
	contracts.PasswordResetTokenRepository
	tokens map[string]*models.PasswordResetToken
}

func (m *mockPasswordResetTokenRepo) Create(_ context.Context, token *models.PasswordResetToken) error {
	if m.tokens == nil {
		m.tokens = make(map[string]*models.PasswordResetToken)
	}
	m.tokens[token.Email] = token
	return nil
}

func (m *mockPasswordResetTokenRepo) FindByEmail(_ context.Context, email string) (*models.PasswordResetToken, error) {
	if t, ok := m.tokens[email]; ok {
		return t, nil
	}
	return nil, nil
}

func (m *mockPasswordResetTokenRepo) Delete(_ context.Context, email string) error {
	delete(m.tokens, email)
	return nil
}

type mockPATRepo struct {
	contracts.PersonalAccessTokenRepository
	tokens map[string]*models.PersonalAccessToken
}

func (m *mockPATRepo) Create(_ context.Context, token *models.PersonalAccessToken) error {
	if m.tokens == nil {
		m.tokens = make(map[string]*models.PersonalAccessToken)
	}
	m.tokens[token.ID] = token
	return nil
}

func (m *mockPATRepo) FindByToken(_ context.Context, token string) (*models.PersonalAccessToken, error) {
	for _, t := range m.tokens {
		if t.Token == token {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockPATRepo) UpdateLastUsed(_ context.Context, _ string) error { return nil }

func (m *mockPATRepo) Delete(_ context.Context, id string) error {
	delete(m.tokens, id)
	return nil
}

func (m *mockPATRepo) DeleteByUser(_ context.Context, id, userID string) (int64, error) {
	if t, ok := m.tokens[id]; ok && t.TokenableID == userID {
		delete(m.tokens, id)
		return 1, nil
	}
	return 0, nil
}

func (m *mockPATRepo) GetByUser(_ context.Context, userID string) ([]models.PersonalAccessToken, error) {
	var result []models.PersonalAccessToken
	for _, t := range m.tokens {
		if t.TokenableID == userID {
			result = append(result, *t)
		}
	}
	return result, nil
}

type mockPasskeyRepo struct {
	contracts.PasskeyRepository
	passkeys map[string]*models.Passkey
}

func (m *mockPasskeyRepo) Create(_ context.Context, passkey *models.Passkey) error {
	if m.passkeys == nil {
		m.passkeys = make(map[string]*models.Passkey)
	}
	m.passkeys[passkey.ID] = passkey
	return nil
}

func (m *mockPasskeyRepo) FindByID(_ context.Context, id string) (*models.Passkey, error) {
	if p, ok := m.passkeys[id]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockPasskeyRepo) FindByCredentialID(_ context.Context, credentialID string) (*models.Passkey, error) {
	for _, p := range m.passkeys {
		if p.CredentialID == credentialID {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockPasskeyRepo) FindByUserID(_ context.Context, userID string) ([]models.Passkey, error) {
	var result []models.Passkey
	for _, p := range m.passkeys {
		if p.UserID == userID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockPasskeyRepo) Update(_ context.Context, passkey *models.Passkey) error {
	m.passkeys[passkey.ID] = passkey
	return nil
}

func (m *mockPasskeyRepo) Delete(_ context.Context, id string) error {
	delete(m.passkeys, id)
	return nil
}

func (m *mockPasskeyRepo) DeleteByUserID(_ context.Context, id, userID string) error {
	if p, ok := m.passkeys[id]; ok && p.UserID == userID {
		delete(m.passkeys, id)
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (m *mockPasskeyRepo) CountByUserID(_ context.Context, userID string) (int64, error) {
	var count int64
	for _, p := range m.passkeys {
		if p.UserID == userID {
			count++
		}
	}
	return count, nil
}

type mockSessionRepo struct {
	contracts.SessionRepository
	sessions map[string]*models.Session
}

func (m *mockSessionRepo) Create(_ context.Context, session *models.Session) error {
	if session.ID == "" {
		session.ID = "generated_session_id"
	}
	if m.sessions == nil {
		m.sessions = make(map[string]*models.Session)
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionRepo) FindByID(_ context.Context, id string) (*models.Session, error) {
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockSessionRepo) GetByUser(_ context.Context, userID string) ([]models.Session, error) {
	var result []models.Session
	for _, s := range m.sessions {
		if s.UserID != nil && *s.UserID == userID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockSessionRepo) UpdateLastActivity(_ context.Context, _ string) error { return nil }

func (m *mockSessionRepo) Delete(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionRepo) DeleteByUser(_ context.Context, id, userID string) error {
	if s, ok := m.sessions[id]; ok && s.UserID != nil && *s.UserID == userID {
		delete(m.sessions, id)
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (m *mockSessionRepo) DeleteAllByUserExcept(_ context.Context, userID, exceptID string) (int64, error) {
	var count int64
	for id, s := range m.sessions {
		if s.UserID != nil && *s.UserID == userID && id != exceptID {
			delete(m.sessions, id)
			count++
		}
	}
	return count, nil
}

func (m *mockSessionRepo) DeleteAllByUser(_ context.Context, userID string) error {
	for id, s := range m.sessions {
		if s.UserID != nil && *s.UserID == userID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *mockSessionRepo) Exists(_ context.Context, id string) (bool, error) {
	_, ok := m.sessions[id]
	return ok, nil
}

// ============================================================================
// Mock repository registry
// ============================================================================

type mockRepoRegistry struct {
	user           *mockUserRepo
	team           *mockTeamRepo
	teamMember     *mockTeamMemberRepo
	teamInvitation *mockTeamInvitationRepo
	passwordReset  *mockPasswordResetTokenRepo
	pat            *mockPATRepo
	passkey        *mockPasskeyRepo
	session        *mockSessionRepo
}

func (m *mockRepoRegistry) User() contracts.UserRepository             { return m.user }
func (m *mockRepoRegistry) Team() contracts.TeamRepository             { return m.team }
func (m *mockRepoRegistry) TeamMember() contracts.TeamMemberRepository { return m.teamMember }
func (m *mockRepoRegistry) TeamInvitation() contracts.TeamInvitationRepository {
	return m.teamInvitation
}
func (m *mockRepoRegistry) PasswordResetToken() contracts.PasswordResetTokenRepository {
	return m.passwordReset
}
func (m *mockRepoRegistry) PersonalAccessToken() contracts.PersonalAccessTokenRepository {
	return m.pat
}
func (m *mockRepoRegistry) Passkey() contracts.PasskeyRepository              { return m.passkey }
func (m *mockRepoRegistry) Session() contracts.SessionRepository              { return m.session }
func (m *mockRepoRegistry) DB() *gorm.DB                                      { return nil }
func (m *mockRepoRegistry) IsTeamSubscribed(_ context.Context, _ string) bool { return false }
func (m *mockRepoRegistry) IsUserAdmin(_ context.Context, _ string) bool      { return false }

// ============================================================================
// Test helpers
// ============================================================================

func newMockRegistry() *mockRepoRegistry {
	return &mockRepoRegistry{
		user:           &mockUserRepo{users: make(map[string]*models.User)},
		team:           &mockTeamRepo{teams: make(map[string]*models.Team), members: make(map[string][]models.TeamMember)},
		teamMember:     &mockTeamMemberRepo{members: make(map[string]map[string]*models.TeamMember)},
		teamInvitation: &mockTeamInvitationRepo{invitations: make(map[string]*models.TeamInvitation)},
		passwordReset:  &mockPasswordResetTokenRepo{tokens: make(map[string]*models.PasswordResetToken)},
		pat:            &mockPATRepo{tokens: make(map[string]*models.PersonalAccessToken)},
		passkey:        &mockPasskeyRepo{passkeys: make(map[string]*models.Passkey)},
		session:        &mockSessionRepo{sessions: make(map[string]*models.Session)},
	}
}

func newTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		},
	})
}

func makeJSONRequest(method, path string, body any) *http.Request {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

type testResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Errors  json.RawMessage `json:"errors"`
}

func parseResponse(resp *http.Response) testResponse {
	var r testResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &r)
	return r
}

func newTestUser(id, name, email, password string) *models.User {
	now := time.Now()
	u := &models.User{
		Name:      name,
		Email:     email,
		Password:  password,
		Onboarded: true,
	}
	u.ID = id
	u.CreatedAt = &now
	u.UpdatedAt = &now
	return u
}

// ============================================================================
// Shared test infrastructure
// ============================================================================

// mockCache implements cache.Cache for testing
type mockCache struct{}

func (m *mockCache) Get(_ context.Context, _ string) (string, error) {
	return "", errors.New("not found")
}

func (m *mockCache) Set(_ context.Context, _, _ string, _ time.Duration) error { return nil }
func (m *mockCache) Delete(_ context.Context, _ string) error                  { return nil }
func (m *mockCache) Exists(_ context.Context, _ string) (bool, error)          { return false, nil }

var _ cache.Cache = (*mockCache)(nil)

func testConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-jwt-secret-at-least-32-chars-long",
			Expiration: 72,
		},
		App: config.AppConfig{
			Name: "TestApp",
			URL:  "http://localhost:8080",
		},
		Passkey: config.PasskeyConfig{
			RPName:     "TestApp",
			RPID:       "localhost",
			RPOrigin:   "http://localhost:8080",
			Timeout:    60000,
			MaxPerUser: 10,
		},
	}
}

func newTestAppWithValidation() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: fiberutil.NewErrorHandler(),
	})
}
