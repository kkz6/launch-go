package models

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func TestInstalledServicePhpRuntimeIdentityUsesSoftwareSeries(t *testing.T) {
	t.Parallel()

	service := &InstalledService{
		Type:     types.ServiceTypePhp,
		Software: types.SoftwarePhp83.String(),
		Version:  "8.3.6",
	}

	assert.Equal(t, "8.3", service.PhpVersionSeries())
	assert.Equal(t, "php8.3-fpm", service.GetServiceName())
}

func TestInstalledServicePhpRuntimeIdentityFallsBackToDetectedVersion(t *testing.T) {
	t.Parallel()

	service := &InstalledService{
		Type:    types.ServiceTypePhp,
		Version: "8.2.21",
	}

	assert.Equal(t, "8.2", service.PhpVersionSeries())
	assert.Equal(t, "php8.2-fpm", service.GetServiceName())
}
