package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts/mocks"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
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
