package services

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// =============================================================================
// Mock implementations
// =============================================================================

type mockRepoRegistry struct {
	contracts.RepositoryRegistry
	user           *mockUserRepo
	team           *mockTeamRepo
	teamMember     *mockTeamMemberRepo
	teamInvitation *mockTeamInvitationRepo
}

func (m *mockRepoRegistry) User() contracts.UserRepository             { return m.user }
func (m *mockRepoRegistry) Team() contracts.TeamRepository             { return m.team }
func (m *mockRepoRegistry) TeamMember() contracts.TeamMemberRepository { return m.teamMember }
func (m *mockRepoRegistry) TeamInvitation() contracts.TeamInvitationRepository {
	return m.teamInvitation
}

type mockUserRepo struct {
	contracts.UserRepository
	users map[string]*models.User
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

func (m *mockUserRepo) SetCurrentTeam(_ context.Context, _, _ string) error {
	return nil
}

type mockTeamRepo struct {
	contracts.TeamRepository
	teams   map[string]*models.Team
	members map[string][]models.TeamMember
}

func (m *mockTeamRepo) FindByID(_ context.Context, id string) (*models.Team, error) {
	if t, ok := m.teams[id]; ok {
		return t, nil
	}
	return nil, nil
}

func (m *mockTeamRepo) GetMembers(_ context.Context, teamID string) ([]models.TeamMember, error) {
	return m.members[teamID], nil
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

type mockTeamMemberRepo struct {
	contracts.TeamMemberRepository
	members  map[string]map[string]*models.TeamMember // teamID -> userID -> member
	addedErr error
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
		if _, ok := team[userID]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTeamMemberRepo) AddUser(_ context.Context, teamID, userID, role string) error {
	if m.addedErr != nil {
		return m.addedErr
	}
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
	return errors.New("member not found")
}

type mockTeamInvitationRepo struct {
	contracts.TeamInvitationRepository
	invitations map[string]*models.TeamInvitation
	createErr   error
	created     *models.TeamInvitation
}

func (m *mockTeamInvitationRepo) Create(_ context.Context, invitation *models.TeamInvitation) error {
	if m.createErr != nil {
		return m.createErr
	}
	if invitation.ID == "" {
		invitation.ID = "inv_test_001"
	}
	m.created = invitation
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

// =============================================================================
// Helper functions
// =============================================================================

func strPtr(s string) *string { return &s }

func newTestRegistry() (*mockRepoRegistry, *TeamMemberService) {
	reg := &mockRepoRegistry{
		user: &mockUserRepo{
			users: map[string]*models.User{
				"owner_001": {Name: "Owner", Email: "owner@example.com"},
				"admin_001": {Name: "Admin", Email: "admin@example.com"},
				"user_001":  {Name: "User", Email: "user@example.com"},
				"user_002":  {Name: "User Two", Email: "user2@example.com"},
			},
		},
		team: &mockTeamRepo{
			teams: map[string]*models.Team{
				"team_001": {UserID: "owner_001", Name: "Test Team"},
			},
			members: map[string][]models.TeamMember{},
		},
		teamMember: &mockTeamMemberRepo{
			members: map[string]map[string]*models.TeamMember{
				"team_001": {
					"admin_001": {TeamID: "team_001", UserID: "admin_001", Role: strPtr("admin")},
				},
			},
		},
		teamInvitation: &mockTeamInvitationRepo{
			invitations: map[string]*models.TeamInvitation{},
		},
	}

	// Set IDs on user models
	reg.user.users["owner_001"].ID = "owner_001"
	reg.user.users["admin_001"].ID = "admin_001"
	reg.user.users["user_001"].ID = "user_001"
	reg.user.users["user_002"].ID = "user_002"

	// Set ID on team model
	reg.team.teams["team_001"].ID = "team_001"

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	svc := &TeamMemberService{repos: reg, logger: &logger}
	return reg, svc
}

// =============================================================================
// Tests: InviteTeamMember
// =============================================================================

func TestInviteTeamMember_OwnerCanInvite(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.InviteTeamMemberRequest{
		Email: "newuser@example.com",
		Role:  "editor",
	}

	err := svc.InviteTeamMember(ctx, "owner_001", "team_001", req)
	require.NoError(t, err)

	assert.NotNil(t, reg.teamInvitation.created)
	assert.Equal(t, "newuser@example.com", reg.teamInvitation.created.Email)
	assert.Equal(t, "team_001", reg.teamInvitation.created.TeamID)
	assert.Equal(t, "editor", *reg.teamInvitation.created.Role)
}

func TestInviteTeamMember_AdminCanInvite(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.InviteTeamMemberRequest{
		Email: "newuser@example.com",
		Role:  "member",
	}

	err := svc.InviteTeamMember(ctx, "admin_001", "team_001", req)
	require.NoError(t, err)

	assert.NotNil(t, reg.teamInvitation.created)
	assert.Equal(t, "newuser@example.com", reg.teamInvitation.created.Email)
}

func TestInviteTeamMember_RegularMemberCannotInvite(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.InviteTeamMemberRequest{
		Email: "newuser@example.com",
		Role:  "member",
	}

	err := svc.InviteTeamMember(ctx, "user_001", "team_001", req)
	assert.Error(t, err)
}

func TestInviteTeamMember_TeamNotFound(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.InviteTeamMemberRequest{
		Email: "newuser@example.com",
		Role:  "member",
	}

	err := svc.InviteTeamMember(ctx, "owner_001", "nonexistent", req)
	assert.Error(t, err)
}

func TestInviteTeamMember_ExistingMemberCannotBeInvited(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	// admin_001 is already a member of team_001
	req := &dto.InviteTeamMemberRequest{
		Email: "admin@example.com",
		Role:  "editor",
	}

	err := svc.InviteTeamMember(ctx, "owner_001", "team_001", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already a team member")
}

func TestInviteTeamMember_DuplicateInvitationRejected(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	// Create an existing invitation
	role := "member"
	reg.teamInvitation.invitations["existing_inv"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "pending@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["existing_inv"].ID = "existing_inv"

	req := &dto.InviteTeamMemberRequest{
		Email: "pending@example.com",
		Role:  "member",
	}

	err := svc.InviteTeamMember(ctx, "owner_001", "team_001", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitation already sent")
}

func TestInviteTeamMember_EmailIsNormalized(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.InviteTeamMemberRequest{
		Email: "  NewUser@Example.COM  ",
		Role:  "member",
	}

	err := svc.InviteTeamMember(ctx, "owner_001", "team_001", req)
	require.NoError(t, err)

	assert.Equal(t, "newuser@example.com", reg.teamInvitation.created.Email)
}

// =============================================================================
// Tests: AcceptTeamInvitation
// =============================================================================

func TestAcceptTeamInvitation_Success(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	// Create an invitation for user_001
	role := "editor"
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "user@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	err := svc.AcceptTeamInvitation(ctx, "user_001", "inv_001")
	require.NoError(t, err)

	// User should be added to team
	member, ok := reg.teamMember.members["team_001"]["user_001"]
	require.True(t, ok)
	assert.Equal(t, "editor", *member.Role)

	// Invitation should be deleted
	_, exists := reg.teamInvitation.invitations["inv_001"]
	assert.False(t, exists)
}

func TestAcceptTeamInvitation_InvitationNotFound(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	err := svc.AcceptTeamInvitation(ctx, "user_001", "nonexistent")
	assert.Error(t, err)
}

func TestAcceptTeamInvitation_WrongEmail(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	// Create an invitation for a different email
	role := "member"
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "other@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	// user_001 has email "user@example.com", not "other@example.com"
	err := svc.AcceptTeamInvitation(ctx, "user_001", "inv_001")
	assert.Error(t, err)
}

func TestAcceptTeamInvitation_DefaultsToMemberRole(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	// Create an invitation without a role
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "user@example.com",
		Role:   nil,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	err := svc.AcceptTeamInvitation(ctx, "user_001", "inv_001")
	require.NoError(t, err)

	member, ok := reg.teamMember.members["team_001"]["user_001"]
	require.True(t, ok)
	assert.Equal(t, "member", *member.Role)
}

// =============================================================================
// Tests: CancelTeamInvitation
// =============================================================================

func TestCancelTeamInvitation_OwnerCanCancel(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	role := "member"
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "pending@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	err := svc.CancelTeamInvitation(ctx, "owner_001", "team_001", "inv_001")
	require.NoError(t, err)

	_, exists := reg.teamInvitation.invitations["inv_001"]
	assert.False(t, exists)
}

func TestCancelTeamInvitation_AdminCanCancel(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	role := "member"
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "pending@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	err := svc.CancelTeamInvitation(ctx, "admin_001", "team_001", "inv_001")
	require.NoError(t, err)

	_, exists := reg.teamInvitation.invitations["inv_001"]
	assert.False(t, exists)
}

func TestCancelTeamInvitation_RegularMemberCannotCancel(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	role := "member"
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "team_001",
		Email:  "pending@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	err := svc.CancelTeamInvitation(ctx, "user_001", "team_001", "inv_001")
	assert.Error(t, err)
}

func TestCancelTeamInvitation_InvitationFromDifferentTeam(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	role := "member"
	reg.teamInvitation.invitations["inv_001"] = &models.TeamInvitation{
		TeamID: "other_team",
		Email:  "pending@example.com",
		Role:   &role,
	}
	reg.teamInvitation.invitations["inv_001"].ID = "inv_001"

	err := svc.CancelTeamInvitation(ctx, "owner_001", "team_001", "inv_001")
	assert.Error(t, err)
}

// =============================================================================
// Tests: RemoveTeamMember
// =============================================================================

func TestRemoveTeamMember_OwnerCanRemove(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	reg.teamMember.members["team_001"]["user_001"] = &models.TeamMember{
		TeamID: "team_001",
		UserID: "user_001",
		Role:   strPtr("member"),
	}

	err := svc.RemoveTeamMember(ctx, "owner_001", "team_001", "user_001")
	require.NoError(t, err)

	_, exists := reg.teamMember.members["team_001"]["user_001"]
	assert.False(t, exists)
}

func TestRemoveTeamMember_MemberCanRemoveSelf(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	reg.teamMember.members["team_001"]["user_001"] = &models.TeamMember{
		TeamID: "team_001",
		UserID: "user_001",
		Role:   strPtr("member"),
	}

	err := svc.RemoveTeamMember(ctx, "user_001", "team_001", "user_001")
	require.NoError(t, err)

	_, exists := reg.teamMember.members["team_001"]["user_001"]
	assert.False(t, exists)
}

func TestRemoveTeamMember_CannotRemoveOwner(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	err := svc.RemoveTeamMember(ctx, "owner_001", "team_001", "owner_001")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove team owner")
}

func TestRemoveTeamMember_MemberCannotRemoveOthers(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	reg.teamMember.members["team_001"]["user_001"] = &models.TeamMember{
		TeamID: "team_001",
		UserID: "user_001",
		Role:   strPtr("member"),
	}

	err := svc.RemoveTeamMember(ctx, "user_002", "team_001", "user_001")
	assert.Error(t, err)
}

// =============================================================================
// Tests: UpdateTeamMemberRole
// =============================================================================

func TestUpdateTeamMemberRole_OwnerCanUpdate(t *testing.T) {
	reg, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.UpdateTeamMemberRequest{Role: "editor"}

	err := svc.UpdateTeamMemberRole(ctx, "owner_001", "team_001", "admin_001", req)
	require.NoError(t, err)

	member := reg.teamMember.members["team_001"]["admin_001"]
	assert.Equal(t, "editor", *member.Role)
}

func TestUpdateTeamMemberRole_NonOwnerCannotUpdate(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.UpdateTeamMemberRequest{Role: "member"}

	err := svc.UpdateTeamMemberRole(ctx, "admin_001", "team_001", "user_001", req)
	assert.Error(t, err)
}

func TestUpdateTeamMemberRole_CannotUpdateOwnerRole(t *testing.T) {
	_, svc := newTestRegistry()
	ctx := context.Background()

	req := &dto.UpdateTeamMemberRequest{Role: "admin"}

	err := svc.UpdateTeamMemberRole(ctx, "owner_001", "team_001", "owner_001", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update owner's role")
}
