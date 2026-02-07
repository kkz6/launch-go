package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

// =============================================================================
// Mock implementations
// =============================================================================

// mockServerRepo implements ServerRepository for tests
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

// fullMockUpstreamRepo provides full mock upstream repo with configurable behavior
type fullMockUpstreamRepo struct {
	contracts.LoadBalancerUpstreamRepository

	// stored data
	upstreams map[string]*models.LoadBalancerUpstream

	// configurable returns
	findByServerIDResult        []models.LoadBalancerUpstream
	findByServerIDAndAddrResult *models.LoadBalancerUpstream
	createErr                   error
	updateErr                   error
	deleteErr                   error

	// tracking calls
	createdUpstream *models.LoadBalancerUpstream
	updatedID       string
	updatedFields   map[string]any
	deletedID       string
}

func (m *fullMockUpstreamRepo) Create(_ context.Context, upstream *models.LoadBalancerUpstream) error {
	if m.createErr != nil {
		return m.createErr
	}
	if upstream.ID == "" {
		upstream.ID = "01HTESTUPSTREAM000000001"
	}
	m.createdUpstream = upstream
	if m.upstreams == nil {
		m.upstreams = make(map[string]*models.LoadBalancerUpstream)
	}
	m.upstreams[upstream.ID] = upstream
	return nil
}

func (m *fullMockUpstreamRepo) FindByID(_ context.Context, id string) (*models.LoadBalancerUpstream, error) {
	if m.upstreams != nil {
		if u, ok := m.upstreams[id]; ok {
			return u, nil
		}
	}
	return nil, errors.New("upstream not found")
}

func (m *fullMockUpstreamRepo) FindByIDWithBackends(_ context.Context, id string) (*models.LoadBalancerUpstream, error) {
	if m.upstreams != nil {
		if u, ok := m.upstreams[id]; ok {
			return u, nil
		}
	}
	return nil, errors.New("upstream not found")
}

func (m *fullMockUpstreamRepo) FindByServerID(_ context.Context, _ string) ([]models.LoadBalancerUpstream, error) {
	return m.findByServerIDResult, nil
}

func (m *fullMockUpstreamRepo) FindByServerIDAndAddress(_ context.Context, _, _ string) (*models.LoadBalancerUpstream, error) {
	return m.findByServerIDAndAddrResult, nil
}

func (m *fullMockUpstreamRepo) Update(_ context.Context, id string, updates map[string]any) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updatedID = id
	m.updatedFields = updates
	return nil
}

func (m *fullMockUpstreamRepo) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deletedID = id
	return nil
}

// fullMockBackendRepo provides full mock backend repo
type fullMockBackendRepo struct {
	contracts.LoadBalancerBackendRepository

	// stored data
	backends map[string]*models.LoadBalancerBackend

	// configurable returns
	findByUpstreamAndSiteResult *models.LoadBalancerBackend
	findByUpstreamIDResult      []models.LoadBalancerBackend

	createErr           error
	updateErr           error
	deleteErr           error
	deleteByUpstreamErr error

	// tracking calls
	createdBackend    *models.LoadBalancerBackend
	updatedID         string
	updatedFields     map[string]any
	deletedID         string
	deletedUpstreamID string
}

func (m *fullMockBackendRepo) Create(_ context.Context, backend *models.LoadBalancerBackend) error {
	if m.createErr != nil {
		return m.createErr
	}
	if backend.ID == "" {
		backend.ID = "01HTESTBACKEND0000000001"
	}
	m.createdBackend = backend
	if m.backends == nil {
		m.backends = make(map[string]*models.LoadBalancerBackend)
	}
	m.backends[backend.ID] = backend
	return nil
}

func (m *fullMockBackendRepo) FindByID(_ context.Context, id string) (*models.LoadBalancerBackend, error) {
	if m.backends != nil {
		if b, ok := m.backends[id]; ok {
			return b, nil
		}
	}
	return nil, errors.New("backend not found")
}

