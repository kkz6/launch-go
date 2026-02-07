package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// --- minimal mock implementations ---

// mockServerRepo implements only FindByIDAndTeam for CheckDomain
type mockServerRepo struct {
	contracts.ServerRepository
	server *models.Server
	err    error
}

func (m *mockServerRepo) FindByIDAndTeam(_ context.Context, _, _ string) (*models.Server, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.server, nil
}

// mockUpstreamRepo is a no-op upstream repo for constructing the registry
type mockUpstreamRepo struct {
	contracts.LoadBalancerUpstreamRepository
}

// mockBackendRepo is a no-op backend repo for constructing the registry
type mockBackendRepo struct {
	contracts.LoadBalancerBackendRepository
}

// mockRegistry implements RepositoryRegistry
type mockRegistry struct {
	serverRepo   contracts.ServerRepository
	upstreamRepo contracts.LoadBalancerUpstreamRepository
	backendRepo  contracts.LoadBalancerBackendRepository
}

func (r *mockRegistry) Server() contracts.ServerRepository { return r.serverRepo }
func (r *mockRegistry) LoadBalancerUpstream() contracts.LoadBalancerUpstreamRepository {
	return r.upstreamRepo
}
func (r *mockRegistry) LoadBalancerBackend() contracts.LoadBalancerBackendRepository {
	return r.backendRepo
}

// unused but required by interface
func (r *mockRegistry) Service() contracts.ServiceRepository               { return nil }
func (r *mockRegistry) FirewallRule() contracts.FirewallRuleRepository     { return nil }
func (r *mockRegistry) Cron() contracts.CronRepository                     { return nil }
func (r *mockRegistry) Daemon() contracts.DaemonRepository                 { return nil }
func (r *mockRegistry) SSHKey() contracts.SSHKeyRepository                 { return nil }
func (r *mockRegistry) Task() contracts.TaskRepository                     { return nil }
func (r *mockRegistry) Metric() contracts.MetricRepository                 { return nil }
func (r *mockRegistry) ServerProvider() contracts.ServerProviderRepository { return nil }
func (r *mockRegistry) Database() contracts.DatabaseRepository             { return nil }
func (r *mockRegistry) DB() *gorm.DB                                       { return nil }

// mockSiteReader implements contracts.SiteReader
type mockSiteReader struct {
	sites []contracts.SiteInfo
	err   error
}

func (m *mockSiteReader) FindByAddressAndTeam(_ context.Context, _, _ string) ([]contracts.SiteInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.sites, nil
}

func (m *mockSiteReader) FindByID(_ context.Context, _ string) (*contracts.SiteInfo, error) {
	return nil, nil
}

func (m *mockSiteReader) IsLoadBalanced(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockSiteReader) UpdateLoadBalancedUpstreamID(_ context.Context, _ string, _ *string) error {
	return nil
}

// helper to create a LoadBalancerService with mocks
func newTestLBService(serverRepo contracts.ServerRepository, siteReader contracts.SiteReader) *LoadBalancerService {
	registry := &mockRegistry{
		serverRepo:   serverRepo,
		upstreamRepo: &mockUpstreamRepo{},
		backendRepo:  &mockBackendRepo{},
	}

	return NewLoadBalancerService(
		service.Dependencies{},
		registry,
		siteReader,
	)
}

func lbServer() *models.Server {
	t := string(types.ServerTypeLoadBalancer)
	s := &models.Server{
		Name: "test-lb",
		Type: &t,
	}
	s.ID = "01HTESTLB0000000000000001"
	s.TeamID = "01HTESTTEAM000000000000001"
	return s
}

// --- Tests ---

func TestCheckDomain_NoSitesFound(t *testing.T) {
	srv := lbServer()
	svc := newTestLBService(
		&mockServerRepo{server: srv},
		&mockSiteReader{sites: nil},
	)

	resp, err := svc.CheckDomain(context.Background(), srv.ID, srv.TeamID, "example.com")
	require.NoError(t, err)

	assert.Equal(t, "example.com", resp.Address)
	assert.False(t, resp.Exists)
	assert.Empty(t, resp.Sites)
	assert.Empty(t, resp.Warning)
}

func TestCheckDomain_ExistingSitesFound(t *testing.T) {
	srv := lbServer()
	sites := []contracts.SiteInfo{
		{
			ID:       "01HTESTSITE0000000000001",
			ServerID: "01HTESTSRV00000000000001",
			TeamID:   srv.TeamID,
			Address:  "example.com",
			Type:     "laravel",
		},
		{
			ID:       "01HTESTSITE0000000000002",
			ServerID: "01HTESTSRV00000000000002",
			TeamID:   srv.TeamID,
			Address:  "example.com",
			Type:     "php",
		},
	}
	svc := newTestLBService(
		&mockServerRepo{server: srv},
		&mockSiteReader{sites: sites},
	)

	resp, err := svc.CheckDomain(context.Background(), srv.ID, srv.TeamID, "example.com")
	require.NoError(t, err)

	assert.Equal(t, "example.com", resp.Address)
	assert.True(t, resp.Exists)
	assert.Len(t, resp.Sites, 2)
	assert.NotEmpty(t, resp.Warning)

	assert.Equal(t, "01HTESTSITE0000000000001", resp.Sites[0].ID)
	assert.Equal(t, "laravel", resp.Sites[0].Type)
	assert.Equal(t, "01HTESTSITE0000000000002", resp.Sites[1].ID)
	assert.Equal(t, "php", resp.Sites[1].Type)
}

func TestCheckDomain_NilSiteReader(t *testing.T) {
	srv := lbServer()
	svc := newTestLBService(
		&mockServerRepo{server: srv},
		nil, // no site reader
	)

	resp, err := svc.CheckDomain(context.Background(), srv.ID, srv.TeamID, "example.com")
	require.NoError(t, err)

	assert.Equal(t, "example.com", resp.Address)
	assert.False(t, resp.Exists)
	assert.Empty(t, resp.Sites)
}

func TestCheckDomain_SiteReaderError(t *testing.T) {
	srv := lbServer()
	svc := newTestLBService(
		&mockServerRepo{server: srv},
		&mockSiteReader{err: assert.AnError},
	)

	// SiteReader error is silently handled — returns empty response
	resp, err := svc.CheckDomain(context.Background(), srv.ID, srv.TeamID, "example.com")
	require.NoError(t, err)

	assert.Equal(t, "example.com", resp.Address)
	assert.False(t, resp.Exists)
}

func TestCheckDomain_ServerNotFound(t *testing.T) {
	svc := newTestLBService(
		&mockServerRepo{err: assert.AnError},
		&mockSiteReader{},
	)

	_, err := svc.CheckDomain(context.Background(), "nonexistent", "team1", "example.com")
	assert.Error(t, err)
}
