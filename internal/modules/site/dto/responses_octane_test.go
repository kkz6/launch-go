package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func strPtr(s string) *string { return &s }

func newTestSite() *models.Site {
	phpVer := sitetypes.PhpVersion("php83")
	now := time.Now()

	site := &models.Site{
		Address:                "example.com",
		Type:                   sitetypes.SiteTypeLaravel,
		TLSSetting:             sitetypes.TLSSettingAuto,
		User:                   "launcher",
		Path:                   "/home/launcher/example.com",
		WebFolder:              "public",
		PhpVersion:             &phpVer,
		ZeroDowntimeDeployment: true,
	}
	site.ID = "01HTEST000000000000000001"
	site.InstallableModel = basemodels.InstallableModel{
		InstalledAt: &now,
	}
	site.ServerID = "01HTEST000000000000000010"
	site.UserID = "01HTEST000000000000000020"

	return site
}

func TestToSiteResponse_WithOctane(t *testing.T) {
	site := newTestSite()
	port := 8042
	server := "frankenphp"
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", QueueID: strPtr("q1"), OctanePort: &port, OctaneServer: &server},
	}

	resp := ToSiteResponse(site)

	require.NotNil(t, resp.OctanePort)
	assert.Equal(t, 8042, *resp.OctanePort)
	require.NotNil(t, resp.OctaneServer)
	assert.Equal(t, "frankenphp", *resp.OctaneServer)
	assert.Contains(t, resp.EnabledFeatures, "octane")
}

func TestToSiteResponse_WithOctaneSwoole(t *testing.T) {
	site := newTestSite()
	port := 8001
	server := "swoole"
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", QueueID: strPtr("q1"), OctanePort: &port, OctaneServer: &server},
	}

	resp := ToSiteResponse(site)

	require.NotNil(t, resp.OctanePort)
	assert.Equal(t, 8001, *resp.OctanePort)
	require.NotNil(t, resp.OctaneServer)
	assert.Equal(t, "swoole", *resp.OctaneServer)
}

func TestToSiteResponse_WithOctaneRoadRunner(t *testing.T) {
	site := newTestSite()
	port := 8002
	server := "roadrunner"
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", QueueID: strPtr("q1"), OctanePort: &port, OctaneServer: &server},
	}

	resp := ToSiteResponse(site)

	require.NotNil(t, resp.OctanePort)
	assert.Equal(t, 8002, *resp.OctanePort)
	require.NotNil(t, resp.OctaneServer)
	assert.Equal(t, "roadrunner", *resp.OctaneServer)
}

func TestToSiteResponse_WithoutOctane(t *testing.T) {
	site := newTestSite()
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "queue", QueueID: strPtr("q1")},
		{Name: "scheduler", CronID: strPtr("c1")},
	}

	resp := ToSiteResponse(site)

	assert.Nil(t, resp.OctanePort)
	assert.Nil(t, resp.OctaneServer)
	assert.Contains(t, resp.EnabledFeatures, "queue")
	assert.Contains(t, resp.EnabledFeatures, "scheduler")
	assert.NotContains(t, resp.EnabledFeatures, "octane")
}

func TestToSiteResponse_EmptyFeatures(t *testing.T) {
	site := newTestSite()

	resp := ToSiteResponse(site)

	assert.Nil(t, resp.OctanePort)
	assert.Nil(t, resp.OctaneServer)
	assert.Empty(t, resp.EnabledFeatures)
}

func TestToSiteResponse_OctaneWithOtherFeatures(t *testing.T) {
	site := newTestSite()
	port := 8000
	server := "frankenphp"
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "scheduler", CronID: strPtr("c1")},
		{Name: "octane", QueueID: strPtr("q1"), OctanePort: &port, OctaneServer: &server},
	}

	resp := ToSiteResponse(site)

	require.NotNil(t, resp.OctanePort)
	assert.Equal(t, 8000, *resp.OctanePort)
	assert.Contains(t, resp.EnabledFeatures, "scheduler")
	assert.Contains(t, resp.EnabledFeatures, "octane")
	assert.Len(t, resp.EnabledFeatures, 2)
}

func TestToSiteResponse_OctaneNilPortInFeature(t *testing.T) {
	site := newTestSite()
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", OctaneServer: strPtr("frankenphp")},
	}

	resp := ToSiteResponse(site)

	assert.Nil(t, resp.OctanePort)
	require.NotNil(t, resp.OctaneServer)
	assert.Equal(t, "frankenphp", *resp.OctaneServer)
}