func (m *fullMockBackendRepo) FindByUpstreamAndSite(_ context.Context, _, _ string) (*models.LoadBalancerBackend, error) {
	return m.findByUpstreamAndSiteResult, nil
}

func (m *fullMockBackendRepo) FindByUpstreamID(_ context.Context, _ string) ([]models.LoadBalancerBackend, error) {
	return m.findByUpstreamIDResult, nil
}

func (m *fullMockBackendRepo) Update(_ context.Context, id string, updates map[string]any) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updatedID = id
	m.updatedFields = updates
	return nil
}

func (m *fullMockBackendRepo) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deletedID = id
	return nil
}

func (m *fullMockBackendRepo) DeleteByUpstreamID(_ context.Context, upstreamID string) error {
	if m.deleteByUpstreamErr != nil {
		return m.deleteByUpstreamErr
	}
	m.deletedUpstreamID = upstreamID
	return nil
}

// fullMockSiteReader provides configurable site reader mock
type fullMockSiteReader struct {
	sites    []contracts.SiteInfo
	siteByID map[string]*contracts.SiteInfo
	findErr  error

	updatedSiteIDs     []string
	updatedUpstreamIDs []*string
}

func (m *fullMockSiteReader) FindByAddressAndTeam(_ context.Context, _, _ string) ([]contracts.SiteInfo, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.sites, nil
}

func (m *fullMockSiteReader) FindByID(_ context.Context, id string) (*contracts.SiteInfo, error) {
	if m.siteByID != nil {
		if s, ok := m.siteByID[id]; ok {
			return s, nil
		}
	}
	return nil, nil
}

func (m *fullMockSiteReader) IsLoadBalanced(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *fullMockSiteReader) UpdateLoadBalancedUpstreamID(_ context.Context, siteID string, upstreamID *string) error {
	m.updatedSiteIDs = append(m.updatedSiteIDs, siteID)
	m.updatedUpstreamIDs = append(m.updatedUpstreamIDs, upstreamID)
	return nil
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

// =============================================================================
// Helper constructors
// =============================================================================

func newFullTestLBService(
	serverRepo contracts.ServerRepository,
	upstreamRepo contracts.LoadBalancerUpstreamRepository,
	backendRepo contracts.LoadBalancerBackendRepository,
	siteReader contracts.SiteReader,
) *LoadBalancerService {
	registry := &mockRegistry{
		serverRepo:   serverRepo,
		upstreamRepo: upstreamRepo,
		backendRepo:  backendRepo,
	}
	return NewLoadBalancerService(service.Dependencies{}, registry, siteReader)
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

func nonLBServer() *models.Server {
	t := "app"
	s := &models.Server{
		Name: "test-app",
		Type: &t,
	}
	s.ID = "01HTESTAPP0000000000001"
	s.TeamID = "01HTESTTEAM000000000000001"
	return s
}

func testUpstream(serverID, teamID string) *models.LoadBalancerUpstream {
	u := &models.LoadBalancerUpstream{
		Name:                "test-upstream",
		Address:             "example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}
	u.ID = "01HTESTUPSTREAM000000001"
	u.ServerID = serverID
	u.TeamID = teamID
	return u
}

func testBackend(upstreamID, siteID, serverID string) *models.LoadBalancerBackend {
	b := &models.LoadBalancerBackend{
		UpstreamID:   upstreamID,
		SiteID:       siteID,
		ServerID:     serverID,
		Port:         8080,
		IsDown:       false,
		HealthStatus: types.HealthStatusUnknown,
	}
	b.ID = "01HTESTBACKEND0000000001"
	return b
}

// =============================================================================
// CheckDomain tests
// =============================================================================

func TestCheckDomain_NoSitesFound(t *testing.T) {
	srv := lbServer()
	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		&fullMockSiteReader{sites: nil},
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
	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		&fullMockSiteReader{sites: sites},
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
	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
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
	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		&fullMockSiteReader{findErr: assert.AnError},
	)

	resp, err := svc.CheckDomain(context.Background(), srv.ID, srv.TeamID, "example.com")
	require.NoError(t, err)

	assert.Equal(t, "example.com", resp.Address)
	assert.False(t, resp.Exists)
}

func TestCheckDomain_ServerNotFound(t *testing.T) {
	svc := newFullTestLBService(
		&mockServerRepo{err: assert.AnError},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		&fullMockSiteReader{},
	)

	_, err := svc.CheckDomain(context.Background(), "nonexistent", "team1", "example.com")
	assert.Error(t, err)
}

// =============================================================================
// ListUpstreams tests
// =============================================================================

func TestListUpstreams_Success(t *testing.T) {
	srv := lbServer()
	upstream1 := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		findByServerIDResult: []models.LoadBalancerUpstream{*upstream1},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	result, err := svc.ListUpstreams(context.Background(), srv.ID, srv.TeamID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "example.com", result[0].Address)
}

func TestListUpstreams_NotLoadBalancer(t *testing.T) {
	srv := nonLBServer()
	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		nil,
	)

	_, err := svc.ListUpstreams(context.Background(), srv.ID, srv.TeamID)
	assert.Error(t, err)
	assert.Equal(t, ErrNotLoadBalancer, err)
}

func TestListUpstreams_NilServerType(t *testing.T) {
	srv := &models.Server{Name: "no-type"}
	srv.ID = "01HTESTNULL0000000000001"
	srv.TeamID = "01HTESTTEAM000000000000001"
	// Type is nil

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		nil,
	)

	_, err := svc.ListUpstreams(context.Background(), srv.ID, srv.TeamID)
	assert.Error(t, err)
	assert.Equal(t, ErrNotLoadBalancer, err)
}

