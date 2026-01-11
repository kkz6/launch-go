package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/queue"
	sshClient "github.com/kkz6/launch-go/internal/ssh"
	"github.com/kkz6/launch-go/internal/taskrunner"
	"github.com/kkz6/launch-go/internal/websocket"
)

var (
	ErrServerNotProvisioned = errors.New("server is not provisioned")
	ErrServerNotConnected   = errors.New("server is not connected")
	ErrInvalidProvider      = errors.New("invalid server provider")
	ErrInvalidServerType    = errors.New("invalid server type")
	ErrInvalidSoftware      = errors.New("invalid software")
	ErrServiceAlreadyExists = errors.New("service already exists")
	ErrCannotDeleteService  = errors.New("cannot delete service")
)

// Service provides business logic for server operations
type Service struct {
	repo       *Repository
	queue      *queue.Client
	ws         *websocket.Hub
	dispatcher *taskrunner.Dispatcher
	logger     *zerolog.Logger
}

// NewService creates a new Service instance
func NewService(repo *Repository, queue *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger) *Service {
	return &Service{
		repo:       repo,
		queue:      queue,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// Server operations

// ListServers returns all servers for a team
func (s *Service) ListServers(ctx context.Context, teamID string) ([]Server, error) {
	return s.repo.FindAllServersByTeam(ctx, teamID)
}

// ListServersPaginated returns servers with pagination
func (s *Service) ListServersPaginated(ctx context.Context, teamID string, page, perPage int) ([]Server, int64, error) {
	offset := (page - 1) * perPage
	return s.repo.FindAllServersByTeamPaginated(ctx, teamID, perPage, offset)
}

// GetServer returns a server by ID
func (s *Service) GetServer(ctx context.Context, id, teamID string) (*Server, error) {
	return s.repo.FindServerByIDAndTeam(ctx, id, teamID)
}

// GetServerWithRelations returns a server with all relations
func (s *Service) GetServerWithRelations(ctx context.Context, id, teamID string) (*Server, error) {
	return s.repo.FindServerWithRelations(ctx, id, teamID)
}

// CreateServer creates a new server
func (s *Service) CreateServer(ctx context.Context, teamID, userID string, req *CreateServerRequest) (*Server, error) {
	provider, err := ParseServerProvider(req.Provider)
	if err != nil {
		return nil, ErrInvalidProvider
	}

	serverType, err := ParseServerType(req.Type)
	if err != nil {
		return nil, ErrInvalidServerType
	}

	os := OSUbuntu24
	if req.OperatingSystem != "" {
		os, err = ParseOperatingSystem(req.OperatingSystem)
		if err != nil {
			os = OSUbuntu24
		}
	}

	// Generate SSH key pair
	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	server := &Server{
		TeamID:          teamID,
		UserID:          userID,
		Name:            req.Name,
		Description:     req.Description,
		Provider:        provider,
		Type:            serverType,
		OperatingSystem: os,
		Status:          ServerStatusNew,
		SSHPort:         22,
		Username:        "launch",
		PrivateKey:      &privateKey,
		PublicKey:       &publicKey,
	}

	if req.SSHPort > 0 {
		server.SSHPort = req.SSHPort
	}

	if req.SSHUser != "" {
		server.Username = req.SSHUser
	}

	// For custom servers, use provided details
	if provider == ProviderCustom {
		server.PublicIPv4 = &req.IPAddress
		server.PrivateKey = &req.PrivateKey
		server.PublicKey = nil
	}

	workingDir := ".launch"
	server.WorkingDirectory = &workingDir

	if err := s.repo.CreateServer(ctx, server); err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Dispatch provisioning job for cloud providers
	if provider != ProviderCustom {
		if err := s.dispatchProvisionJob(server); err != nil {
			s.logger.Error().Err(err).Str("server_id", server.ID).Msg("Failed to dispatch provision job")
		}
	}

	return server, nil
}

// UpdateServer updates a server
func (s *Service) UpdateServer(ctx context.Context, id, teamID string, req *UpdateServerRequest) (*Server, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		server.Name = *req.Name
	}

	if req.Description != nil {
		server.Description = req.Description
	}

	if req.MonitoringEnabled != nil {
		server.MonitoringEnabled = *req.MonitoringEnabled
	}

	if req.AutoUpdate != nil {
		server.AutoUpdate = *req.AutoUpdate
	}

	if err := s.repo.UpdateServer(ctx, server); err != nil {
		return nil, err
	}

	s.broadcastServerUpdate(server)

	return server, nil
}

// DeleteServer deletes a server
func (s *Service) DeleteServer(ctx context.Context, id, teamID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	// Update status to deleting
	if err := s.repo.UpdateServerStatus(ctx, id, ServerStatusDeleting); err != nil {
		return err
	}

	// Dispatch deletion job for cloud providers
	if server.Provider != ProviderCustom {
		if err := s.dispatchDeleteJob(server); err != nil {
			s.logger.Error().Err(err).Str("server_id", server.ID).Msg("Failed to dispatch delete job")
		}
	}

	// For custom servers, just delete from database
	if server.Provider == ProviderCustom {
		return s.repo.DeleteServer(ctx, id)
	}

	return nil
}

// ArchiveServer archives a server
func (s *Service) ArchiveServer(ctx context.Context, id, teamID string) error {
	_, err := s.repo.FindServerByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	return s.repo.ArchiveServer(ctx, id)
}

// UnarchiveServer unarchives a server
func (s *Service) UnarchiveServer(ctx context.Context, id, teamID string) error {
	return s.repo.UnarchiveServer(ctx, id)
}

// RebootServer initiates a server reboot
func (s *Service) RebootServer(ctx context.Context, id, teamID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	if !server.IsProvisioned() {
		return ErrServerNotProvisioned
	}

	task, err := jobs.NewRebootTask(server.ID)
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

// ConnectServer tests the connection to a server
func (s *Service) ConnectServer(ctx context.Context, id, teamID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
		return errors.New("server has no IP address")
	}

	if server.PrivateKey == nil || *server.PrivateKey == "" {
		return errors.New("server has no private key")
	}

	// Try to connect via SSH
	client, err := sshClient.NewClient(sshClient.Config{
		Host:       *server.PublicIPv4,
		Port:       server.SSHPort,
		User:       server.RootUsername(),
		PrivateKey: *server.PrivateKey,
		Timeout:    30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Run a simple command to verify connection
	result, err := client.Run(ctx, "whoami")
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("command failed with exit code %d", result.ExitCode)
	}

	// Update server connection status
	now := time.Now()
	if err := s.repo.UpdateServerFields(ctx, id, map[string]interface{}{
		"connected":                true,
		"last_connectivity_check": now,
	}); err != nil {
		return err
	}

	return nil
}

// Service operations

// ListServices returns all services for a server
func (s *Service) ListServices(ctx context.Context, serverID, teamID string) ([]InstalledService, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	services, err := s.repo.FindServicesByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	result := make([]InstalledService, len(services))
	for i, svc := range services {
		result[i] = svc
	}
	return result, nil
}

// InstallService installs a software on a server
func (s *Service) InstallService(ctx context.Context, serverID, teamID string, req *CreateServiceRequest) (*InstalledService, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	software, err := ParseSoftware(req.Software)
	if err != nil {
		return nil, ErrInvalidSoftware
	}

	// Check if service already exists
	existingService, _ := s.repo.FindServiceByServerAndSoftware(ctx, serverID, software)
	if existingService != nil {
		return nil, ErrServiceAlreadyExists
	}

	service := &InstalledService{
		ServerID:  serverID,
		Type:      software.GetServiceType(),
		Name:      software.Label(),
		Software:  &software,
		Status:    ServiceStatusPending,
		IsDefault: false,
	}

	version := software.GetVersion()
	service.Version = &version

	if err := s.repo.CreateService(ctx, service); err != nil {
		return nil, err
	}

	// Dispatch installation job
	if err := s.dispatchServiceInstallJob(server, service); err != nil {
		s.logger.Error().Err(err).
			Str("server_id", serverID).
			Str("service_id", service.ID).
			Msg("Failed to dispatch service install job")
	}

	return service, nil
}

// HandleServiceOperation handles service operations (start, stop, restart, remove, status)
func (s *Service) HandleServiceOperation(ctx context.Context, serverID, teamID, serviceID string, operation ServiceOption) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repo.FindServiceByID(ctx, serviceID)
	if err != nil {
		return err
	}

	if service.ServerID != serverID {
		return ErrServiceNotFound
	}

	switch operation {
	case ServiceOptionStart, ServiceOptionRestart:
		return s.dispatchServiceRestartJob(server, service)
	case ServiceOptionStop:
		return s.dispatchServiceStopJob(server, service)
	case ServiceOptionRemove:
		return s.dispatchServiceRemoveJob(server, service)
	case ServiceOptionStatus:
		return s.dispatchServiceStatusJob(server, service)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

// Firewall Rule operations

// ListFirewallRules returns all firewall rules for a server
func (s *Service) ListFirewallRules(ctx context.Context, serverID, teamID string) ([]FirewallRule, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindFirewallRulesByServer(ctx, serverID)
}

// CreateFirewallRule creates a new firewall rule
func (s *Service) CreateFirewallRule(ctx context.Context, serverID, teamID string, req *CreateFirewallRuleRequest) (*FirewallRule, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	action, err := ParseRuleAction(req.Action)
	if err != nil {
		return nil, err
	}

	rule := &FirewallRule{
		ServerID: serverID,
		Name:     req.Name,
		Action:   action,
		Port:     &req.Port,
		FromIPv4: req.FromIPv4,
		Mask:     req.Mask,
		Note:     req.Note,
	}

	if err := s.repo.CreateFirewallRule(ctx, rule); err != nil {
		return nil, err
	}

	// Dispatch installation job
	if server.IsProvisioned() {
		if err := s.dispatchFirewallRuleInstallJob(server, rule); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("rule_id", rule.ID).
				Msg("Failed to dispatch firewall rule install job")
		}
	}

	return rule, nil
}

// UpdateFirewallRule updates a firewall rule
func (s *Service) UpdateFirewallRule(ctx context.Context, serverID, teamID, ruleID string, req *UpdateFirewallRuleRequest) (*FirewallRule, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	rule, err := s.repo.FindFirewallRuleByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}

	if req.Action != nil {
		action, err := ParseRuleAction(*req.Action)
		if err != nil {
			return nil, err
		}
		rule.Action = action
	}

	if req.Port != nil {
		rule.Port = req.Port
	}

	if req.FromIPv4 != nil {
		rule.FromIPv4 = req.FromIPv4
	}

	if req.Mask != nil {
		rule.Mask = req.Mask
	}

	if req.Note != nil {
		rule.Note = req.Note
	}

	if err := s.repo.UpdateFirewallRule(ctx, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

// DeleteFirewallRule deletes a firewall rule
func (s *Service) DeleteFirewallRule(ctx context.Context, serverID, teamID, ruleID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	rule, err := s.repo.FindFirewallRuleByIDAndServer(ctx, ruleID, serverID)
	if err != nil {
		return err
	}

	// If installed, dispatch uninstallation job first
	if rule.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		rule.UninstallationRequestedAt = &now
		if err := s.repo.UpdateFirewallRule(ctx, rule); err != nil {
			return err
		}

		// Dispatch uninstallation job
		if err := s.dispatchFirewallRuleUninstallJob(server, rule); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("rule_id", ruleID).
				Msg("Failed to dispatch firewall rule uninstall job")
		}

		return nil
	}

	return s.repo.DeleteFirewallRule(ctx, ruleID)
}

// Cron operations

// ListCrons returns all cron jobs for a server
func (s *Service) ListCrons(ctx context.Context, serverID, teamID string) ([]Cron, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindVisibleCronsByServer(ctx, serverID)
}

// CreateCron creates a new cron job
func (s *Service) CreateCron(ctx context.Context, serverID, teamID string, req *CreateCronRequest) (*Cron, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	user := "root"
	if req.User != "" {
		user = req.User
	}

	cron := &Cron{
		ServerID:   serverID,
		SiteID:     req.SiteID,
		User:       user,
		Expression: req.Expression,
		Command:    req.Command,
		Frequency:  req.Frequency,
		Hidden:     false,
	}

	if err := s.repo.CreateCron(ctx, cron); err != nil {
		return nil, err
	}

	// Dispatch installation job
	if server.IsProvisioned() {
		if err := s.dispatchCronInstallJob(server, cron); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("cron_id", cron.ID).
				Msg("Failed to dispatch cron install job")
		}
	}

	return cron, nil
}

