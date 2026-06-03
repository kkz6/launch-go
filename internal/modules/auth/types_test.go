package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
)

func TestUserStatus_IsValid(t *testing.T) {
	assert.True(t, authtypes.UserStatusActive.IsValid())
	assert.True(t, authtypes.UserStatusSuspended.IsValid())
	assert.False(t, authtypes.UserStatus("").IsValid())
	assert.False(t, authtypes.UserStatus("frozen").IsValid())
}