func TestListUpstreams_ServerNotFound(t *testing.T) {
	svc := newFullTestLBService(
		&mockServerRepo{err: assert.AnError},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		nil,
	)

	_, err := svc.ListUpstreams(context.Background(), "nonexistent", "team1")
	assert.Error(t, err)
}

// =============================================================================
// GetUpstream tests
// =============================================================================

func TestGetUpstream_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	result, err := svc.GetUpstream(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)
	assert.Equal(t, upstream.ID, result.ID)
	assert.Equal(t, "example.com", result.Address)
}

func TestGetUpstream_ServerMismatch(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream("01HDIFFERENTSERVER000001", srv.TeamID) // different server ID
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	_, err := svc.GetUpstream(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	assert.Error(t, err)
}

func TestGetUpstream_NotFound(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	_, err := svc.GetUpstream(context.Background(), srv.ID, srv.TeamID, "nonexistent")
	assert.Error(t, err)
}

// =============================================================================
// CreateUpstream tests
// =============================================================================

func TestCreateUpstream_Success(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: make(map[string]*models.LoadBalancerUpstream),
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	req := &dto.CreateUpstreamRequest{
		Name:     "My Upstream",
		Address:  "example.com",
		LBPolicy: "round_robin",
	}

	result, err := svc.CreateUpstream(context.Background(), srv.ID, srv.TeamID, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "My Upstream", result.Name)
	assert.Equal(t, "example.com", result.Address)
	assert.Equal(t, 443, result.Port)                  // default
	assert.Equal(t, "auto", result.TLSSetting)         // default
	assert.Equal(t, "/health", result.HealthCheckPath) // default
	assert.Equal(t, "30s", result.HealthCheckInterval) // default
	assert.Equal(t, "10s", result.HealthCheckTimeout)  // default
	assert.Equal(t, types.LBPolicyRoundRobin, result.LBPolicy)
}

func TestCreateUpstream_CustomValues(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: make(map[string]*models.LoadBalancerUpstream),
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	req := &dto.CreateUpstreamRequest{
		Name:                "Custom Upstream",
		Address:             "custom.example.com",
		Port:                8443,
		TLSSetting:          "internal",
		LBPolicy:            "ip_hash",
		HealthCheckPath:     "/status",
		HealthCheckInterval: "60s",
		HealthCheckTimeout:  "5s",
	}

	result, err := svc.CreateUpstream(context.Background(), srv.ID, srv.TeamID, req)
	require.NoError(t, err)
	assert.Equal(t, 8443, result.Port)
	assert.Equal(t, "internal", result.TLSSetting)
	assert.Equal(t, "/status", result.HealthCheckPath)
	assert.Equal(t, "60s", result.HealthCheckInterval)
	assert.Equal(t, "5s", result.HealthCheckTimeout)
	assert.Equal(t, types.LBPolicyIPHash, result.LBPolicy)
}

func TestCreateUpstream_NotLoadBalancer(t *testing.T) {
	srv := nonLBServer()
	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		nil,
	)

	req := &dto.CreateUpstreamRequest{
		Name:     "Test",
		Address:  "example.com",
		LBPolicy: "round_robin",
	}

	_, err := svc.CreateUpstream(context.Background(), srv.ID, srv.TeamID, req)
	assert.Equal(t, ErrNotLoadBalancer, err)
}

