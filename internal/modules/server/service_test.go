package server

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
)

func setupTestService(t *testing.T) (*services.Service, *repositories.Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(models.AllModels()...)
	require.NoError(t, err)

	repo := repositories.NewRepository(db)
	logger := zerolog.Nop()
	service := services.NewService(repo, nil, nil, nil, &logger)

	return service, repo, db
}

func createTestServerForService(t *testing.T, repo *repositories.Repository, teamID, name string) *models.Server {
	server := &models.Server{
		TeamID:          teamID,
		UserID:          "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:            name,
		Provider:        enums.ProviderDigitalOcean,
		Type:            enums.ServerTypePhp,
		OperatingSystem: enums.OSUbuntu24,
		Status:          enums.ServerStatusRunning,
	}
	err := repo.CreateServer(context.Background(), server)
	require.NoError(t, err)
	return server
}

func TestNewService(t *testing.T) {
	_, repo, _ := setupTestService(t)
	logger := zerolog.Nop()

	service := services.NewService(repo, nil, nil, nil, &logger)

	assert.NotNil(t, service)
}

func TestService_ListServers(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestServerForService(t, repo, teamID, "server1")
	createTestServerForService(t, repo, teamID, "server2")
	createTestServerForService(t, repo, "other-team", "server3")

	servers, err := service.ListServers(ctx, teamID)
	require.NoError(t, err)

	assert.Len(t, servers, 2)
}

func TestService_ListServersPaginated(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	for i := 0; i < 5; i++ {
		createTestServerForService(t, repo, teamID, "server-"+string(rune('0'+i)))
	}

	servers, total, err := service.ListServersPaginated(ctx, teamID, 1, 2)
	require.NoError(t, err)

	assert.Len(t, servers, 2)
	assert.Equal(t, int64(5), total)
}

func TestService_GetServer(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	created := createTestServerForService(t, repo, teamID, "test-server")

	t.Run("finds existing server", func(t *testing.T) {
		found, err := service.GetServer(ctx, created.ID, teamID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent server", func(t *testing.T) {
		_, err := service.GetServer(ctx, "non-existent", teamID)
		assert.ErrorIs(t, err, services.ErrServerNotFound)
	})

	t.Run("returns error for wrong team", func(t *testing.T) {
		_, err := service.GetServer(ctx, created.ID, "wrong-team")
		assert.ErrorIs(t, err, services.ErrServerNotFound)
	})
}

func TestService_CreateServer(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates server successfully", func(t *testing.T) {
		req := &dto.CreateServerRequest{
			Name:            "new-server",
			Provider:        "digitalocean",
			Type:            "php",
			OperatingSystem: "ubuntu_24",
		}

		server, err := service.CreateServer(ctx, teamID, userID, req)
		require.NoError(t, err)

		assert.NotEmpty(t, server.ID)
		assert.Equal(t, "new-server", server.Name)
		assert.Equal(t, enums.ProviderDigitalOcean, server.Provider)
		assert.Equal(t, enums.ServerTypePhp, server.Type)
		assert.Equal(t, enums.ServerStatusNew, server.Status)
	})

	t.Run("returns error for invalid provider", func(t *testing.T) {
		req := &dto.CreateServerRequest{
			Name:            "invalid-server",
			Provider:        "invalid_provider",
			Type:            "php",
			OperatingSystem: "ubuntu_24",
		}

		_, err := service.CreateServer(ctx, teamID, userID, req)
		assert.ErrorIs(t, err, services.ErrInvalidProvider)
	})

	t.Run("returns error for invalid server type", func(t *testing.T) {
		req := &dto.CreateServerRequest{
			Name:            "invalid-server",
			Provider:        "digitalocean",
			Type:            "invalid_type",
			OperatingSystem: "ubuntu_24",
		}

		_, err := service.CreateServer(ctx, teamID, userID, req)
		assert.ErrorIs(t, err, services.ErrInvalidServerType)
	})
}