// UpdateCron updates a cron job
func (s *Service) UpdateCron(ctx context.Context, serverID, teamID, cronID string, req *UpdateCronRequest) (*Cron, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	cron, err := s.repo.FindCronByIDAndServer(ctx, cronID, serverID)
	if err != nil {
		return nil, err
	}

	if req.User != nil {
		cron.User = *req.User
	}

	if req.Expression != nil {
		cron.Expression = *req.Expression
	}

	if req.Command != nil {
		cron.Command = *req.Command
	}

	if req.Frequency != nil {
		cron.Frequency = req.Frequency
	}

	if err := s.repo.UpdateCron(ctx, cron); err != nil {
		return nil, err
	}

	return cron, nil
}

// DeleteCron deletes a cron job
func (s *Service) DeleteCron(ctx context.Context, serverID, teamID, cronID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	cron, err := s.repo.FindCronByIDAndServer(ctx, cronID, serverID)
	if err != nil {
		return err
	}

	// If installed, dispatch uninstallation job first
	if cron.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		cron.UninstallationRequestedAt = &now
		if err := s.repo.UpdateCron(ctx, cron); err != nil {
			return err
		}

		// Dispatch uninstallation job
		if err := s.dispatchCronUninstallJob(server, cron); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("cron_id", cronID).
				Msg("Failed to dispatch cron uninstall job")
		}

		return nil
	}

	return s.repo.DeleteCron(ctx, cronID)
}