func TestCreateUpstream_DuplicateAddress(t *testing.T) {
	srv := lbServer()
	existing := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		findByServerIDAndAddrResult: existing,
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	req := &dto.CreateUpstreamRequest{
		Name:     "Duplicate",
		Address:  "example.com",
		LBPolicy: "round_robin",
	}

	_, err := svc.CreateUpstream(context.Background(), srv.ID, srv.TeamID, req)
	assert.Equal(t, ErrUpstreamExists, err)
}

func TestCreateUpstream_AutoAddExistingSites(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: make(map[string]*models.LoadBalancerUpstream),
	}
	backendRepo := &fullMockBackendRepo{}

	siteReader := &fullMockSiteReader{
		sites: []contracts.SiteInfo{
			{
				ID:       "01HTESTSITE0000000000001",
				ServerID: "01HTESTSRV00000000000001",
				TeamID:   srv.TeamID,
				Address:  "example.com",
				Type:     "laravel",
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	req := &dto.CreateUpstreamRequest{
		Name:                 "Auto-add",
		Address:              "example.com",
		LBPolicy:             "round_robin",
		AutoAddExistingSites: true,
	}

	_, err := svc.CreateUpstream(context.Background(), srv.ID, srv.TeamID, req)
	require.NoError(t, err)

	// Backend should have been created for the existing site
	assert.NotNil(t, backendRepo.createdBackend)
	assert.Equal(t, "01HTESTSITE0000000000001", backendRepo.createdBackend.SiteID)
	assert.Equal(t, 8080, backendRepo.createdBackend.Port)
}

func TestCreateUpstream_AutoAddSkipsAlreadyBalanced(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: make(map[string]*models.LoadBalancerUpstream),
	}
	backendRepo := &fullMockBackendRepo{}

	existingUpstreamID := "01HEXISTINGUPSTREAM00001"
	siteReader := &fullMockSiteReader{
		sites: []contracts.SiteInfo{
			{
				ID:                     "01HTESTSITE0000000000001",
				ServerID:               "01HTESTSRV00000000000001",
				TeamID:                 srv.TeamID,
				Address:                "example.com",
				Type:                   "laravel",
				LoadBalancedUpstreamID: &existingUpstreamID, // already load balanced
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	req := &dto.CreateUpstreamRequest{
		Name:                 "Auto-add",
		Address:              "example.com",
		LBPolicy:             "round_robin",
		AutoAddExistingSites: true,
	}

	_, err := svc.CreateUpstream(context.Background(), srv.ID, srv.TeamID, req)
	require.NoError(t, err)

	// Backend should NOT have been created (site already balanced)
	assert.Nil(t, backendRepo.createdBackend)
}

// =============================================================================
// UpdateUpstream tests
// =============================================================================

func TestUpdateUpstream_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	newName := "Updated Name"
	newPolicy := "least_conn"
	req := &dto.UpdateUpstreamRequest{
		Name:     &newName,
		LBPolicy: &newPolicy,
	}

	result, err := svc.UpdateUpstream(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, upstream.ID, upstreamRepo.updatedID)
	assert.Equal(t, "Updated Name", upstreamRepo.updatedFields["name"])
	assert.Equal(t, "least_conn", upstreamRepo.updatedFields["lb_policy"])
}

func TestUpdateUpstream_NoChanges(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	req := &dto.UpdateUpstreamRequest{} // all nil

	result, err := svc.UpdateUpstream(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	require.NoError(t, err)
	assert.Equal(t, upstream.ID, result.ID)
	assert.Empty(t, upstreamRepo.updatedID) // Update should not be called
}

func TestUpdateUpstream_AllFields(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	name := "New Name"
	policy := "ip_hash"
	path := "/status"
	interval := "60s"
	timeout := "5s"
	req := &dto.UpdateUpstreamRequest{
		Name:                &name,
		LBPolicy:            &policy,
		HealthCheckPath:     &path,
		HealthCheckInterval: &interval,
		HealthCheckTimeout:  &timeout,
	}

	_, err := svc.UpdateUpstream(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 5, len(upstreamRepo.updatedFields))
	assert.Equal(t, "New Name", upstreamRepo.updatedFields["name"])
	assert.Equal(t, "ip_hash", upstreamRepo.updatedFields["lb_policy"])
	assert.Equal(t, "/status", upstreamRepo.updatedFields["health_check_path"])
	assert.Equal(t, "60s", upstreamRepo.updatedFields["health_check_interval"])
	assert.Equal(t, "5s", upstreamRepo.updatedFields["health_check_timeout"])
}

// =============================================================================
// DeleteUpstream tests
// =============================================================================

func TestDeleteUpstream_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")
	upstream.Backends = []models.LoadBalancerBackend{*backend}

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{}
	siteReader := &fullMockSiteReader{}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	err := svc.DeleteUpstream(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)

	// Backends should be deleted by upstream ID
	assert.Equal(t, upstream.ID, backendRepo.deletedUpstreamID)
	// Upstream should be deleted
	assert.Equal(t, upstream.ID, upstreamRepo.deletedID)
	// Site should be un-marked as load balanced (upstream ID set to nil)
	assert.Len(t, siteReader.updatedSiteIDs, 1)
	assert.Equal(t, "01HTESTSITE0000000000001", siteReader.updatedSiteIDs[0])
	assert.Nil(t, siteReader.updatedUpstreamIDs[0])
}

func TestDeleteUpstream_NotFound(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	err := svc.DeleteUpstream(context.Background(), srv.ID, srv.TeamID, "nonexistent")
	assert.Error(t, err)
}

// =============================================================================
// AddBackend tests
// =============================================================================

func TestAddBackend_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{}

	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{
			"01HTESTSITE0000000000001": {
				ID:       "01HTESTSITE0000000000001",
				ServerID: "01HTESTSRV00000000000001",
				TeamID:   srv.TeamID,
				Address:  "example.com",
				Type:     "laravel",
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	req := &dto.AddBackendRequest{
		SiteID: "01HTESTSITE0000000000001",
	}

	result, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 8080, result.Port) // default port
	assert.Equal(t, upstream.ID, result.UpstreamID)
	assert.Equal(t, "01HTESTSITE0000000000001", result.SiteID)
	assert.Equal(t, types.HealthStatusUnknown, result.HealthStatus)

	// Site should be marked as load balanced
	assert.Len(t, siteReader.updatedSiteIDs, 1)
	assert.Equal(t, "01HTESTSITE0000000000001", siteReader.updatedSiteIDs[0])
}

func TestAddBackend_CustomPort(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{}

	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{
			"01HTESTSITE0000000000001": {
				ID:       "01HTESTSITE0000000000001",
				ServerID: "01HTESTSRV00000000000001",
				TeamID:   srv.TeamID,
				Address:  "example.com",
				Type:     "laravel",
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	req := &dto.AddBackendRequest{
		SiteID: "01HTESTSITE0000000000001",
		Port:   9090,
	}

	result, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 9090, result.Port)
}

func TestAddBackend_SiteNotFound(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{}, // empty
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		siteReader,
	)

	req := &dto.AddBackendRequest{SiteID: "nonexistent"}

	_, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	assert.Error(t, err)
}

func TestAddBackend_AddressMismatch(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID) // address: example.com
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{
			"01HTESTSITE0000000000001": {
				ID:       "01HTESTSITE0000000000001",
				ServerID: "01HTESTSRV00000000000001",
				TeamID:   srv.TeamID,
				Address:  "different.com", // doesn't match upstream
				Type:     "laravel",
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		siteReader,
	)

	req := &dto.AddBackendRequest{SiteID: "01HTESTSITE0000000000001"}

	_, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	assert.Equal(t, ErrAddressMismatch, err)
}

func TestAddBackend_SiteAlreadyBalanced(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	existingUpstream := "01HEXISTINGUPSTREAM00001"
	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{
			"01HTESTSITE0000000000001": {
				ID:                     "01HTESTSITE0000000000001",
				ServerID:               "01HTESTSRV00000000000001",
				TeamID:                 srv.TeamID,
				Address:                "example.com",
				Type:                   "laravel",
				LoadBalancedUpstreamID: &existingUpstream,
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		siteReader,
	)

	req := &dto.AddBackendRequest{SiteID: "01HTESTSITE0000000000001"}

	_, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	assert.Equal(t, ErrSiteAlreadyBalanced, err)
}

func TestAddBackend_DifferentTeam(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{
			"01HTESTSITE0000000000001": {
				ID:       "01HTESTSITE0000000000001",
				ServerID: "01HTESTSRV00000000000001",
				TeamID:   "01HDIFFERENTTEAM0000001", // different team
				Address:  "example.com",
				Type:     "laravel",
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		siteReader,
	)

	req := &dto.AddBackendRequest{SiteID: "01HTESTSITE0000000000001"}

	_, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	assert.Error(t, err)
}

func TestAddBackend_NilSiteReader(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil, // no site reader
	)

	req := &dto.AddBackendRequest{SiteID: "01HTESTSITE0000000000001"}

	_, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	assert.Error(t, err)
}

func TestAddBackend_DuplicateReturnsExisting(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	existingBackend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")
	backendRepo := &fullMockBackendRepo{
		findByUpstreamAndSiteResult: existingBackend,
	}

	siteReader := &fullMockSiteReader{
		siteByID: map[string]*contracts.SiteInfo{
			"01HTESTSITE0000000000001": {
				ID:       "01HTESTSITE0000000000001",
				ServerID: "01HTESTSRV00000000000001",
				TeamID:   srv.TeamID,
				Address:  "example.com",
				Type:     "laravel",
			},
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	req := &dto.AddBackendRequest{SiteID: "01HTESTSITE0000000000001"}

	result, err := svc.AddBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, req)
	require.NoError(t, err)
	assert.Equal(t, existingBackend.ID, result.ID)
	assert.Nil(t, backendRepo.createdBackend) // should NOT create new
}

// =============================================================================
// UpdateBackend tests
// =============================================================================

func TestUpdateBackend_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	newPort := 9090
	req := &dto.UpdateBackendRequest{Port: &newPort}

	result, err := svc.UpdateBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, backend.ID, backendRepo.updatedID)
	assert.Equal(t, 9090, backendRepo.updatedFields["port"])
}

func TestUpdateBackend_NoChanges(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	req := &dto.UpdateBackendRequest{} // all nil

	result, err := svc.UpdateBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID, req)
	require.NoError(t, err)
	assert.Equal(t, backend.ID, result.ID)
	assert.Empty(t, backendRepo.updatedID) // Update should not be called
}

func TestUpdateBackend_WrongUpstream(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend("01HDIFFERENTUPSTREAM0001", "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	newPort := 9090
	req := &dto.UpdateBackendRequest{Port: &newPort}

	_, err := svc.UpdateBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID, req)
	assert.Error(t, err)
}

// =============================================================================
// RemoveBackend tests
// =============================================================================

func TestRemoveBackend_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}
	siteReader := &fullMockSiteReader{}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		siteReader,
	)

	err := svc.RemoveBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID)
	require.NoError(t, err)

	// Backend should be deleted
	assert.Equal(t, backend.ID, backendRepo.deletedID)
	// Site should be un-marked as load balanced
	assert.Len(t, siteReader.updatedSiteIDs, 1)
	assert.Equal(t, "01HTESTSITE0000000000001", siteReader.updatedSiteIDs[0])
	assert.Nil(t, siteReader.updatedUpstreamIDs[0])
}

