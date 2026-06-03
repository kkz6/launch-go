package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts/mocks"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// AuthServiceLoginSuite covers the suspended-account login gate.
type AuthServiceLoginSuite struct {
	suite.Suite

	registry *mocks.RepositoryRegistry
	userRepo *mocks.UserRepository
	svc      *AuthService
}

func (s *AuthServiceLoginSuite) SetupTest() {
	s.registry = mocks.NewRepositoryRegistry(s.T())
	s.userRepo = mocks.NewUserRepository(s.T())
	s.registry.EXPECT().User().Return(s.userRepo).Maybe()

	cfg := &config.Config{}
	logger := zerolog.Nop()
	s.svc = NewAuthService(s.registry, cfg, &logger, nil)
}

func TestAuthServiceLoginSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceLoginSuite))
}

// TestLogin_SuspendedUserRejected verifies a suspended user with valid
// credentials is denied a token (403 Forbidden) before any session/challenge
// is created. The strict mocks assert no session/2FA path is touched.
func (s *AuthServiceLoginSuite) TestLogin_SuspendedUserRejected() {
	hash, err := security.HashPassword("correct-horse")
	s.Require().NoError(err)

	user := &models.User{
		BaseModel: basemodels.BaseModel{ID: "user_susp"},
		Email:     "suspended@example.com",
		Password:  hash,
		Status:    authtypes.UserStatusSuspended,
	}

	s.userRepo.EXPECT().
		FindByEmail(mock.Anything, "suspended@example.com").
		Return(user, nil).
		Once()

	result, err := s.svc.Login(context.Background(), &dto.LoginRequest{
		Email:    "suspended@example.com",
		Password: "correct-horse",
	})

	s.Require().Error(err)
	s.Nil(result)
	s.True(fiberutil.IsForbidden(err), "suspended login should surface as 403 Forbidden")
}

// TestLogin_ActiveUserPassesGate confirms an active user with valid
// credentials is NOT blocked by the suspended gate and proceeds to session
// creation (which the mock satisfies).
func (s *AuthServiceLoginSuite) TestLogin_ActiveUserPassesGate() {
	hash, err := security.HashPassword("correct-horse")
	s.Require().NoError(err)

	user := &models.User{
		BaseModel: basemodels.BaseModel{ID: "user_active"},
		Email:     "active@example.com",
		Password:  hash,
		Status:    authtypes.UserStatusActive,
	}

	sessionRepo := mocks.NewSessionRepository(s.T())
	s.registry.EXPECT().Session().Return(sessionRepo).Maybe()
	sessionRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	s.registry.EXPECT().IsUserAdmin(mock.Anything, "user_active").Return(false).Maybe()

	s.userRepo.EXPECT().
		FindByEmail(mock.Anything, "active@example.com").
		Return(user, nil).
		Once()

	result, err := s.svc.Login(context.Background(), &dto.LoginRequest{
		Email:    "active@example.com",
		Password: "correct-horse",
	})

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.NotNil(result.AuthResponse)
	s.NotEmpty(result.AuthResponse.AccessToken)
}
