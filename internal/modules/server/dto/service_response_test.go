package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func TestToServiceResponseIncludesRemovalCapability(t *testing.T) {
	redis := ToServiceResponse(&models.InstalledService{Software: types.SoftwareRedis.String()})
	assert.True(t, redis.CanRemove)

	caddy := ToServiceResponse(&models.InstalledService{Software: types.SoftwareCaddy2.String()})
	assert.False(t, caddy.CanRemove)
}
