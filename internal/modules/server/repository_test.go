package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(AllModels()...)
	require.NoError(t, err)

	return NewRepository(db), db
}

func createTestServer(t *testing.T, repo *Repository, teamID, name string) *Server {
	server := &Server{
		TeamID:          teamID,
		UserID:          "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:            name,
		Provider:        ProviderDigitalOcean,
		Type:            ServerTypePhp,
		OperatingSystem: OSUbuntu24,
	}
	err := repo.CreateServer(context.Background(), server)
	require.NoError(t, err)
	return server
}

func createTestService(t *testing.T, repo *Repository, serverID string, software Software) *InstalledService {
	service := &InstalledService{
		ServerID: serverID,
		Type:     software.GetServiceType(),
		Name:     software.Label(),
		Software: &software,
	}
	err := repo.CreateService(context.Background(), service)
	require.NoError(t, err)
	return service
}

func TestNewRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := NewRepository(db)

	assert.NotNil(t, repo)
}

func TestRepository_CreateServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := &Server{
		TeamID:          "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:          "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:            "test-server",
		Provider:        ProviderDigitalOcean,
		Type:            ServerTypePhp,
		OperatingSystem: OSUbuntu24,
	}

	err := repo.CreateServer(ctx, server)
	require.NoError(t, err)

	assert.NotEmpty(t, server.ID)
	assert.Equal(t, ServerStatusNew, server.Status)
	assert.NotZero(t, server.CreatedAt)
}

func TestRepository_FindServerByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds existing server", func(t *testing.T) {
		created := createTestServer(t, repo, "team1", "test-server")

		found, err := repo.FindServerByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Name, found.Name)
	})

	t.Run("returns error for non-existent server", func(t *testing.T) {
		_, err := repo.FindServerByID(ctx, "non_existent_id")
		assert.ErrorIs(t, err, ErrServerNotFound)
	})
}

func TestRepository_FindServerByIDAndTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds server by ID and team", func(t *testing.T) {
		teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestServer(t, repo, teamID, "test-server")

		found, err := repo.FindServerByIDAndTeam(ctx, created.ID, teamID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for wrong team", func(t *testing.T) {
		created := createTestServer(t, repo, "team1", "test-server-2")

		_, err := repo.FindServerByIDAndTeam(ctx, created.ID, "different_team")
		assert.ErrorIs(t, err, ErrServerNotFound)
	})
}

func TestRepository_FindServerWithRelations(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	server := createTestServer(t, repo, teamID, "test-server")

	// Create related entities
	createTestService(t, repo, server.ID, SoftwarePhp84)

	found, err := repo.FindServerWithRelations(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Equal(t, server.ID, found.ID)
	assert.Len(t, found.Services, 1)
}

func TestRepository_FindAllServersByTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestServer(t, repo, teamID, "server1")
	createTestServer(t, repo, teamID, "server2")
	createTestServer(t, repo, "other_team", "server3")

	servers, err := repo.FindAllServersByTeam(ctx, teamID)
	require.NoError(t, err)

	assert.Len(t, servers, 2)
}

func TestRepository_FindAllServersByTeamPaginated(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	for i := 0; i < 5; i++ {
		createTestServer(t, repo, teamID, "server-"+string(rune('0'+i)))
	}

	servers, total, err := repo.FindAllServersByTeamPaginated(ctx, teamID, 2, 0)
	require.NoError(t, err)

	assert.Len(t, servers, 2)
	assert.Equal(t, int64(5), total)

	// Second page
	servers2, total2, err := repo.FindAllServersByTeamPaginated(ctx, teamID, 2, 2)
	require.NoError(t, err)

	assert.Len(t, servers2, 2)
	assert.Equal(t, int64(5), total2)
}

func TestRepository_UpdateServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "original-name")
	server.Name = "updated-name"

	err := repo.UpdateServer(ctx, server)
	require.NoError(t, err)

	found, err := repo.FindServerByID(ctx, server.ID)
	require.NoError(t, err)

	assert.Equal(t, "updated-name", found.Name)
}

func TestRepository_UpdateServerStatus(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	err := repo.UpdateServerStatus(ctx, server.ID, ServerStatusRunning)
	require.NoError(t, err)

	found, err := repo.FindServerByID(ctx, server.ID)
	require.NoError(t, err)

	assert.Equal(t, ServerStatusRunning, found.Status)
}

func TestRepository_UpdateServerProgress(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	err := repo.UpdateServerProgress(ctx, server.ID, 50, "Installing PHP")
	require.NoError(t, err)

	found, err := repo.FindServerByID(ctx, server.ID)
	require.NoError(t, err)

	assert.Equal(t, 50, *found.Progress)
	assert.NotNil(t, found.ProgressStep)
	assert.Equal(t, "Installing PHP", *found.ProgressStep)
}

