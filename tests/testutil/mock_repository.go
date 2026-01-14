package testutil

import (
	"context"
	"time"

	"gorm.io/gorm"

	dbmodels "github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// MockRepository is a test mock implementing the Repository interface.
type MockRepository struct {
	// Storage for test data
	Servers       map[string]*models.Server
	Crons         map[string]*models.Cron
	Daemons       map[string]*models.Daemon
	FirewallRules map[string]*models.FirewallRule
	SshKeys       map[string]*models.SshKey
	Services      map[string]*models.InstalledService
	Tasks         map[string]*models.Task
	Metrics       map[uint64]*models.Metric
	Databases     map[string]*dbmodels.Database
	DatabaseUsers map[string]*dbmodels.DatabaseUser

	// Error injection for testing error handling
	Errors map[string]error
}

// NewMockRepository creates a new mock repository with empty storage.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		Servers:       make(map[string]*models.Server),
		Crons:         make(map[string]*models.Cron),
		Daemons:       make(map[string]*models.Daemon),
		FirewallRules: make(map[string]*models.FirewallRule),
		SshKeys:       make(map[string]*models.SshKey),
		Services:      make(map[string]*models.InstalledService),
		Tasks:         make(map[string]*models.Task),
		Metrics:       make(map[uint64]*models.Metric),
		Databases:     make(map[string]*dbmodels.Database),
		DatabaseUsers: make(map[string]*dbmodels.DatabaseUser),
		Errors:        make(map[string]error),
	}
}

// SetError configures an error to be returned for a specific method.
func (m *MockRepository) SetError(method string, err error) {
	m.Errors[method] = err
}

// getError returns a configured error if one exists.
func (m *MockRepository) getError(method string) error {
	return m.Errors[method]
}

// ---- ServerRepository ----

func (m *MockRepository) CreateServer(ctx context.Context, server *models.Server) error {
	if err := m.getError("CreateServer"); err != nil {
		return err
	}
	m.Servers[server.ID] = server
	return nil
}

func (m *MockRepository) FindServerByID(ctx context.Context, id string) (*models.Server, error) {
	if err := m.getError("FindServerByID"); err != nil {
		return nil, err
	}
	server, ok := m.Servers[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return server, nil
}

func (m *MockRepository) FindServerByIDAndTeam(ctx context.Context, id, teamID string) (*models.Server, error) {
	if err := m.getError("FindServerByIDAndTeam"); err != nil {
		return nil, err
	}
	server, ok := m.Servers[id]
	if !ok || server.TeamID != teamID {
		return nil, gorm.ErrRecordNotFound
	}
	return server, nil
}

func (m *MockRepository) FindServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	return m.FindServerByIDAndTeam(ctx, id, teamID)
}

func (m *MockRepository) FindAllServersByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	if err := m.getError("FindAllServersByTeam"); err != nil {
		return nil, err
	}
	var servers []models.Server
	for _, s := range m.Servers {
		if s.TeamID == teamID {
			servers = append(servers, *s)
		}
	}
	return servers, nil
}

func (m *MockRepository) FindAllServersByTeamPaginated(ctx context.Context, teamID string, limit, offset int) ([]models.Server, int64, error) {
	servers, err := m.FindAllServersByTeam(ctx, teamID)
	return servers, int64(len(servers)), err
}

func (m *MockRepository) FindArchivedServersByTeam(ctx context.Context, teamID string) ([]models.Server, error) {
	if err := m.getError("FindArchivedServersByTeam"); err != nil {
		return nil, err
	}
	var servers []models.Server
	for _, s := range m.Servers {
		if s.TeamID == teamID && s.ArchivedAt != nil {
			servers = append(servers, *s)
		}
	}
	return servers, nil
}

func (m *MockRepository) UpdateServer(ctx context.Context, server *models.Server) error {
	if err := m.getError("UpdateServer"); err != nil {
		return err
	}
	m.Servers[server.ID] = server
	return nil
}

func (m *MockRepository) UpdateServerStatus(ctx context.Context, id string, status enums.ServerStatus) error {
	if err := m.getError("UpdateServerStatus"); err != nil {
		return err
	}
	if server, ok := m.Servers[id]; ok {
		server.Status = status
	}
	return nil
}

