package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	testrequire "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts/mocks"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// AcceptInviteSuite covers AuthService.AcceptTeamInvitation's branching
// (#71): new vs existing invitee, and the guards that fire before any DB
// work (not-found, name-required, wrong-password, suspended).
type AcceptInviteSuite struct {
	suite.Suite

	registry *mocks.RepositoryRegistry
	userRepo *mocks.UserRepository
	invRepo  *mocks.TeamInvitationRepository
	svc      *AuthService
}

func (s *AcceptInviteSuite) SetupTest() {
	s.registry = mocks.NewRepositoryRegistry(s.T())
	s.userRepo = mocks.NewUserRepository(s.T())
	s.invRepo = mocks.NewTeamInvitationRepository(s.T())
	s.registry.EXPECT().User().Return(s.userRepo).Maybe()
	s.registry.EXPECT().TeamInvitation().Return(s.invRepo).Maybe()

	logger := zerolog.Nop()
	s.svc = NewAuthService(s.registry, &config.Config{}, &logger, nil)
}

func TestAcceptInviteSuite(t *testing.T) {
	suite.Run(t, new(AcceptInviteSuite))
}

func (s *AcceptInviteSuite) TestInvitationNotFound() {
	s.invRepo.EXPECT().FindByID(mock.Anything, "tok").Return(nil, nil).Once()

	res, err := s.svc.AcceptTeamInvitation(context.Background(), "tok", "Jane", "password123", "", "")
	s.Require().Error(err)
	s.Nil(res)
	s.True(fiberutil.IsNotFound(err))
}

func (s *AcceptInviteSuite) TestNewInviteeRequiresName() {
	inv := &models.TeamInvitation{TeamID: "team_1", Email: "new@example.com"}
	s.invRepo.EXPECT().FindByID(mock.Anything, "tok").Return(inv, nil).Once()
	// No existing account → registration branch, which requires a name.
	s.userRepo.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(nil, nil).Once()

	res, err := s.svc.AcceptTeamInvitation(context.Background(), "tok", "   ", "password123", "", "")
	s.Require().Error(err)
	s.Nil(res)
	s.True(fiberutil.IsValidationError(err), "empty name on a new-account accept must be a validation error")
}

func (s *AcceptInviteSuite) TestExistingUserWrongPasswordRejected() {
	hash, err := security.HashPassword("correct-horse")
	s.Require().NoError(err)

	inv := &models.TeamInvitation{TeamID: "team_1", Email: "existing@example.com"}
	user := &models.User{
		BaseModel: basemodels.BaseModel{ID: "user_1"},
		Email:     "existing@example.com",
		Password:  hash,
		Status:    authtypes.UserStatusActive,
	}
	s.invRepo.EXPECT().FindByID(mock.Anything, "tok").Return(inv, nil).Once()
	s.userRepo.EXPECT().FindByEmail(mock.Anything, "existing@example.com").Return(user, nil).Once()

	res, err := s.svc.AcceptTeamInvitation(context.Background(), "tok", "", "wrong-password", "", "")
	s.Require().Error(err)
	s.Nil(res)
	s.True(fiberutil.IsUnauthorized(err), "wrong password must be rejected before any membership write")
}

func (s *AcceptInviteSuite) TestExistingSuspendedUserForbidden() {
	inv := &models.TeamInvitation{TeamID: "team_1", Email: "susp@example.com"}
	user := &models.User{
		BaseModel: basemodels.BaseModel{ID: "user_2"},
		Email:     "susp@example.com",
		Password:  "whatever",
		Status:    authtypes.UserStatusSuspended,
	}
	s.invRepo.EXPECT().FindByID(mock.Anything, "tok").Return(inv, nil).Once()
	s.userRepo.EXPECT().FindByEmail(mock.Anything, "susp@example.com").Return(user, nil).Once()

	res, err := s.svc.AcceptTeamInvitation(context.Background(), "tok", "", "any", "", "")
	s.Require().Error(err)
	s.Nil(res)
	s.True(fiberutil.IsForbidden(err))
}