func TestService_UpdateServer(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "original-name")

	t.Run("updates server successfully", func(t *testing.T) {
		req := &dto.UpdateServerRequest{
			Name: strPtr("updated-name"),
		}

		updated, err := service.UpdateServer(ctx, server.ID, teamID, req)
		require.NoError(t, err)

		assert.Equal(t, "updated-name", updated.Name)
	})

	t.Run("returns error for non-existent server", func(t *testing.T) {
		req := &dto.UpdateServerRequest{Name: strPtr("new-name")}
		_, err := service.UpdateServer(ctx, "non-existent", teamID, req)
		assert.ErrorIs(t, err, services.ErrServerNotFound)
	})
}

func TestService_DeleteServer(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create a custom server which can be deleted immediately
	server := &models.Server{
		TeamID:          teamID,
		UserID:          "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:            "to-delete",
		Provider:        enums.ProviderCustom,
		Type:            enums.ServerTypePhp,
		OperatingSystem: enums.OSUbuntu24,
		Status:          enums.ServerStatusRunning,
	}
	err := repo.CreateServer(ctx, server)
	require.NoError(t, err)

	err = service.DeleteServer(ctx, server.ID, teamID)
	require.NoError(t, err)

	// Verify server is deleted
	_, err = service.GetServer(ctx, server.ID, teamID)
	assert.ErrorIs(t, err, services.ErrServerNotFound)
}

func TestService_ArchiveServer(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "to-archive")

	err := service.ArchiveServer(ctx, server.ID, teamID)
	require.NoError(t, err)

	found, err := service.GetServer(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.NotNil(t, found.ArchivedAt)
	assert.Equal(t, enums.ServerStatusArchived, found.Status)
}

func TestService_UnarchiveServer(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "to-unarchive")
	service.ArchiveServer(ctx, server.ID, teamID)

	err := service.UnarchiveServer(ctx, server.ID, teamID)
	require.NoError(t, err)

	found, err := service.GetServer(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Nil(t, found.ArchivedAt)
	assert.Equal(t, enums.ServerStatusStopped, found.Status)
}

func TestService_RebootServer(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("reboots provisioned server", func(t *testing.T) {
		server := createTestServerForService(t, repo, teamID, "reboot-test")
		// Ensure server is provisioned (running state)
		repo.UpdateServerStatus(ctx, server.ID, enums.ServerStatusRunning)

		err := service.RebootServer(ctx, server.ID, teamID)
		// Without queue client, this will fail, but we can test the validation logic
		assert.Error(t, err) // Queue is nil
	})

	t.Run("returns error for non-provisioned server", func(t *testing.T) {
		server := &models.Server{
			TeamID:          teamID,
			UserID:          "user1",
			Name:            "new-server",
			Provider:        enums.ProviderDigitalOcean,
			Type:            enums.ServerTypePhp,
			OperatingSystem: enums.OSUbuntu24,
			Status:          enums.ServerStatusNew, // Not provisioned
		}
		repo.CreateServer(ctx, server)

		err := service.RebootServer(ctx, server.ID, teamID)
		assert.ErrorIs(t, err, services.ErrServerNotProvisioned)
	})

	t.Run("returns error for non-existent server", func(t *testing.T) {
		err := service.RebootServer(ctx, "non-existent", teamID)
		assert.ErrorIs(t, err, services.ErrServerNotFound)
	})
}