func TestRemoveBackend_WrongUpstream(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend("01HDIFFERENTUPSTREAM0001", "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	err := svc.RemoveBackend(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID)
	assert.Error(t, err)
}

// =============================================================================
// ToggleBackendDown tests
// =============================================================================

func TestToggleBackendDown_MarkDown(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")
	backend.IsDown = false

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	_, err := svc.ToggleBackendDown(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID)
	require.NoError(t, err)

	assert.Equal(t, backend.ID, backendRepo.updatedID)
	assert.Equal(t, true, backendRepo.updatedFields["is_down"])
}

func TestToggleBackendDown_MarkUp(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	backend := testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001")
	backend.IsDown = true // currently down

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}
	backendRepo := &fullMockBackendRepo{
		backends: map[string]*models.LoadBalancerBackend{
			backend.ID: backend,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	_, err := svc.ToggleBackendDown(context.Background(), srv.ID, srv.TeamID, upstream.ID, backend.ID)
	require.NoError(t, err)

	assert.Equal(t, backend.ID, backendRepo.updatedID)
	assert.Equal(t, false, backendRepo.updatedFields["is_down"])
}

// =============================================================================
// GetUpstreamHealth tests
// =============================================================================

func TestGetUpstreamHealth_AllHealthy(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)

	b1 := models.LoadBalancerBackend{
		UpstreamID:   upstream.ID,
		SiteID:       "01HTESTSITE0000000000001",
		ServerID:     "01HTESTSRV00000000000001",
		Port:         8080,
		IsDown:       false,
		HealthStatus: types.HealthStatusHealthy,
	}
	b1.ID = "01HTESTBACKEND0000000001"

	b2 := models.LoadBalancerBackend{
		UpstreamID:   upstream.ID,
		SiteID:       "01HTESTSITE0000000000002",
		ServerID:     "01HTESTSRV00000000000002",
		Port:         8080,
		IsDown:       false,
		HealthStatus: types.HealthStatusHealthy,
	}
	b2.ID = "01HTESTBACKEND0000000002"

	upstream.Backends = []models.LoadBalancerBackend{b1, b2}

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	health, err := svc.GetUpstreamHealth(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)
	assert.Equal(t, upstream.ID, health.UpstreamID)
	assert.Equal(t, "example.com", health.Address)
	assert.Equal(t, 2, health.TotalBackends)
	assert.Equal(t, 2, health.HealthyBackends)
	assert.Len(t, health.Backends, 2)
}

func TestGetUpstreamHealth_MixedStatus(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)

	b1 := models.LoadBalancerBackend{
		UpstreamID:   upstream.ID,
		SiteID:       "01HTESTSITE0000000000001",
		ServerID:     "01HTESTSRV00000000000001",
		Port:         8080,
		IsDown:       false,
		HealthStatus: types.HealthStatusHealthy,
	}
	b1.ID = "01HTESTBACKEND0000000001"

	b2 := models.LoadBalancerBackend{
		UpstreamID:   upstream.ID,
		SiteID:       "01HTESTSITE0000000000002",
		ServerID:     "01HTESTSRV00000000000002",
		Port:         8080,
		IsDown:       false,
		HealthStatus: types.HealthStatusUnhealthy,
	}
	b2.ID = "01HTESTBACKEND0000000002"

	b3 := models.LoadBalancerBackend{
		UpstreamID:   upstream.ID,
		SiteID:       "01HTESTSITE0000000000003",
		ServerID:     "01HTESTSRV00000000000003",
		Port:         8080,
		IsDown:       true, // down, should not count as healthy
		HealthStatus: types.HealthStatusHealthy,
	}
	b3.ID = "01HTESTBACKEND0000000003"

	upstream.Backends = []models.LoadBalancerBackend{b1, b2, b3}

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	health, err := svc.GetUpstreamHealth(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, health.TotalBackends)
	assert.Equal(t, 1, health.HealthyBackends) // only b1 is healthy AND not down
}