func (m *MockRepository) UpdateServerProgress(ctx context.Context, id string, progress int, step string) error {
	if err := m.getError("UpdateServerProgress"); err != nil {
		return err
	}
	if server, ok := m.Servers[id]; ok {
		server.Progress = progress
		server.ProgressStep = &step
	}
	return nil
}

func (m *MockRepository) UpdateServerFields(ctx context.Context, id string, fields map[string]interface{}) error {
	if err := m.getError("UpdateServerFields"); err != nil {
		return err
	}
	return nil
}

func (m *MockRepository) ArchiveServer(ctx context.Context, id string) error {
	if err := m.getError("ArchiveServer"); err != nil {
		return err
	}
	if server, ok := m.Servers[id]; ok {
		now := time.Now()
		server.ArchivedAt = &now
	}
	return nil
}

func (m *MockRepository) UnarchiveServer(ctx context.Context, id string) error {
	if err := m.getError("UnarchiveServer"); err != nil {
		return err
	}
	if server, ok := m.Servers[id]; ok {
		server.ArchivedAt = nil
	}
	return nil
}

func (m *MockRepository) DeleteServer(ctx context.Context, id string) error {
	if err := m.getError("DeleteServer"); err != nil {
		return err
	}
	delete(m.Servers, id)
	return nil
}

func (m *MockRepository) CountServersByTeam(ctx context.Context, teamID string) (int64, error) {
	if err := m.getError("CountServersByTeam"); err != nil {
		return 0, err
	}
	var count int64
	for _, s := range m.Servers {
		if s.TeamID == teamID {
			count++
		}
	}
	return count, nil
}

func (m *MockRepository) ServerHasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	if err := m.getError("ServerHasLaunchAgent"); err != nil {
		return false, err
	}
	return false, nil
}

// ---- CronRepository ----

func (m *MockRepository) CreateCron(ctx context.Context, cron *models.Cron) error {
	if err := m.getError("CreateCron"); err != nil {
		return err
	}
	m.Crons[cron.ID] = cron
	return nil
}

