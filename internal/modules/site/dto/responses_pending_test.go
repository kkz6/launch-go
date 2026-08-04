package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

func TestToSiteResponseIncludesPendingConfigurationState(t *testing.T) {
	pendingTLS := time.Date(2026, time.July, 29, 10, 15, 0, 0, time.UTC)
	pendingCaddy := pendingTLS.Add(time.Minute)
	pendingPHP := sitetypes.PhpVersion84
	site := &models.Site{
		PendingTLSUpdateSince:       &pendingTLS,
		PendingCaddyfileUpdateSince: &pendingCaddy,
		PendingPhpVersion:           &pendingPHP,
	}

	response := ToSiteResponse(site)

	require.NotNil(t, response.PendingTLSUpdateSince)
	require.NotNil(t, response.PendingCaddyfileUpdateSince)
	require.Equal(t, "php84", response.PendingPhpVersion)
	require.Contains(t, *response.PendingTLSUpdateSince, "2026-07-29")
	require.Contains(t, *response.PendingCaddyfileUpdateSince, "2026-07-29")
}