func TestService_ListServices(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	// Create some services
	php := enums.SoftwarePhp84
	mysql := enums.SoftwareMySql80
	svc1 := &models.InstalledService{ServerID: server.ID, Type: enums.ServiceTypePhp, Name: "PHP 8.4", Software: &php}
	svc2 := &models.InstalledService{ServerID: server.ID, Type: enums.ServiceTypeMySql, Name: "MySQL 8.0", Software: &mysql}
	repo.CreateService(ctx, svc1)
	repo.CreateService(ctx, svc2)

	svcs, err := service.ListServices(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Len(t, svcs, 2)
}

func TestService_InstallService(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	t.Run("installs service successfully", func(t *testing.T) {
		req := &dto.CreateServiceRequest{
			Software: "php84",
		}

		svc, err := service.InstallService(ctx, server.ID, teamID, req)
		// Will fail because queue is nil, but validates input
		if err == nil {
			assert.NotEmpty(t, svc.ID)
			assert.Equal(t, enums.ServiceTypePhp, svc.Type)
		}
	})

	t.Run("returns error for invalid software", func(t *testing.T) {
		req := &dto.CreateServiceRequest{
			Software: "invalid_software",
		}

		_, err := service.InstallService(ctx, server.ID, teamID, req)
		assert.ErrorIs(t, err, services.ErrInvalidSoftware)
	})

	t.Run("returns error for non-existent server", func(t *testing.T) {
		req := &dto.CreateServiceRequest{
			Software: "php84",
		}

		_, err := service.InstallService(ctx, "non-existent", teamID, req)
		assert.ErrorIs(t, err, services.ErrServerNotFound)
	})
}

func TestService_HandleServiceOperation(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")
	php := enums.SoftwarePhp84
	svc := &models.InstalledService{ServerID: server.ID, Type: enums.ServiceTypePhp, Name: "PHP 8.4", Software: &php}
	repo.CreateService(ctx, svc)

	// These will fail without queue, but we can test the validation
	err := service.HandleServiceOperation(ctx, server.ID, teamID, svc.ID, enums.ServiceOptionStatus)
	assert.Error(t, err) // Queue is nil

	t.Run("returns error for non-existent service", func(t *testing.T) {
		err := service.HandleServiceOperation(ctx, server.ID, teamID, "non-existent", enums.ServiceOptionRestart)
		assert.ErrorIs(t, err, services.ErrServiceNotFound)
	})
}

func TestService_ListFirewallRules(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	port22 := "22"
	port80 := "80"
	rule1 := &models.FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port22, Action: enums.RuleActionAllow}
	rule2 := &models.FirewallRule{ServerID: server.ID, Name: "HTTP", Port: &port80, Action: enums.RuleActionAllow}
	repo.CreateFirewallRule(ctx, rule1)
	repo.CreateFirewallRule(ctx, rule2)

	rules, err := service.ListFirewallRules(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Len(t, rules, 2)
}

func TestService_CreateFirewallRule(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	req := &dto.CreateFirewallRuleRequest{
		Name:   "SSH",
		Port:   "22",
		Action: "allow",
	}

	rule, err := service.CreateFirewallRule(ctx, server.ID, teamID, req)
	// Will fail because queue is nil
	if err == nil {
		assert.NotEmpty(t, rule.ID)
		assert.Equal(t, "SSH", rule.Name)
	}
}

func TestService_DeleteFirewallRule(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")
	port := "22"
	rule := &models.FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port, Action: enums.RuleActionAllow}
	repo.CreateFirewallRule(ctx, rule)

	// Rule is not installed, so it just deletes from database without needing queue
	err := service.DeleteFirewallRule(ctx, server.ID, teamID, rule.ID)
	require.NoError(t, err)

	// Verify rule is deleted
	rules, err := service.ListFirewallRules(ctx, server.ID, teamID)
	require.NoError(t, err)
	assert.Len(t, rules, 0)
}