func (m *MockRepository) FindCronByID(ctx context.Context, id string) (*models.Cron, error) {
	if err := m.getError("FindCronByID"); err != nil {
		return nil, err
	}
	cron, ok := m.Crons[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return cron, nil
}

func (m *MockRepository) FindCronByIDWithServer(ctx context.Context, id string) (*models.Cron, error) {
	if err := m.getError("FindCronByIDWithServer"); err != nil {
		return nil, err
	}
	cron, ok := m.Crons[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	// Preload server
	if server, ok := m.Servers[cron.ServerID]; ok {
		cron.Server = server
	}
	return cron, nil
}

func (m *MockRepository) FindCronByIDAndServer(ctx context.Context, id, serverID string) (*models.Cron, error) {
	if err := m.getError("FindCronByIDAndServer"); err != nil {
		return nil, err
	}
	cron, ok := m.Crons[id]
	if !ok || cron.ServerID != serverID {
		return nil, gorm.ErrRecordNotFound
	}
	return cron, nil
}

func (m *MockRepository) FindCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	if err := m.getError("FindCronsByServer"); err != nil {
		return nil, err
	}
	var crons []models.Cron
	for _, c := range m.Crons {
		if c.ServerID == serverID {
			crons = append(crons, *c)
		}
	}
	return crons, nil
}

func (m *MockRepository) FindVisibleCronsByServer(ctx context.Context, serverID string) ([]models.Cron, error) {
	if err := m.getError("FindVisibleCronsByServer"); err != nil {
		return nil, err
	}
	var crons []models.Cron
	for _, c := range m.Crons {
		if c.ServerID == serverID && !c.Hidden {
			crons = append(crons, *c)
		}
	}
	return crons, nil
}

func (m *MockRepository) UpdateCron(ctx context.Context, cron *models.Cron) error {
	if err := m.getError("UpdateCron"); err != nil {
		return err
	}
	m.Crons[cron.ID] = cron
	return nil
}

func (m *MockRepository) MarkCronInstalled(ctx context.Context, id string) error {
	if err := m.getError("MarkCronInstalled"); err != nil {
		return err
	}
	if cron, ok := m.Crons[id]; ok {
		now := time.Now()
		cron.InstalledAt = &now
	}
	return nil
}

func (m *MockRepository) DeleteCron(ctx context.Context, id string) error {
	if err := m.getError("DeleteCron"); err != nil {
		return err
	}
	delete(m.Crons, id)
	return nil
}

// ---- DaemonRepository ----

func (m *MockRepository) CreateDaemon(ctx context.Context, daemon *models.Daemon) error {
	if err := m.getError("CreateDaemon"); err != nil {
		return err
	}
	m.Daemons[daemon.ID] = daemon
	return nil
}

func (m *MockRepository) FindDaemonByID(ctx context.Context, id string) (*models.Daemon, error) {
	if err := m.getError("FindDaemonByID"); err != nil {
		return nil, err
	}
	daemon, ok := m.Daemons[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return daemon, nil
}

func (m *MockRepository) FindDaemonByIDWithServer(ctx context.Context, id string) (*models.Daemon, error) {
	if err := m.getError("FindDaemonByIDWithServer"); err != nil {
		return nil, err
	}
	daemon, ok := m.Daemons[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	if server, ok := m.Servers[daemon.ServerID]; ok {
		daemon.Server = server
	}
	return daemon, nil
}

func (m *MockRepository) FindDaemonByIDAndServer(ctx context.Context, id, serverID string) (*models.Daemon, error) {
	if err := m.getError("FindDaemonByIDAndServer"); err != nil {
		return nil, err
	}
	daemon, ok := m.Daemons[id]
	if !ok || daemon.ServerID != serverID {
		return nil, gorm.ErrRecordNotFound
	}
	return daemon, nil
}

func (m *MockRepository) FindDaemonsByServer(ctx context.Context, serverID string) ([]models.Daemon, error) {
	if err := m.getError("FindDaemonsByServer"); err != nil {
		return nil, err
	}
	var daemons []models.Daemon
	for _, d := range m.Daemons {
		if d.ServerID == serverID {
			daemons = append(daemons, *d)
		}
	}
	return daemons, nil
}

func (m *MockRepository) UpdateDaemon(ctx context.Context, daemon *models.Daemon) error {
	if err := m.getError("UpdateDaemon"); err != nil {
		return err
	}
	m.Daemons[daemon.ID] = daemon
	return nil
}

func (m *MockRepository) UpdateDaemonStatus(ctx context.Context, id string, running bool) error {
	if err := m.getError("UpdateDaemonStatus"); err != nil {
		return err
	}
	return nil
}

func (m *MockRepository) MarkDaemonInstalled(ctx context.Context, id string) error {
	if err := m.getError("MarkDaemonInstalled"); err != nil {
		return err
	}
	if daemon, ok := m.Daemons[id]; ok {
		now := time.Now()
		daemon.InstalledAt = &now
	}
	return nil
}

func (m *MockRepository) DeleteDaemon(ctx context.Context, id string) error {
	if err := m.getError("DeleteDaemon"); err != nil {
		return err
	}
	delete(m.Daemons, id)
	return nil
}

// ---- FirewallRuleRepository ----

func (m *MockRepository) CreateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	if err := m.getError("CreateFirewallRule"); err != nil {
		return err
	}
	m.FirewallRules[rule.ID] = rule
	return nil
}

func (m *MockRepository) FindFirewallRuleByID(ctx context.Context, id string) (*models.FirewallRule, error) {
	if err := m.getError("FindFirewallRuleByID"); err != nil {
		return nil, err
	}
	rule, ok := m.FirewallRules[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return rule, nil
}

func (m *MockRepository) FindFirewallRuleByIDWithServer(ctx context.Context, id string) (*models.FirewallRule, error) {
	if err := m.getError("FindFirewallRuleByIDWithServer"); err != nil {
		return nil, err
	}
	rule, ok := m.FirewallRules[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	if server, ok := m.Servers[rule.ServerID]; ok {
		rule.Server = server
	}
	return rule, nil
}

func (m *MockRepository) FindFirewallRuleByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error) {
	if err := m.getError("FindFirewallRuleByIDAndServer"); err != nil {
		return nil, err
	}
	rule, ok := m.FirewallRules[id]
	if !ok || rule.ServerID != serverID {
		return nil, gorm.ErrRecordNotFound
	}
	return rule, nil
}

func (m *MockRepository) FindFirewallRulesByServer(ctx context.Context, serverID string) ([]models.FirewallRule, error) {
	if err := m.getError("FindFirewallRulesByServer"); err != nil {
		return nil, err
	}
	var rules []models.FirewallRule
	for _, r := range m.FirewallRules {
		if r.ServerID == serverID {
			rules = append(rules, *r)
		}
	}
	return rules, nil
}

func (m *MockRepository) UpdateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	if err := m.getError("UpdateFirewallRule"); err != nil {
		return err
	}
	m.FirewallRules[rule.ID] = rule
	return nil
}

func (m *MockRepository) MarkFirewallRuleInstalled(ctx context.Context, id string) error {
	if err := m.getError("MarkFirewallRuleInstalled"); err != nil {
		return err
	}
	if rule, ok := m.FirewallRules[id]; ok {
		now := time.Now()
		rule.InstalledAt = &now
	}
	return nil
}

func (m *MockRepository) MarkFirewallRuleFailed(ctx context.Context, id string) error {
	if err := m.getError("MarkFirewallRuleFailed"); err != nil {
		return err
	}
	return nil
}

func (m *MockRepository) DeleteFirewallRule(ctx context.Context, id string) error {
	if err := m.getError("DeleteFirewallRule"); err != nil {
		return err
	}
	delete(m.FirewallRules, id)
	return nil
}

// ---- SshKeyRepository ----

func (m *MockRepository) CreateSshKey(ctx context.Context, key *models.SshKey) error {
	if err := m.getError("CreateSshKey"); err != nil {
		return err
	}
	m.SshKeys[key.ID] = key
	return nil
}

func (m *MockRepository) FindSshKeyByID(ctx context.Context, id string) (*models.SshKey, error) {
	if err := m.getError("FindSshKeyByID"); err != nil {
		return nil, err
	}
	key, ok := m.SshKeys[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return key, nil
}

func (m *MockRepository) FindSshKeysByTeam(ctx context.Context, teamID string) ([]models.SshKey, error) {
	if err := m.getError("FindSshKeysByTeam"); err != nil {
		return nil, err
	}
	var keys []models.SshKey
	for _, k := range m.SshKeys {
		keys = append(keys, *k)
	}
	return keys, nil
}

func (m *MockRepository) FindSshKeysByServer(ctx context.Context, serverID string) ([]models.SshKey, error) {
	if err := m.getError("FindSshKeysByServer"); err != nil {
		return nil, err
	}
	var keys []models.SshKey
	for _, k := range m.SshKeys {
		keys = append(keys, *k)
	}
	return keys, nil
}

func (m *MockRepository) FindGlobalSshKeys(ctx context.Context) ([]models.SshKey, error) {
	if err := m.getError("FindGlobalSshKeys"); err != nil {
		return nil, err
	}
	return []models.SshKey{}, nil
}

func (m *MockRepository) UpdateSshKey(ctx context.Context, key *models.SshKey) error {
	if err := m.getError("UpdateSshKey"); err != nil {
		return err
	}
	m.SshKeys[key.ID] = key
	return nil
}

func (m *MockRepository) DeleteSshKey(ctx context.Context, id string) error {
	if err := m.getError("DeleteSshKey"); err != nil {
		return err
	}
	delete(m.SshKeys, id)
	return nil
}

func (m *MockRepository) AttachSshKeyToServer(ctx context.Context, serverID, sshKeyID string) error {
	if err := m.getError("AttachSshKeyToServer"); err != nil {
		return err
	}
	return nil
}

func (m *MockRepository) DetachSshKeyFromServer(ctx context.Context, serverID, sshKeyID string) error {
	if err := m.getError("DetachSshKeyFromServer"); err != nil {
		return err
	}
	return nil
}

func (m *MockRepository) IsSshKeyAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error) {
	if err := m.getError("IsSshKeyAttachedToServer"); err != nil {
		return false, err
	}
	return false, nil
}

// ---- ServiceRepository ----

func (m *MockRepository) CreateService(ctx context.Context, service *models.InstalledService) error {
	if err := m.getError("CreateService"); err != nil {
		return err
	}
	m.Services[service.ID] = service
	return nil
}

func (m *MockRepository) FindServiceByID(ctx context.Context, id string) (*models.InstalledService, error) {
	if err := m.getError("FindServiceByID"); err != nil {
		return nil, err
	}
	service, ok := m.Services[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return service, nil
}

func (m *MockRepository) FindServicesByServer(ctx context.Context, serverID string) ([]models.InstalledService, error) {
	if err := m.getError("FindServicesByServer"); err != nil {
		return nil, err
	}
	var services []models.InstalledService
	for _, s := range m.Services {
		if s.ServerID == serverID {
			services = append(services, *s)
		}
	}
	return services, nil
}

func (m *MockRepository) FindServicesByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) ([]models.InstalledService, error) {
	if err := m.getError("FindServicesByServerAndType"); err != nil {
		return nil, err
	}
	var services []models.InstalledService
	for _, s := range m.Services {
		if s.ServerID == serverID && s.Type == serviceType {
			services = append(services, *s)
		}
	}
	return services, nil
}

func (m *MockRepository) FindServiceByServerAndType(ctx context.Context, serverID string, serviceType enums.ServiceType) (*models.InstalledService, error) {
	if err := m.getError("FindServiceByServerAndType"); err != nil {
		return nil, err
	}
	for _, s := range m.Services {
		if s.ServerID == serverID && s.Type == serviceType {
			return s, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockRepository) FindServiceByServerAndSoftware(ctx context.Context, serverID string, software enums.Software) (*models.InstalledService, error) {
	if err := m.getError("FindServiceByServerAndSoftware"); err != nil {
		return nil, err
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockRepository) FindDatabaseService(ctx context.Context, serverID string) (*models.InstalledService, error) {
	if err := m.getError("FindDatabaseService"); err != nil {
		return nil, err
	}
	for _, s := range m.Services {
		if s.ServerID == serverID && (s.Type == enums.ServiceTypeMySql || s.Type == enums.ServiceTypePostgreSql) {
			return s, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockRepository) UpdateService(ctx context.Context, service *models.InstalledService) error {
	if err := m.getError("UpdateService"); err != nil {
		return err
	}
	m.Services[service.ID] = service
	return nil
}

func (m *MockRepository) UpdateServiceStatus(ctx context.Context, id string, status enums.ServiceStatus) error {
	if err := m.getError("UpdateServiceStatus"); err != nil {
		return err
	}
	if service, ok := m.Services[id]; ok {
		service.Status = status
	}
	return nil
}

func (m *MockRepository) UpdateServiceWithTypeData(ctx context.Context, id string, status enums.ServiceStatus, typeData map[string]any) error {
	if err := m.getError("UpdateServiceWithTypeData"); err != nil {
		return err
	}
	if service, ok := m.Services[id]; ok {
		service.Status = status
		if service.TypeData == nil {
			service.TypeData = make(map[string]any)
		}
		for k, v := range typeData {
			service.TypeData[k] = v
		}
	}
	return nil
}

func (m *MockRepository) DeleteService(ctx context.Context, id string) error {
	if err := m.getError("DeleteService"); err != nil {
		return err
	}
	delete(m.Services, id)
	return nil
}

// ---- TaskRepository ----

func (m *MockRepository) CreateTask(ctx context.Context, task *models.Task) error {
	if err := m.getError("CreateTask"); err != nil {
		return err
	}
	m.Tasks[task.ID] = task
	return nil
}

func (m *MockRepository) FindTaskByID(ctx context.Context, id string) (*models.Task, error) {
	if err := m.getError("FindTaskByID"); err != nil {
		return nil, err
	}
	task, ok := m.Tasks[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return task, nil
}

func (m *MockRepository) FindTasksByServer(ctx context.Context, serverID string, limit int) ([]models.Task, error) {
	if err := m.getError("FindTasksByServer"); err != nil {
		return nil, err
	}
	var tasks []models.Task
	for _, t := range m.Tasks {
		if t.ServerID == serverID {
			tasks = append(tasks, *t)
		}
	}
	return tasks, nil
}

func (m *MockRepository) FindLatestTaskByServer(ctx context.Context, serverID string) (*models.Task, error) {
	if err := m.getError("FindLatestTaskByServer"); err != nil {
		return nil, err
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockRepository) UpdateTask(ctx context.Context, task *models.Task) error {
	if err := m.getError("UpdateTask"); err != nil {
		return err
	}
	m.Tasks[task.ID] = task
	return nil
}

// ---- MetricRepository ----

func (m *MockRepository) CreateMetric(ctx context.Context, metric *models.Metric) error {
	if err := m.getError("CreateMetric"); err != nil {
		return err
	}
	m.Metrics[metric.ID] = metric
	return nil
}

func (m *MockRepository) FindMetricsByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error) {
	if err := m.getError("FindMetricsByServer"); err != nil {
		return nil, err
	}
	var metrics []models.Metric
	for _, met := range m.Metrics {
		if met.ServerID == serverID {
			metrics = append(metrics, *met)
		}
	}
	return metrics, nil
}

func (m *MockRepository) FindLatestMetricByServer(ctx context.Context, serverID string) (*models.Metric, error) {
	if err := m.getError("FindLatestMetricByServer"); err != nil {
		return nil, err
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MockRepository) DeleteOldMetrics(ctx context.Context, serverID string, before time.Time) error {
	if err := m.getError("DeleteOldMetrics"); err != nil {
		return err
	}
	return nil
}

// ---- ServerProviderRepository ----

func (m *MockRepository) FindServerProvidersByTeam(ctx context.Context, teamID string) ([]models.ServerProvider, error) {
	return nil, nil
}

func (m *MockRepository) FindServerProviderByID(ctx context.Context, id string) (*models.ServerProvider, error) {
	return nil, gorm.ErrRecordNotFound
}

func (m *MockRepository) CreateServerProvider(ctx context.Context, provider *models.ServerProvider) error {
	return nil
}

func (m *MockRepository) DeleteServerProvider(ctx context.Context, id string) error {
	return nil
}

// ---- DatabaseRepository ----

func (m *MockRepository) FindDatabasesByServer(ctx context.Context, serverID string) ([]dbmodels.Database, error) {
	if err := m.getError("FindDatabasesByServer"); err != nil {
		return nil, err
	}
	var dbs []dbmodels.Database
	for _, db := range m.Databases {
		if db.ServerID == serverID {
			dbs = append(dbs, *db)
		}
	}
	return dbs, nil
}

func (m *MockRepository) FindDatabaseByID(ctx context.Context, id string) (*dbmodels.Database, error) {
	if err := m.getError("FindDatabaseByID"); err != nil {
		return nil, err
	}
	db, ok := m.Databases[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return db, nil
}

func (m *MockRepository) FindDatabaseByIDWithServer(ctx context.Context, id string) (*dbmodels.Database, error) {
	return m.FindDatabaseByID(ctx, id)
}

func (m *MockRepository) CreateDatabase(ctx context.Context, db *dbmodels.Database) error {
	if err := m.getError("CreateDatabase"); err != nil {
		return err
	}
	m.Databases[db.ID] = db
	return nil
}

func (m *MockRepository) DeleteDatabase(ctx context.Context, id string) error {
	if err := m.getError("DeleteDatabase"); err != nil {
		return err
	}
	delete(m.Databases, id)
	return nil
}

// ---- DatabaseUserRepository ----

func (m *MockRepository) FindDatabaseUsersByServer(ctx context.Context, serverID string) ([]dbmodels.DatabaseUser, error) {
	if err := m.getError("FindDatabaseUsersByServer"); err != nil {
		return nil, err
	}
	var users []dbmodels.DatabaseUser
	for _, u := range m.DatabaseUsers {
		if u.ServerID == serverID {
			users = append(users, *u)
		}
	}
	return users, nil
}

func (m *MockRepository) FindDatabaseUserByID(ctx context.Context, id string) (*dbmodels.DatabaseUser, error) {
	if err := m.getError("FindDatabaseUserByID"); err != nil {
		return nil, err
	}
	user, ok := m.DatabaseUsers[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (m *MockRepository) FindDatabaseUserByIDWithServer(ctx context.Context, id string) (*dbmodels.DatabaseUser, error) {
	return m.FindDatabaseUserByID(ctx, id)
}

func (m *MockRepository) CreateDatabaseUser(ctx context.Context, user *dbmodels.DatabaseUser) error {
	if err := m.getError("CreateDatabaseUser"); err != nil {
		return err
	}
	m.DatabaseUsers[user.ID] = user
	return nil
}

func (m *MockRepository) DeleteDatabaseUser(ctx context.Context, id string) error {
	if err := m.getError("DeleteDatabaseUser"); err != nil {
		return err
	}
	delete(m.DatabaseUsers, id)
	return nil
}

// ---- Transaction ----

func (m *MockRepository) Transaction(ctx context.Context, fn func(tx contracts.Repository) error) error {
	return fn(m)
}

func (m *MockRepository) DB() *gorm.DB {
	return nil
}

// Ensure MockRepository implements Repository interface
var _ contracts.Repository = (*MockRepository)(nil)