func TestGetUpstreamHealth_NoBackends(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstream.Backends = nil

	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	health, err := svc.GetUpstreamHealth(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, health.TotalBackends)
	assert.Equal(t, 0, health.HealthyBackends)
}

// =============================================================================
// ListBackends tests
// =============================================================================

func TestListBackends_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	backends := []models.LoadBalancerBackend{
		*testBackend(upstream.ID, "01HTESTSITE0000000000001", "01HTESTSRV00000000000001"),
	}
	backendRepo := &fullMockBackendRepo{
		findByUpstreamIDResult: backends,
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		backendRepo,
		nil,
	)

	result, err := svc.ListBackends(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// =============================================================================
// TriggerHealthCheck tests
// =============================================================================

func TestTriggerHealthCheck_Success(t *testing.T) {
	srv := lbServer()
	upstream := testUpstream(srv.ID, srv.TeamID)
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{
			upstream.ID: upstream,
		},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	err := svc.TriggerHealthCheck(context.Background(), srv.ID, srv.TeamID, upstream.ID)
	require.NoError(t, err)
}

func TestTriggerHealthCheck_UpstreamNotFound(t *testing.T) {
	srv := lbServer()
	upstreamRepo := &fullMockUpstreamRepo{
		upstreams: map[string]*models.LoadBalancerUpstream{},
	}

	svc := newFullTestLBService(
		&mockServerRepo{server: srv},
		upstreamRepo,
		&fullMockBackendRepo{},
		nil,
	)

	err := svc.TriggerHealthCheck(context.Background(), srv.ID, srv.TeamID, "nonexistent")
	assert.Error(t, err)
}

// =============================================================================
// isLoadBalancer tests (tested via ListUpstreams)
// =============================================================================

func TestIsLoadBalancer_Various(t *testing.T) {
	tests := []struct {
		name   string
		server *models.Server
		want   bool
	}{
		{"load_balancer type", lbServer(), true},
		{"app type", nonLBServer(), false},
		{"nil type", func() *models.Server {
			s := &models.Server{Name: "nil-type"}
			s.ID = "01HTEST0000000000000001"
			return s
		}(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newFullTestLBService(
				&mockServerRepo{server: tt.server},
				&fullMockUpstreamRepo{},
				&fullMockBackendRepo{},
				nil,
			)
			got := svc.isLoadBalancer(tt.server)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// getUpstreamServerIP tests
// =============================================================================

func TestGetUpstreamServerIP(t *testing.T) {
	svc := newFullTestLBService(
		&mockServerRepo{},
		&fullMockUpstreamRepo{},
		&fullMockBackendRepo{},
		nil,
	)

	t.Run("with server IP", func(t *testing.T) {
		ip := "10.0.0.50"
		upstream := &models.LoadBalancerUpstream{
			Server: &models.Server{PublicIPv4: &ip},
		}
		assert.Equal(t, "10.0.0.50", svc.getUpstreamServerIP(upstream))
	})

	t.Run("nil server", func(t *testing.T) {
		upstream := &models.LoadBalancerUpstream{Server: nil}
		assert.Equal(t, "", svc.getUpstreamServerIP(upstream))
	})

	t.Run("nil IP", func(t *testing.T) {
		upstream := &models.LoadBalancerUpstream{
			Server: &models.Server{PublicIPv4: nil},
		}
		assert.Equal(t, "", svc.getUpstreamServerIP(upstream))
	})
}