func TestRepository_ArchiveServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	err := repo.ArchiveServer(ctx, server.ID)
	require.NoError(t, err)

	found, err := repo.FindServerByID(ctx, server.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.ArchivedAt)
	assert.Equal(t, ServerStatusArchived, found.Status)
}

func TestRepository_UnarchiveServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	err := repo.ArchiveServer(ctx, server.ID)
	require.NoError(t, err)

	err = repo.UnarchiveServer(ctx, server.ID)
	require.NoError(t, err)

	found, err := repo.FindServerByID(ctx, server.ID)
	require.NoError(t, err)

	assert.Nil(t, found.ArchivedAt)
	assert.Equal(t, ServerStatusStopped, found.Status)
}

func TestRepository_DeleteServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	err := repo.DeleteServer(ctx, server.ID)
	require.NoError(t, err)

	_, err = repo.FindServerByID(ctx, server.ID)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

// Service tests

func TestRepository_CreateService(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	software := SoftwarePhp84
	service := &InstalledService{
		ServerID: server.ID,
		Type:     ServiceTypePhp,
		Name:     "PHP 8.4",
		Software: &software,
	}

	err := repo.CreateService(ctx, service)
	require.NoError(t, err)

	assert.NotEmpty(t, service.ID)
	assert.Equal(t, ServiceStatusPending, service.Status)
}

func TestRepository_FindServiceByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	service := createTestService(t, repo, server.ID, SoftwarePhp84)

	found, err := repo.FindServiceByID(ctx, service.ID)
	require.NoError(t, err)

	assert.Equal(t, service.ID, found.ID)
}

func TestRepository_FindServicesByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	createTestService(t, repo, server.ID, SoftwarePhp84)
	createTestService(t, repo, server.ID, SoftwareMySql80)

	services, err := repo.FindServicesByServer(ctx, server.ID)
	require.NoError(t, err)

	assert.Len(t, services, 2)
}

func TestRepository_UpdateServiceStatus(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	service := createTestService(t, repo, server.ID, SoftwarePhp84)

	err := repo.UpdateServiceStatus(ctx, service.ID, ServiceStatusRunning)
	require.NoError(t, err)

	found, err := repo.FindServiceByID(ctx, service.ID)
	require.NoError(t, err)

	assert.Equal(t, ServiceStatusRunning, found.Status)
}

func TestRepository_DeleteService(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	service := createTestService(t, repo, server.ID, SoftwarePhp84)

	err := repo.DeleteService(ctx, service.ID)
	require.NoError(t, err)

	_, err = repo.FindServiceByID(ctx, service.ID)
	assert.ErrorIs(t, err, ErrServiceNotFound)
}

// Firewall rule tests

func TestRepository_CreateFirewallRule(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	port := "22"
	rule := &FirewallRule{
		ServerID: server.ID,
		Name:     "SSH",
		Port:     &port,
		Action:   RuleActionAllow,
	}

	err := repo.CreateFirewallRule(ctx, rule)
	require.NoError(t, err)

	assert.NotEmpty(t, rule.ID)
}

func TestRepository_FindFirewallRulesByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	port22 := "22"
	port80 := "80"
	rule1 := &FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port22, Action: RuleActionAllow}
	rule2 := &FirewallRule{ServerID: server.ID, Name: "HTTP", Port: &port80, Action: RuleActionAllow}
	repo.CreateFirewallRule(ctx, rule1)
	repo.CreateFirewallRule(ctx, rule2)

	rules, err := repo.FindFirewallRulesByServer(ctx, server.ID)
	require.NoError(t, err)

	assert.Len(t, rules, 2)
}

func TestRepository_MarkFirewallRuleInstalled(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	port := "22"
	rule := &FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port, Action: RuleActionAllow}
	repo.CreateFirewallRule(ctx, rule)

	err := repo.MarkFirewallRuleInstalled(ctx, rule.ID)
	require.NoError(t, err)

	found, err := repo.FindFirewallRuleByID(ctx, rule.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_DeleteFirewallRule(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	port := "22"
	rule := &FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port, Action: RuleActionAllow}
	repo.CreateFirewallRule(ctx, rule)

	err := repo.DeleteFirewallRule(ctx, rule.ID)
	require.NoError(t, err)

	_, err = repo.FindFirewallRuleByID(ctx, rule.ID)
	assert.ErrorIs(t, err, ErrFirewallRuleNotFound)
}

// Cron tests

func TestRepository_CreateCron(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	cron := &Cron{
		ServerID:   server.ID,
		User:       "root",
		Expression: "*/5 * * * *",
		Command:    "php artisan schedule:run",
	}

	err := repo.CreateCron(ctx, cron)
	require.NoError(t, err)

	assert.NotEmpty(t, cron.ID)
}

