package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestToLoadBalancerUpstreamResponse_Basic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	installedAt := now.Add(-1 * time.Hour)

	upstream := &models.LoadBalancerUpstream{
		Name:                "my-upstream",
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}
	upstream.ID = "upstream-001"
	upstream.TeamID = "team-001"
	upstream.ServerID = "server-001"
	upstream.InstalledAt = &installedAt
	upstream.CreatedAt = &now
	upstream.UpdatedAt = &now

	resp := ToLoadBalancerUpstreamResponse(upstream)

	assert.Equal(t, "upstream-001", resp.ID)
	assert.Equal(t, "server-001", resp.ServerID)
	assert.Equal(t, "team-001", resp.TeamID)
	assert.Equal(t, "my-upstream", resp.Name)
	assert.Equal(t, "app.example.com", resp.Address)
	assert.Equal(t, 443, resp.Port)
	assert.Equal(t, "auto", resp.TLSSetting)
	assert.Equal(t, "round_robin", resp.LBPolicy)
	assert.Equal(t, "Round Robin", resp.LBPolicyLabel)
	assert.Equal(t, "/health", resp.HealthCheckPath)
	assert.Equal(t, "30s", resp.HealthCheckInterval)
	assert.Equal(t, "10s", resp.HealthCheckTimeout)

	assert.NotNil(t, resp.InstalledAt)
	assert.Equal(t, installedAt.Format(time.RFC3339), *resp.InstalledAt)

	assert.Equal(t, now.Format(time.RFC3339), resp.CreatedAt)
	assert.Equal(t, now.Format(time.RFC3339), resp.UpdatedAt)

	assert.Nil(t, resp.Backends)
}

func TestToLoadBalancerUpstreamResponse_WithBackends(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	healthCheckTime := now.Add(-5 * time.Minute)

	upstream := &models.LoadBalancerUpstream{
		Name:                "multi-backend",
		Address:             "lb.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyLeastConn,
		HealthCheckPath:     "/ping",
		HealthCheckInterval: "15s",
		HealthCheckTimeout:  "5s",
		Backends: []models.LoadBalancerBackend{
			{
				BaseModel: basemodels.BaseModel{
					ID:        "backend-001",
					CreatedAt: &now,
					UpdatedAt: &now,
				},
				UpstreamID:        "upstream-002",
				SiteID:            "site-001",
				ServerID:          "server-010",
				Port:              8080,
				IsDown:            false,
				HealthStatus:      types.HealthStatusHealthy,
				LastHealthCheckAt: &healthCheckTime,
			},
			{
				BaseModel: basemodels.BaseModel{
					ID:        "backend-002",
					CreatedAt: &now,
					UpdatedAt: &now,
				},
				UpstreamID:   "upstream-002",
				SiteID:       "site-002",
				ServerID:     "server-011",
				Port:         8081,
				IsDown:       true,
				HealthStatus: types.HealthStatusUnhealthy,
			},
		},
	}
	upstream.ID = "upstream-002"
	upstream.TeamID = "team-002"
	upstream.ServerID = "server-002"
	upstream.CreatedAt = &now
	upstream.UpdatedAt = &now

	resp := ToLoadBalancerUpstreamResponse(upstream)

	assert.Equal(t, "upstream-002", resp.ID)
	assert.Equal(t, "Least Connections", resp.LBPolicyLabel)
	assert.Len(t, resp.Backends, 2)

	b1 := resp.Backends[0]
	assert.Equal(t, "backend-001", b1.ID)
	assert.Equal(t, "upstream-002", b1.UpstreamID)
	assert.Equal(t, "site-001", b1.SiteID)
	assert.Equal(t, "server-010", b1.ServerID)
	assert.Equal(t, 8080, b1.Port)
	assert.False(t, b1.IsDown)
	assert.Equal(t, "healthy", b1.HealthStatus)
	assert.NotNil(t, b1.LastHealthCheckAt)
	assert.Equal(t, healthCheckTime.Format(time.RFC3339), *b1.LastHealthCheckAt)

	b2 := resp.Backends[1]
	assert.Equal(t, "backend-002", b2.ID)
	assert.Equal(t, "site-002", b2.SiteID)
	assert.Equal(t, "server-011", b2.ServerID)
	assert.Equal(t, 8081, b2.Port)
	assert.True(t, b2.IsDown)
	assert.Equal(t, "unhealthy", b2.HealthStatus)
	assert.Nil(t, b2.LastHealthCheckAt)
}