func TestAcceptTeamInvitationExistingUserRefreshesTeamState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require := testrequire.New(t)
	require.NoError(err)
	require.NoError(db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.TeamInvitation{}))

	hash, err := security.HashPassword("correct-horse")
	require.NoError(err)
	user := &models.User{BaseModel: basemodels.BaseModel{ID: "user"}, Name: "User", Email: "user@example.com", Password: hash, Status: authtypes.UserStatusActive}
	oldTeam := &models.Team{BaseModel: basemodels.BaseModel{ID: "old-team"}, UserID: user.ID, Name: "Personal", PersonalTeam: true}
	newOwner := &models.User{BaseModel: basemodels.BaseModel{ID: "owner"}, Name: "Owner", Email: "owner@example.com"}
	newTeam := &models.Team{BaseModel: basemodels.BaseModel{ID: "new-team"}, UserID: newOwner.ID, Name: "Shared"}
	require.NoError(db.Create(user).Error)
	require.NoError(db.Create(newOwner).Error)
	require.NoError(db.Create(oldTeam).Error)
	require.NoError(db.Create(newTeam).Error)
	require.NoError(db.Model(user).Update("current_team_id", oldTeam.ID).Error)
	role := "editor"
	invitation := &models.TeamInvitation{BaseModel: basemodels.BaseModel{ID: "invite"}, TeamID: newTeam.ID, Email: user.Email, Role: &role, Team: newTeam}
	require.NoError(db.Omit("Team").Create(invitation).Error)
	user.CurrentTeamID = &oldTeam.ID
	user.CurrentTeam = oldTeam

	registry := mocks.NewRepositoryRegistry(t)
	userRepo := mocks.NewUserRepository(t)
	invitationRepo := mocks.NewTeamInvitationRepository(t)
	sessionRepo := mocks.NewSessionRepository(t)
	registry.EXPECT().TeamInvitation().Return(invitationRepo).Maybe()
	registry.EXPECT().User().Return(userRepo).Maybe()
	registry.EXPECT().Session().Return(sessionRepo).Maybe()
	registry.EXPECT().DB().Return(db).Once()
	registry.EXPECT().IsUserAdmin(mock.Anything, user.ID).Return(false).Once()
	registry.EXPECT().IsTeamSubscribed(mock.Anything, newTeam.ID).Return(false).Once()
	invitationRepo.EXPECT().FindByID(mock.Anything, invitation.ID).Return(invitation, nil).Once()
	userRepo.EXPECT().FindByEmail(mock.Anything, user.Email).Return(user, nil).Once()
	sessionRepo.EXPECT().Create(mock.Anything, mock.Anything).Run(func(_ context.Context, session *models.Session) {
		session.ID = "session"
	}).Return(nil).Once()

	logger := zerolog.Nop()
	service := NewAuthService(registry, &config.Config{JWT: config.JWTConfig{Secret: "secret", Expiration: 1}}, &logger, nil)
	cache := &recordingCache{}
	service.SetMembershipCache(launchcache.NewTeamMembershipCache(cache, db))
	result, err := service.AcceptTeamInvitation(context.Background(), invitation.ID, "", "correct-horse", "127.0.0.1", "test")
	require.NoError(err)
	require.Equal(newTeam.ID, *result.User.CurrentTeamID)
	require.NotNil(result.User.CurrentTeam)
	require.Equal(newTeam.ID, result.User.CurrentTeam.ID)
	require.Equal([]string{"team_membership:user:new-team"}, cache.deleted)

	var member models.TeamMember
	require.NoError(db.First(&member, "team_id = ? AND user_id = ?", newTeam.ID, user.ID).Error)
	var savedUser models.User
	require.NoError(db.First(&savedUser, "id = ?", user.ID).Error)
	require.Equal(newTeam.ID, *savedUser.CurrentTeamID)
	var invitationCount int64
	require.NoError(db.Model(&models.TeamInvitation{}).Where("id = ?", invitation.ID).Count(&invitationCount).Error)
	require.Zero(invitationCount)
}