// Daemon operations

// ListDaemons returns all daemons for a server
func (s *Service) ListDaemons(ctx context.Context, serverID, teamID string) ([]Daemon, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindDaemonsByServer(ctx, serverID)
}

// CreateDaemon creates a new daemon
func (s *Service) CreateDaemon(ctx context.Context, serverID, teamID string, req *CreateDaemonRequest) (*Daemon, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	user := "root"
	if req.User != "" {
		user = req.User
	}

	processes := 1
	if req.Processes > 0 {
		processes = req.Processes
	}

	stopWaitSeconds := 10
	if req.StopWaitSeconds > 0 {
		stopWaitSeconds = req.StopWaitSeconds
	}

	daemon := &Daemon{
		ServerID:        serverID,
		User:            user,
		Directory:       req.Directory,
		Command:         req.Command,
		Processes:       processes,
		StopWaitSeconds: stopWaitSeconds,
		StopSignal:      req.StopSignal,
	}

	if err := s.repo.CreateDaemon(ctx, daemon); err != nil {
		return nil, err
	}

	// Dispatch installation job
	if server.IsProvisioned() {
		if err := s.dispatchDaemonInstallJob(server, daemon); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("daemon_id", daemon.ID).
				Msg("Failed to dispatch daemon install job")
		}
	}

	return daemon, nil
}