func TestRepository_FindCronsByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	cron1 := &Cron{ServerID: server.ID, User: "root", Expression: "*/5 * * * *", Command: "cmd1"}
	cron2 := &Cron{ServerID: server.ID, User: "root", Expression: "0 * * * *", Command: "cmd2"}
	repo.CreateCron(ctx, cron1)
	repo.CreateCron(ctx, cron2)

	crons, err := repo.FindCronsByServer(ctx, server.ID)
	require.NoError(t, err)

	assert.Len(t, crons, 2)
}

func TestRepository_DeleteCron(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	cron := &Cron{ServerID: server.ID, User: "root", Expression: "*/5 * * * *", Command: "cmd1"}
	repo.CreateCron(ctx, cron)

	err := repo.DeleteCron(ctx, cron.ID)
	require.NoError(t, err)

	_, err = repo.FindCronByID(ctx, cron.ID)
	assert.ErrorIs(t, err, ErrCronNotFound)
}

// Daemon tests

func TestRepository_CreateDaemon(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	dir := "/var/www/app"
	daemon := &Daemon{
		ServerID:  server.ID,
		Command:   "php artisan queue:work",
		User:      "deploy",
		Directory: &dir,
		Processes: 2,
	}

	err := repo.CreateDaemon(ctx, daemon)
	require.NoError(t, err)

	assert.NotEmpty(t, daemon.ID)
}

func TestRepository_FindDaemonsByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	dir := "/tmp"
	daemon1 := &Daemon{ServerID: server.ID, Command: "cmd1", User: "root", Directory: &dir}
	daemon2 := &Daemon{ServerID: server.ID, Command: "cmd2", User: "root", Directory: &dir}
	repo.CreateDaemon(ctx, daemon1)
	repo.CreateDaemon(ctx, daemon2)

	daemons, err := repo.FindDaemonsByServer(ctx, server.ID)
	require.NoError(t, err)

	assert.Len(t, daemons, 2)
}

func TestRepository_UpdateDaemonStatus(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	dir := "/tmp"
	daemon := &Daemon{ServerID: server.ID, Command: "cmd", User: "root", Directory: &dir}
	repo.CreateDaemon(ctx, daemon)

	err := repo.UpdateDaemonStatus(ctx, daemon.ID, true)
	require.NoError(t, err)

	found, err := repo.FindDaemonByID(ctx, daemon.ID)
	require.NoError(t, err)

	assert.True(t, found.Running)
	assert.NotNil(t, found.LastStatusCheck)
}

func TestRepository_DeleteDaemon(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	dir := "/tmp"
	daemon := &Daemon{ServerID: server.ID, Command: "cmd", User: "root", Directory: &dir}
	repo.CreateDaemon(ctx, daemon)

	err := repo.DeleteDaemon(ctx, daemon.ID)
	require.NoError(t, err)

	_, err = repo.FindDaemonByID(ctx, daemon.ID)
	assert.ErrorIs(t, err, ErrDaemonNotFound)
}

// SSH Key tests

func TestRepository_CreateSshKey(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	key := &SshKey{
		TeamID:    &teamID,
		UserID:    &userID,
		Name:      "My SSH Key",
		PublicKey: "ssh-rsa AAAAB3... user@example.com",
	}

	err := repo.CreateSshKey(ctx, key)
	require.NoError(t, err)

	assert.NotEmpty(t, key.ID)
}

func TestRepository_FindSshKeysByTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	key1 := &SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 1", PublicKey: "ssh-rsa AAA1"}
	key2 := &SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 2", PublicKey: "ssh-rsa AAA2"}
	repo.CreateSshKey(ctx, key1)
	repo.CreateSshKey(ctx, key2)

	keys, err := repo.FindSshKeysByTeam(ctx, teamID)
	require.NoError(t, err)

	assert.Len(t, keys, 2)
}

func TestRepository_AttachDetachSshKey(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")
	teamID := "team1"
	userID := "user1"
	key := &SshKey{TeamID: &teamID, UserID: &userID, Name: "Key 1", PublicKey: "ssh-rsa AAA"}
	repo.CreateSshKey(ctx, key)

	// Attach
	err := repo.AttachSshKeyToServer(ctx, server.ID, key.ID)
	require.NoError(t, err)

	attached, err := repo.IsSshKeyAttachedToServer(ctx, server.ID, key.ID)
	require.NoError(t, err)
	assert.True(t, attached)

	// Detach
	err = repo.DetachSshKeyFromServer(ctx, server.ID, key.ID)
	require.NoError(t, err)

	attached, err = repo.IsSshKeyAttachedToServer(ctx, server.ID, key.ID)
	require.NoError(t, err)
	assert.False(t, attached)
}

// Task tests

func TestRepository_CreateTask(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	taskName := "provision"
	taskScript := "echo 'hello'"
	task := &Task{
		ServerID: server.ID,
		Type:     "provision",
		Name:     &taskName,
		Script:   &taskScript,
	}

	err := repo.CreateTask(ctx, task)
	require.NoError(t, err)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "pending", task.Status)
}

