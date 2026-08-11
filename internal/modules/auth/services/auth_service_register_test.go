package services

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts/mocks"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
)

func TestRegisterRejectsExistingEmail(t *testing.T) {
	registry := mocks.NewRepositoryRegistry(t)
	users := mocks.NewUserRepository(t)
	registry.EXPECT().User().Return(users).Once()
	users.EXPECT().ExistsByEmail(mock.Anything, "member@example.com").Return(true, nil).Once()

	logger := zerolog.Nop()
	service := NewAuthService(registry, &config.Config{}, &logger, nil)
	result, err := service.Register(context.Background(), &dto.RegisterRequest{Email: "MEMBER@example.com"})

	require.Nil(t, result)
	require.EqualError(t, err, "email already registered")
}

func TestRegisterReturnsEmailLookupError(t *testing.T) {
	registry := mocks.NewRepositoryRegistry(t)
	users := mocks.NewUserRepository(t)
	lookupErr := errors.New("lookup failed")
	registry.EXPECT().User().Return(users).Once()
	users.EXPECT().ExistsByEmail(mock.Anything, "member@example.com").Return(false, lookupErr).Once()

	logger := zerolog.Nop()
	service := NewAuthService(registry, &config.Config{}, &logger, nil)
	result, err := service.Register(context.Background(), &dto.RegisterRequest{Email: "member@example.com"})

	require.Nil(t, result)
	require.ErrorIs(t, err, lookupErr)
}