// UpdateDaemon updates a daemon
func (s *Service) UpdateDaemon(ctx context.Context, serverID, teamID, daemonID string, req *UpdateDaemonRequest) (*Daemon, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	daemon, err := s.repo.FindDaemonByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return nil, err
	}

	if req.User != nil {
		daemon.User = *req.User
	}

	if req.Directory != nil {
		daemon.Directory = req.Directory
	}

	if req.Command != nil {
		daemon.Command = *req.Command
	}

	if req.Processes != nil {
		daemon.Processes = *req.Processes
	}

	if req.StopWaitSeconds != nil {
		daemon.StopWaitSeconds = *req.StopWaitSeconds
	}

	if req.StopSignal != nil {
		daemon.StopSignal = req.StopSignal
	}

	if err := s.repo.UpdateDaemon(ctx, daemon); err != nil {
		return nil, err
	}

	return daemon, nil
}

// DeleteDaemon deletes a daemon
func (s *Service) DeleteDaemon(ctx context.Context, serverID, teamID, daemonID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemon, err := s.repo.FindDaemonByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return err
	}

	// If installed, dispatch uninstallation job first
	if daemon.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		daemon.UninstallationRequestedAt = &now
		if err := s.repo.UpdateDaemon(ctx, daemon); err != nil {
			return err
		}

		// Dispatch uninstallation job
		if err := s.dispatchDaemonUninstallJob(server, daemon); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("daemon_id", daemonID).
				Msg("Failed to dispatch daemon uninstall job")
		}

		return nil
	}

	return s.repo.DeleteDaemon(ctx, daemonID)
}

