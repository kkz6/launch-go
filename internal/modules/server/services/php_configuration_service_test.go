package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func TestPHPConfigurationForService(t *testing.T) {
	service := &models.InstalledService{
		Type:     types.ServiceTypePhp,
		Software: string(types.SoftwarePhp83),
	}

	ini, err := phpConfigurationForService(service, PHPConfigurationKindINI)
	require.NoError(t, err)
	require.Equal(t, "8.3", ini.Version)
	require.Equal(t, "/etc/php/8.3/fpm/php.ini", ini.Path)
	require.Equal(t, "php.ini", ini.Label)

	fpm, err := phpConfigurationForService(service, PHPConfigurationKindFPM)
	require.NoError(t, err)
	require.Equal(t, "/etc/php/8.3/fpm/php-fpm.conf", fpm.Path)
}

func TestPHPConfigurationForServiceRejectsUnknownKind(t *testing.T) {
	service := &models.InstalledService{
		Type:     types.ServiceTypePhp,
		Software: string(types.SoftwarePhp83),
	}

	_, err := phpConfigurationForService(service, "arbitrary_file")
	require.Error(t, err)
}

func TestPHPConfigurationForServiceRejectsNonPHPService(t *testing.T) {
	service := &models.InstalledService{
		Type:     types.ServiceTypeRedis,
		Software: string(types.SoftwareRedis),
	}

	_, err := phpConfigurationForService(service, PHPConfigurationKindINI)
	require.Error(t, err)
}