func TestAcceptTeamInvitationExistingUserReturnsCurrentTeamUpdateError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require := testrequire.New(t)
	require.NoError(err)
	require.NoError(db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}))

	hash, err := security.HashPassword("correct-horse")
	require.NoError(err)
	user := &models.User{BaseModel: basemodels.BaseModel{ID: "user"}, Email: "user@example.com", Password: hash, Status: authtypes.UserStatusActive}
	team := &models.Team{BaseModel: basemodels.BaseModel{ID: "team"}, UserID: "owner", Name: "Shared"}
	role := "member"
	require.NoError(db.Create(user).Error)
	require.NoError(db.Create(team).Error)
	require.NoError(db.Create(&models.TeamMember{TeamID: team.ID, UserID: user.ID, Role: &role}).Error)
	require.NoError(db.Migrator().DropTable(&models.User{}))

	registry := mocks.NewRepositoryRegistry(t)
	registry.EXPECT().DB().Return(db).Once()
	logger := zerolog.Nop()
	service := NewAuthService(registry, &config.Config{}, &logger, nil)
	result, err := service.acceptTeamInvitationExistingUser(
		context.Background(),
		&models.TeamInvitation{TeamID: team.ID, Email: user.Email},
		user,
		"correct-horse",
		"",
		"",
	)

	require.Nil(result)
	require.ErrorContains(err, "failed to set current team")
}

func TestServiceWiresMembershipCacheToTeamServices(t *testing.T) {
	cache := launchcache.NewTeamMembershipCache(&recordingCache{}, nil)
	service := &Service{Auth: &AuthService{}, Team: &TeamService{}, TeamMember: &TeamMemberService{}}
	service.SetMembershipCache(cache)
	require := testrequire.New(t)
	require.Same(cache, service.membershipCache)
	require.Same(cache, service.Auth.membershipCache)
	require.Same(cache, service.Team.membershipCache)
	require.Same(cache, service.TeamMember.membershipCache)
}

func TestAcceptTeamInvitationRegistersNewInvitee(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require := testrequire.New(t)
	require.NoError(err)
	require.NoError(db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.TeamInvitation{},
		&models.Session{},
	))
	require.NoError(db.Exec("CREATE TABLE subscriptions (id INTEGER PRIMARY KEY, billable_id TEXT, billable_type TEXT, status TEXT)").Error)

	owner := &models.User{BaseModel: basemodels.BaseModel{ID: "owner"}, Name: "Owner", Email: "owner@example.com"}
	team := &models.Team{BaseModel: basemodels.BaseModel{ID: "team"}, UserID: owner.ID, Name: "Shared"}
	require.NoError(db.Create(owner).Error)
	require.NoError(db.Create(team).Error)
	role := "member"
	invitation := &models.TeamInvitation{BaseModel: basemodels.BaseModel{ID: "invite-new"}, TeamID: team.ID, Email: "new@example.com", Role: &role}
	require.NoError(db.Create(invitation).Error)

	logger := zerolog.Nop()
	service := NewAuthService(
		repositories.NewRegistry(db),
		&config.Config{JWT: config.JWTConfig{Secret: "secret", Expiration: 1}},
		&logger,
		nil,
	)
	cache := &recordingCache{}
	service.SetMembershipCache(launchcache.NewTeamMembershipCache(cache, db))
	result, err := service.AcceptTeamInvitation(
		context.Background(), invitation.ID, "New Member", "password123", "127.0.0.1", "test",
	)
	require.NoError(err)
	require.Equal(team.ID, *result.User.CurrentTeamID)
	require.NotNil(result.User.CurrentTeam)
	require.Equal(team.ID, result.User.CurrentTeam.ID)
	require.Equal([]string{"team_membership:" + result.User.ID + ":" + team.ID}, cache.deleted)

	var membershipCount, invitationCount, personalTeamCount int64
	require.NoError(db.Model(&models.TeamMember{}).Where("team_id = ? AND user_id = ?", team.ID, result.User.ID).Count(&membershipCount).Error)
	require.NoError(db.Model(&models.TeamInvitation{}).Where("id = ?", invitation.ID).Count(&invitationCount).Error)
	require.NoError(db.Model(&models.Team{}).Where("user_id = ? AND personal_team = ?", result.User.ID, true).Count(&personalTeamCount).Error)
	require.EqualValues(1, membershipCount)
	require.Zero(invitationCount)
	require.Zero(personalTeamCount)
}