// SSH Key operations

// ListSshKeys returns all SSH keys for a team
func (s *Service) ListSshKeys(ctx context.Context, teamID string) ([]SshKey, error) {
	return s.repo.FindSshKeysByTeam(ctx, teamID)
}

// ListServerSshKeys returns all SSH keys attached to a server
func (s *Service) ListServerSshKeys(ctx context.Context, serverID, teamID string) ([]SshKey, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindSshKeysByServer(ctx, serverID)
}

// CreateSshKey creates a new SSH key
func (s *Service) CreateSshKey(ctx context.Context, teamID, userID string, req *CreateSshKeyRequest) (*SshKey, error) {
	key := &SshKey{
		TeamID:      &teamID,
		UserID:      &userID,
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Description: req.Description,
		IsGlobal:    req.IsGlobal,
	}

	if err := s.repo.CreateSshKey(ctx, key); err != nil {
		return nil, err
	}

	return key, nil
}

// AttachSshKey attaches an SSH key to a server
func (s *Service) AttachSshKey(ctx context.Context, serverID, teamID, sshKeyID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	key, err := s.repo.FindSshKeyByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	// Check if already attached
	attached, err := s.repo.IsSshKeyAttachedToServer(ctx, serverID, sshKeyID)
	if err != nil {
		return err
	}

	if attached {
		return nil
	}

	if err := s.repo.AttachSshKeyToServer(ctx, serverID, sshKeyID); err != nil {
		return err
	}

	// Dispatch job to add key to server
	if server.IsProvisioned() {
		if err := s.dispatchSshKeyAddJob(server, key); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("ssh_key_id", sshKeyID).
				Msg("Failed to dispatch SSH key add job")
		}
	}

	return nil
}

// DetachSshKey detaches an SSH key from a server
func (s *Service) DetachSshKey(ctx context.Context, serverID, teamID, sshKeyID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	key, err := s.repo.FindSshKeyByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	if err := s.repo.DetachSshKeyFromServer(ctx, serverID, sshKeyID); err != nil {
		return err
	}

	// Dispatch job to remove key from server
	if server.IsProvisioned() {
		if err := s.dispatchSshKeyRemoveJob(server, key); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("ssh_key_id", sshKeyID).
				Msg("Failed to dispatch SSH key remove job")
		}
	}

	return nil
}

// DeleteSshKey deletes an SSH key
func (s *Service) DeleteSshKey(ctx context.Context, teamID, sshKeyID string) error {
	key, err := s.repo.FindSshKeyByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	// Verify ownership
	if key.TeamID != nil && *key.TeamID != teamID {
		return ErrSshKeyNotFound
	}

	return s.repo.DeleteSshKey(ctx, sshKeyID)
}

// Task operations

// ListTasks returns tasks for a server
func (s *Service) ListTasks(ctx context.Context, serverID, teamID string, limit int) ([]Task, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindTasksByServer(ctx, serverID, limit)
}

// GetLatestTask returns the latest task for a server
func (s *Service) GetLatestTask(ctx context.Context, serverID, teamID string) (*Task, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindLatestTaskByServer(ctx, serverID)
}

// Metric operations

// GetLatestMetric returns the latest metric for a server
func (s *Service) GetLatestMetric(ctx context.Context, serverID, teamID string) (*Metric, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindLatestMetricByServer(ctx, serverID)
}

// GetMetrics returns metrics for a server
func (s *Service) GetMetrics(ctx context.Context, serverID, teamID string, from, to *time.Time, limit int) ([]Metric, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindMetricsByServer(ctx, serverID, from, to, limit)
}

// Database operations (placeholder - would connect to actual database management)

// ListDatabases returns all databases on a server
func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]DatabaseResponse, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// TODO: Implement actual database listing via SSH or database service
	return []DatabaseResponse{}, nil
}

// CreateDatabase creates a new database on a server
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID string, req *CreateDatabaseRequest) (*DatabaseResponse, error) {
	_, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// TODO: Implement actual database creation via SSH or database service
	return &DatabaseResponse{
		Name: req.Name,
	}, nil
}

