package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
)

func TestValidateActiveServerPHPUsesCanonicalSoftware(t *testing.T) {
	server := &servermodels.Server{
		Services: []servermodels.InstalledService{
			{
				Type:     servertypes.ServiceTypePhp,
				Software: servertypes.SoftwarePhp83.String(),
				Version:  "8.3.26",
				Status:   servertypes.ServiceStatusRunning,
			},
		},
	}

	require.NoError(t, validateActiveServerPHP(server, "php83"))
}

func TestValidateActiveServerPHPRejectsInactiveService(t *testing.T) {
	server := &servermodels.Server{
		Services: []servermodels.InstalledService{
			{
				Type:     servertypes.ServiceTypePhp,
				Software: servertypes.SoftwarePhp83.String(),
				Version:  "8.3.26",
				Status:   servertypes.ServiceStatusStopped,
			},
		},
	}

	err := validateActiveServerPHP(server, "php83")

	require.Error(t, err)
	require.Contains(t, err.Error(), "not active")
}

func TestValidateActiveServerPHPDoesNotMatchPatchVersionOnly(t *testing.T) {
	server := &servermodels.Server{
		Services: []servermodels.InstalledService{
			{
				Type:     servertypes.ServiceTypePhp,
				Software: servertypes.SoftwarePhp84.String(),
				Version:  "8.3",
				Status:   servertypes.ServiceStatusRunning,
			},
		},
	}

	err := validateActiveServerPHP(server, "php83")

	require.Error(t, err)
	require.Contains(t, err.Error(), "not installed")
}

func TestResolveServerPHPVersionPrefersActiveDefault(t *testing.T) {
	server := &servermodels.Server{
		Services: []servermodels.InstalledService{
			{
				Type:      servertypes.ServiceTypePhp,
				Software:  servertypes.SoftwarePhp83.String(),
				Status:    servertypes.ServiceStatusRunning,
				IsDefault: false,
			},
			{
				Type:      servertypes.ServiceTypePhp,
				Software:  servertypes.SoftwarePhp84.String(),
				Status:    servertypes.ServiceStatusInstalled,
				IsDefault: true,
			},
		},
	}

	version, err := resolveServerPHPVersion(server)

	require.NoError(t, err)
	require.Equal(t, servertypes.SoftwarePhp84.String(), version)
}

func TestResolveServerPHPVersionIgnoresInactiveDefault(t *testing.T) {
	server := &servermodels.Server{
		Services: []servermodels.InstalledService{
			{
				Type:      servertypes.ServiceTypePhp,
				Software:  servertypes.SoftwarePhp84.String(),
				Status:    servertypes.ServiceStatusStopped,
				IsDefault: true,
			},
		},
	}

	_, err := resolveServerPHPVersion(server)

	require.Error(t, err)
	require.Contains(t, err.Error(), "current PHP version is unknown")
}