func TestToLoadBalancerUpstreamResponse_InstalledAt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	installedAt := now.Add(-2 * time.Hour)

	t.Run("with InstalledAt set", func(t *testing.T) {
		upstream := &models.LoadBalancerUpstream{
			Name:     "installed-upstream",
			Address:  "installed.example.com",
			Port:     443,
			LBPolicy: types.LBPolicyIPHash,
		}
		upstream.ID = "upstream-010"
		upstream.TeamID = "team-010"
		upstream.ServerID = "server-010"
		upstream.InstalledAt = &installedAt
		upstream.CreatedAt = &now
		upstream.UpdatedAt = &now

		resp := ToLoadBalancerUpstreamResponse(upstream)

		assert.NotNil(t, resp.InstalledAt)
		assert.Equal(t, installedAt.Format(time.RFC3339), *resp.InstalledAt)
	})

	t.Run("with InstalledAt nil", func(t *testing.T) {
		upstream := &models.LoadBalancerUpstream{
			Name:     "pending-upstream",
			Address:  "pending.example.com",
			Port:     443,
			LBPolicy: types.LBPolicyRandom,
		}
		upstream.ID = "upstream-011"
		upstream.TeamID = "team-011"
		upstream.ServerID = "server-011"
		upstream.InstalledAt = nil
		upstream.CreatedAt = &now
		upstream.UpdatedAt = &now

		resp := ToLoadBalancerUpstreamResponse(upstream)

		assert.Nil(t, resp.InstalledAt)
	})
}

func TestToLoadBalancerBackendResponse_Basic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	healthCheckTime := now.Add(-3 * time.Minute)

	backend := &models.LoadBalancerBackend{
		BaseModel: basemodels.BaseModel{
			ID:        "backend-100",
			CreatedAt: &now,
			UpdatedAt: &now,
		},
		UpstreamID:        "upstream-100",
		SiteID:            "site-100",
		ServerID:          "server-100",
		Port:              9090,
		IsDown:            false,
		HealthStatus:      types.HealthStatusHealthy,
		LastHealthCheckAt: &healthCheckTime,
	}

	resp := ToLoadBalancerBackendResponse(backend)

	assert.Equal(t, "backend-100", resp.ID)
	assert.Equal(t, "upstream-100", resp.UpstreamID)
	assert.Equal(t, "site-100", resp.SiteID)
	assert.Equal(t, "server-100", resp.ServerID)
	assert.Equal(t, 9090, resp.Port)
	assert.False(t, resp.IsDown)
	assert.Equal(t, "healthy", resp.HealthStatus)
	assert.NotNil(t, resp.LastHealthCheckAt)
	assert.Equal(t, healthCheckTime.Format(time.RFC3339), *resp.LastHealthCheckAt)
	assert.Equal(t, now.Format(time.RFC3339), resp.CreatedAt)
	assert.Equal(t, now.Format(time.RFC3339), resp.UpdatedAt)
}

func TestToLoadBalancerBackendResponse_WithLastHealthCheck(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	healthCheckTime := now.Add(-10 * time.Minute)

	t.Run("with LastHealthCheckAt set", func(t *testing.T) {
		backend := &models.LoadBalancerBackend{
			BaseModel: basemodels.BaseModel{
				ID:        "backend-200",
				CreatedAt: &now,
				UpdatedAt: &now,
			},
			UpstreamID:        "upstream-200",
			SiteID:            "site-200",
			ServerID:          "server-200",
			Port:              8080,
			IsDown:            true,
			HealthStatus:      types.HealthStatusUnhealthy,
			LastHealthCheckAt: &healthCheckTime,
		}

		resp := ToLoadBalancerBackendResponse(backend)

		assert.NotNil(t, resp.LastHealthCheckAt)
		assert.Equal(t, healthCheckTime.Format(time.RFC3339), *resp.LastHealthCheckAt)
		assert.True(t, resp.IsDown)
		assert.Equal(t, "unhealthy", resp.HealthStatus)
	})

	t.Run("with LastHealthCheckAt nil", func(t *testing.T) {
		backend := &models.LoadBalancerBackend{
			BaseModel: basemodels.BaseModel{
				ID:        "backend-201",
				CreatedAt: &now,
				UpdatedAt: &now,
			},
			UpstreamID:        "upstream-201",
			SiteID:            "site-201",
			ServerID:          "server-201",
			Port:              8080,
			IsDown:            false,
			HealthStatus:      types.HealthStatusUnknown,
			LastHealthCheckAt: nil,
		}

		resp := ToLoadBalancerBackendResponse(backend)

		assert.Nil(t, resp.LastHealthCheckAt)
		assert.False(t, resp.IsDown)
		assert.Equal(t, "unknown", resp.HealthStatus)
	})
}