// HasLaunchAgent checks if a server has the Launch Agent installed
func (s *Service) HasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	return s.repo.ServerHasLaunchAgent(ctx, serverID)
}

// GetShowPageData returns all data needed for the server show page
func (s *Service) GetShowPageData(ctx context.Context, serverID, teamID string) (*ServerShowPageData, error) {
	server, err := s.repo.FindServerWithRelations(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	latestTask, _ := s.repo.FindLatestTaskByServer(ctx, serverID)
	latestMetric, _ := s.repo.FindLatestMetricByServer(ctx, serverID)
	hasLaunchAgent, _ := s.repo.ServerHasLaunchAgent(ctx, serverID)

	// Convert to response types
	services := make([]ServiceResponse, len(server.Services))
	for i, svc := range server.Services {
		services[i] = ToServiceResponse(&svc)
	}

	firewallRules := make([]FirewallRuleResponse, len(server.FirewallRules))
	for i, rule := range server.FirewallRules {
		firewallRules[i] = ToFirewallRuleResponse(&rule)
	}

	crons := make([]CronResponse, len(server.Crons))
	for i, cron := range server.Crons {
		crons[i] = ToCronResponse(&cron)
	}

	daemons := make([]DaemonResponse, len(server.Daemons))
	for i, daemon := range server.Daemons {
		daemons[i] = ToDaemonResponse(&daemon)
	}

	sshKeys := make([]SshKeyResponse, len(server.SshKeys))
	for i, key := range server.SshKeys {
		sshKeys[i] = ToSshKeyResponse(&key)
	}

	data := &ServerShowPageData{
		Server:         ToServerResponse(server),
		Services:       services,
		FirewallRules:  firewallRules,
		Crons:          crons,
		Daemons:        daemons,
		SshKeys:        sshKeys,
		RuleActions:    GetAllRuleActions(),
		HasLaunchAgent: hasLaunchAgent,
	}

	if latestTask != nil {
		taskResp := ToTaskResponse(latestTask)
		data.LatestTask = &taskResp
	}

	if latestMetric != nil {
		metricResp := ToMetricResponse(latestMetric)
		data.LatestMetric = &metricResp
	}

	return data, nil
}

// Helper methods

func (s *Service) broadcastServerUpdate(server *Server) {
	if s.ws != nil {
		s.ws.BroadcastToTeam(server.TeamID, "server.updated", ToServerResponse(server))
	}
}

func (s *Service) dispatchProvisionJob(server *Server) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewProvisionTask(server.ID, server.TeamID)
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueCritical(task)
	return err
}

func (s *Service) dispatchDeleteJob(server *Server) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewDeleteTask(server.ID, server.TeamID)
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchServiceInstallJob(server *Server, service *InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewInstallServiceTask(server.ID, service.ID, service.Software.String())
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchServiceRestartJob(server *Server, service *InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "restart")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchServiceStopJob(server *Server, service *InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "stop")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchServiceRemoveJob(server *Server, service *InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "remove")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchServiceStatusJob(server *Server, service *InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "status")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchFirewallRuleInstallJob(server *Server, rule *FirewallRule) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewFirewallRuleTask(server.ID, rule.ID, "install")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchFirewallRuleUninstallJob(server *Server, rule *FirewallRule) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewFirewallRuleTask(server.ID, rule.ID, "uninstall")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchCronInstallJob(server *Server, cron *Cron) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCronTask(server.ID, cron.ID, "install")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchCronUninstallJob(server *Server, cron *Cron) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCronTask(server.ID, cron.ID, "uninstall")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchDaemonInstallJob(server *Server, daemon *Daemon) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewDaemonTask(server.ID, daemon.ID, "install")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchDaemonUninstallJob(server *Server, daemon *Daemon) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewDaemonTask(server.ID, daemon.ID, "uninstall")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchSshKeyAddJob(server *Server, key *SshKey) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewSshKeyTask(server.ID, key.ID, "add")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func (s *Service) dispatchSshKeyRemoveJob(server *Server, key *SshKey) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewSshKeyTask(server.ID, key.ID, "remove")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)
	return err
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(publicKey))

	return string(privateKeyPEM), publicKeyStr, nil
}