func TestRepository_FindTasksByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	name1 := "task1"
	name2 := "task2"
	script1 := "cmd1"
	script2 := "cmd2"
	task1 := &Task{ServerID: server.ID, Type: "task", Name: &name1, Script: &script1}
	task2 := &Task{ServerID: server.ID, Type: "task", Name: &name2, Script: &script2}
	repo.CreateTask(ctx, task1)
	repo.CreateTask(ctx, task2)

	tasks, err := repo.FindTasksByServer(ctx, server.ID, 10)
	require.NoError(t, err)

	assert.Len(t, tasks, 2)
}

func TestRepository_FindLatestTaskByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	name1 := "first"
	name2 := "second"
	script := "cmd"
	task1 := &Task{ServerID: server.ID, Type: "task", Name: &name1, Script: &script}
	repo.CreateTask(ctx, task1)

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	task2 := &Task{ServerID: server.ID, Type: "task", Name: &name2, Script: &script}
	repo.CreateTask(ctx, task2)

	latest, err := repo.FindLatestTaskByServer(ctx, server.ID)
	require.NoError(t, err)

	assert.Equal(t, "second", *latest.Name)
}

// Metric tests

func TestRepository_CreateMetric(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	metric := &Metric{
		ServerID:    server.ID,
		CPUUsage:    25.5,
		MemoryUsage: 60.0,
		DiskUsage:   50.0,
	}

	err := repo.CreateMetric(ctx, metric)
	require.NoError(t, err)

	assert.NotEmpty(t, metric.ID)
}

func TestRepository_FindMetricsByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	for i := 0; i < 5; i++ {
		metric := &Metric{
			ServerID: server.ID,
			CPUUsage: float64(10 * (i + 1)),
		}
		repo.CreateMetric(ctx, metric)
	}

	metrics, err := repo.FindMetricsByServer(ctx, server.ID, nil, nil, 3)
	require.NoError(t, err)

	assert.Len(t, metrics, 3)
}

func TestRepository_FindLatestMetricByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	now := time.Now()
	metric1 := &Metric{ServerID: server.ID, CPUUsage: 10.0, RecordedAt: now}
	repo.CreateMetric(ctx, metric1)

	metric2 := &Metric{ServerID: server.ID, CPUUsage: 50.0, RecordedAt: now.Add(time.Second)}
	repo.CreateMetric(ctx, metric2)

	latest, err := repo.FindLatestMetricByServer(ctx, server.ID)
	require.NoError(t, err)

	assert.Equal(t, 50.0, latest.CPUUsage)
}

// Count tests

func TestRepository_CountServersByTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestServer(t, repo, teamID, "server1")
	createTestServer(t, repo, teamID, "server2")
	createTestServer(t, repo, "other_team", "server3")

	count, err := repo.CountServersByTeam(ctx, teamID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestRepository_ServerHasLaunchAgent(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	server := createTestServer(t, repo, "team1", "test-server")

	// Initially no launch agent
	has, err := repo.ServerHasLaunchAgent(ctx, server.ID)
	require.NoError(t, err)
	assert.False(t, has)

	// Add launch agent
	agent := &InstalledService{
		ServerID: server.ID,
		Type:     ServiceTypeLaunchAgent,
		Name:     "Launch Agent",
	}
	repo.CreateService(ctx, agent)

	has, err = repo.ServerHasLaunchAgent(ctx, server.ID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestRepository_Transaction(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("successful transaction", func(t *testing.T) {
		err := repo.Transaction(ctx, func(tx *Repository) error {
			server := &Server{
				TeamID:          "team1",
				UserID:          "user1",
				Name:            "tx-server",
				Provider:        ProviderCustom,
				Type:            ServerTypePhp,
				OperatingSystem: OSUbuntu24,
			}
			return tx.CreateServer(ctx, server)
		})
		require.NoError(t, err)

		servers, _ := repo.FindAllServersByTeam(ctx, "team1")
		assert.Len(t, servers, 1)
	})

	t.Run("rollback on error", func(t *testing.T) {
		initialCount, _ := repo.CountServersByTeam(ctx, "team2")

		err := repo.Transaction(ctx, func(tx *Repository) error {
			server := &Server{
				TeamID:          "team2",
				UserID:          "user1",
				Name:            "should-not-exist",
				Provider:        ProviderCustom,
				Type:            ServerTypePhp,
				OperatingSystem: OSUbuntu24,
			}
			tx.CreateServer(ctx, server)
			return assert.AnError // Force rollback
		})
		assert.Error(t, err)

		finalCount, _ := repo.CountServersByTeam(ctx, "team2")
		assert.Equal(t, initialCount, finalCount)
	})
}