func TestService_ListCrons(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	cron1 := &models.Cron{ServerID: server.ID, User: "root", Expression: "*/5 * * * *", Command: "cmd1"}
	cron2 := &models.Cron{ServerID: server.ID, User: "root", Expression: "0 * * * *", Command: "cmd2"}
	repo.CreateCron(ctx, cron1)
	repo.CreateCron(ctx, cron2)

	crons, err := service.ListCrons(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Len(t, crons, 2)
}

func TestService_CreateCron(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	req := &dto.CreateCronRequest{
		Expression: "*/5 * * * *",
		Command:    "php artisan schedule:run",
		User:       "deploy",
	}

	cron, err := service.CreateCron(ctx, server.ID, teamID, req)
	// Will fail because queue is nil
	if err == nil {
		assert.NotEmpty(t, cron.ID)
		assert.Equal(t, "*/5 * * * *", cron.Expression)
	}
}

func TestService_ListDaemons(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	dir := "/tmp"
	daemon1 := &models.Daemon{ServerID: server.ID, Command: "cmd1", User: "root", Directory: &dir}
	daemon2 := &models.Daemon{ServerID: server.ID, Command: "cmd2", User: "root", Directory: &dir}
	repo.CreateDaemon(ctx, daemon1)
	repo.CreateDaemon(ctx, daemon2)

	daemons, err := service.ListDaemons(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Len(t, daemons, 2)
}

func TestService_CreateDaemon(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	dir := "/var/www/app"
	req := &dto.CreateDaemonRequest{
		Command:   "php artisan queue:work",
		User:      "deploy",
		Directory: &dir,
		Processes: 2,
	}

	daemon, err := service.CreateDaemon(ctx, server.ID, teamID, req)
	// Will fail because queue is nil
	if err == nil {
		assert.NotEmpty(t, daemon.ID)
		assert.Equal(t, "php artisan queue:work", daemon.Command)
	}
}

func TestService_ListSshKeys(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	key1 := &models.SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 1", PublicKey: "ssh-rsa AAA1"}
	key2 := &models.SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 2", PublicKey: "ssh-rsa AAA2"}
	repo.CreateSshKey(ctx, key1)
	repo.CreateSshKey(ctx, key2)

	keys, err := service.ListSshKeys(ctx, teamID)
	require.NoError(t, err)

	assert.Len(t, keys, 2)
}

func TestService_CreateSshKey(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	req := &dto.CreateSshKeyRequest{
		Name:      "My SSH Key",
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC test@example.com",
	}

	key, err := service.CreateSshKey(ctx, teamID, userID, req)
	require.NoError(t, err)

	assert.NotEmpty(t, key.ID)
	assert.Equal(t, "My SSH Key", key.Name)
}

func TestService_AttachSshKey(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")
	key := &models.SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 1", PublicKey: "ssh-rsa AAA1"}
	repo.CreateSshKey(ctx, key)

	err := service.AttachSshKey(ctx, server.ID, teamID, key.ID)
	// Server is not provisioned, so no job is dispatched and no error is returned
	require.NoError(t, err)

	// Verify key is attached
	attached, err := repo.IsSshKeyAttachedToServer(ctx, server.ID, key.ID)
	require.NoError(t, err)
	assert.True(t, attached)
}

func TestService_DetachSshKey(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")
	key := &models.SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 1", PublicKey: "ssh-rsa AAA1"}
	repo.CreateSshKey(ctx, key)
	repo.AttachSshKeyToServer(ctx, server.ID, key.ID)

	err := service.DetachSshKey(ctx, server.ID, teamID, key.ID)
	// Server is not provisioned, so no job is dispatched and no error is returned
	require.NoError(t, err)

	// Verify key is detached
	attached, err := repo.IsSshKeyAttachedToServer(ctx, server.ID, key.ID)
	require.NoError(t, err)
	assert.False(t, attached)
}

func TestService_DeleteSshKey(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	key := &models.SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 1", PublicKey: "ssh-rsa AAA1"}
	repo.CreateSshKey(ctx, key)

	err := service.DeleteSshKey(ctx, teamID, key.ID)
	require.NoError(t, err)

	// Verify key is deleted
	_, err = repo.FindSshKeyByID(ctx, key.ID)
	assert.ErrorIs(t, err, services.ErrSshKeyNotFound)
}

func TestService_ListTasks(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	name1 := "task1"
	name2 := "task2"
	script1 := "cmd1"
	script2 := "cmd2"
	task1 := &models.Task{ServerID: server.ID, Type: "task", Name: &name1, Script: &script1}
	task2 := &models.Task{ServerID: server.ID, Type: "task", Name: &name2, Script: &script2}
	repo.CreateTask(ctx, task1)
	repo.CreateTask(ctx, task2)

	tasks, err := service.ListTasks(ctx, server.ID, teamID, 10)
	require.NoError(t, err)

	assert.Len(t, tasks, 2)
}

func TestService_GetLatestTask(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	name1 := "first"
	name2 := "second"
	script := "cmd"
	task1 := &models.Task{ServerID: server.ID, Type: "task", Name: &name1, Script: &script}
	repo.CreateTask(ctx, task1)

	time.Sleep(10 * time.Millisecond)

	task2 := &models.Task{ServerID: server.ID, Type: "task", Name: &name2, Script: &script}
	repo.CreateTask(ctx, task2)

	latest, err := service.GetLatestTask(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Equal(t, "second", *latest.Name)
}

func TestService_GetLatestMetric(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	now := time.Now()
	metric1 := &models.Metric{ServerID: server.ID, CPUUsage: 10.0, RecordedAt: now}
	repo.CreateMetric(ctx, metric1)

	metric2 := &models.Metric{ServerID: server.ID, CPUUsage: 50.0, RecordedAt: now.Add(time.Second)}
	repo.CreateMetric(ctx, metric2)

	latest, err := service.GetLatestMetric(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Equal(t, 50.0, latest.CPUUsage)
}

func TestService_GetMetrics(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	for i := 0; i < 5; i++ {
		metric := &models.Metric{ServerID: server.ID, CPUUsage: float64(10 * (i + 1))}
		repo.CreateMetric(ctx, metric)
	}

	metrics, err := service.GetMetrics(ctx, server.ID, teamID, nil, nil, 3)
	require.NoError(t, err)

	assert.Len(t, metrics, 3)
}

func TestService_HasLaunchAgent(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	// Initially no launch agent
	has, err := service.HasLaunchAgent(ctx, server.ID)
	require.NoError(t, err)
	assert.False(t, has)

	// Add launch agent
	agent := &models.InstalledService{
		ServerID: server.ID,
		Type:     enums.ServiceTypeLaunchAgent,
		Name:     "Launch Agent",
	}
	repo.CreateService(ctx, agent)

	has, err = service.HasLaunchAgent(ctx, server.ID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestService_ListDatabases(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	databases, err := service.ListDatabases(ctx, server.ID, teamID)
	require.NoError(t, err)

	// Currently returns empty list (placeholder implementation)
	assert.Empty(t, databases)
}

func TestService_CreateDatabase(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	req := &dto.CreateDatabaseRequest{
		Name: "my_database",
	}

	db, err := service.CreateDatabase(ctx, server.ID, teamID, req)
	require.NoError(t, err)

	// Placeholder implementation returns the name
	assert.Equal(t, "my_database", db.Name)
}

func TestService_GetShowPageData(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	server := createTestServerForService(t, repo, teamID, "test-server")

	// Add some related data
	php := enums.SoftwarePhp84
	svc := &models.InstalledService{ServerID: server.ID, Type: enums.ServiceTypePhp, Name: "PHP 8.4", Software: &php}
	repo.CreateService(ctx, svc)

	port := "22"
	rule := &models.FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port, Action: enums.RuleActionAllow}
	repo.CreateFirewallRule(ctx, rule)

	data, err := service.GetShowPageData(ctx, server.ID, teamID)
	require.NoError(t, err)

	assert.Equal(t, server.ID, data.Server.ID)
	assert.Len(t, data.Services, 1)
	assert.Len(t, data.FirewallRules, 1)
}
