package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ListServers returns all servers for a team
func (s *Service) ListServers(ctx context.Context, teamID string) ([]models.Server, error) {
	return s.repos.Server().FindAllByTeam(ctx, teamID)
}

// ListArchivedServers returns all archived servers for a team
func (s *Service) ListArchivedServers(ctx context.Context, teamID string) ([]models.Server, error) {
	return s.repos.Server().FindArchivedByTeam(ctx, teamID)
}

// ListServersPaginated returns servers with pagination
func (s *Service) ListServersPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error) {
	return s.repos.Server().FindAllByTeamPaginated(ctx, teamID, page, perPage)
}

// GetServer returns a server by ID
func (s *Service) GetServer(ctx context.Context, id, teamID string) (*models.Server, error) {
	return s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
}

// GetServerWithRelations returns a server with all relations
func (s *Service) GetServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	return s.repos.Server().FindWithRelations(ctx, id, teamID)
}

// CreateServer creates a new server
func (s *Service) CreateServer(ctx context.Context, teamID, userID string, req *dto.CreateServerRequest) (*models.Server, error) {
	provider, err := types.ParseServerProvider(req.Provider)
	if err != nil {
		return nil, ErrInvalidProvider
	}

	serverType, err := types.ParseServerType(req.Type)
	if err != nil {
		return nil, ErrInvalidServerType
	}

	os := types.OSUbuntu24
	if req.OperatingSystem != "" {
		os, err = types.ParseOperatingSystem(req.OperatingSystem)
		if err != nil {
			os = types.OSUbuntu24
		}
	}

	privateKey, publicKey, err := GenerateSSHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	serverTypeStr := serverType.String()
	osStr := os.String()
	defaultSSHPort := 22
	defaults := config.ServerDefaults()
	defaultUsername := defaults.Username

	server := &models.Server{
		Name:            req.Name,
		Description:     req.Description,
		Provider:        provider,
		Type:            &serverTypeStr,
		OperatingSystem: &osStr,
		Status:          types.ServerStatusNew,
		SSHPort:         &defaultSSHPort,
		Username:        &defaultUsername,
		PrivateKey:      dbtype.EncryptedString(privateKey),
		PublicKey:       dbtype.EncryptedString(publicKey),
	}
	server.TeamID = teamID
	server.UserID = userID

	if req.SSHPort > 0 {
		server.SSHPort = &req.SSHPort
	}

	if req.SSHUser != "" {
		server.Username = &req.SSHUser
	}

	if provider == types.ProviderCustom {
		server.PublicIPv4 = &req.IPAddress
		server.PrivateKey = dbtype.EncryptedString(req.PrivateKey)
		server.PublicKey = ""
	}

	workingDir := ".launch"
	server.WorkingDirectory = &workingDir

	// Set provider data for cloud servers
	if provider != types.ProviderCustom {
		server.ServerProviderID = &req.CredentialID
		server.ProviderData = map[string]interface{}{
			"region": req.Region,
			"plan":   req.Size,
		}
	}

	if err := s.repos.Server().Create(ctx, server); err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	activity.RecordCreated(ctx, userID, server, "Server was created")

	if provider != types.ProviderCustom {
		if err := s.dispatchCreateOnProviderJob(server, req.CredentialID, req.SSHKeyIDs); err != nil {
			s.LogError(err, "Failed to dispatch create on provider job", "server_id", server.ID)
		}
	}

	return server, nil
}

// UpdateServer updates a server
func (s *Service) UpdateServer(ctx context.Context, id, teamID string, req *dto.UpdateServerRequest) (*models.Server, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
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

	if err := s.repos.Server().Update(ctx, server); err != nil {
		return nil, err
	}

	activity.RecordEvent(ctx, "updated", "", server, "Server was updated")

	s.broadcastServerUpdate(server)

	return server, nil
}

// DeleteServer deletes a server
func (s *Service) DeleteServer(ctx context.Context, id, teamID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "deleted", "", server, "Server deletion requested")

	if err := s.repos.Server().UpdateStatus(ctx, id, types.ServerStatusDeleting); err != nil {
		return err
	}

	if server.Provider != types.ProviderCustom {
		if err := s.dispatchDeleteJob(server); err != nil {
			s.LogError(err, "Failed to dispatch delete job", "server_id", server.ID)
		}
	}

	if server.Provider == types.ProviderCustom {
		return s.repos.Server().Delete(ctx, id)
	}

	return nil
}

// ArchiveServer archives a server
func (s *Service) ArchiveServer(ctx context.Context, id, teamID string) error {
	_, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	return s.repos.Server().Archive(ctx, id)
}

// UnarchiveServer unarchives a server
func (s *Service) UnarchiveServer(ctx context.Context, id, teamID string) error {
	return s.repos.Server().Unarchive(ctx, id)
}

// RebootServer initiates a server reboot
func (s *Service) RebootServer(ctx context.Context, id, teamID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	if !server.IsProvisioned() {
		return ErrServerNotProvisioned
	}

	task, err := jobs.NewRebootServerTask(server.ID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

// ConnectServer tests the connection to a server
func (s *Service) ConnectServer(ctx context.Context, id, teamID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
		return errors.New("server has no IP address")
	}

	if server.PrivateKey.IsEmpty() {
		return errors.New("server has no private key")
	}

	client, err := server.ConnectionAsRoot().Dial()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	result, err := client.Run(ctx, "whoami")
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("command failed with exit code %d", result.ExitCode)
	}

	now := time.Now()
	return s.repos.Server().UpdateFields(ctx, id, map[string]interface{}{
		"connected":               true,
		"last_connectivity_check": now,
	})
}

// HasLaunchAgent checks if a server has the Launch Agent installed
func (s *Service) HasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	return s.repos.Server().HasLaunchAgent(ctx, serverID)
}

// GetShowPageData returns all data needed for the server show page
func (s *Service) GetShowPageData(ctx context.Context, serverID, teamID string) (*dto.ServerShowPageData, error) {
	server, err := s.repos.Server().FindWithRelations(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	latestTask, err := s.repos.Task().FindLatestByServer(ctx, serverID)
	if err != nil {
		s.LogWarn("Failed to fetch latest task for server", "serverID", serverID, "error", err)
	}
	latestMetric, err := s.repos.Metric().FindLatestByServer(ctx, serverID)
	if err != nil {
		s.LogWarn("Failed to fetch latest metric for server", "serverID", serverID, "error", err)
	}
	hasLaunchAgent, err := s.repos.Server().HasLaunchAgent(ctx, serverID)
	if err != nil {
		s.LogWarn("Failed to check launch agent status", "serverID", serverID, "error", err)
	}

	services := make([]dto.ServiceResponse, len(server.Services))
	for i, svc := range server.Services {
		services[i] = dto.ToServiceResponse(&svc)
	}

	firewallRules := make([]dto.FirewallRuleResponse, len(server.FirewallRules))
	for i, rule := range server.FirewallRules {
		firewallRules[i] = dto.ToFirewallRuleResponse(&rule)
	}

	crons := make([]dto.CronResponse, len(server.Crons))
	for i, cron := range server.Crons {
		crons[i] = dto.ToCronResponse(&cron)
	}

	daemons := make([]dto.DaemonResponse, len(server.Daemons))
	for i, daemon := range server.Daemons {
		daemons[i] = dto.ToDaemonResponse(&daemon)
	}

	sshKeys := make([]dto.SSHKeyResponse, len(server.SSHKeys))
	for i, key := range server.SSHKeys {
		sshKeys[i] = dto.ToSSHKeyResponse(&key)
	}

	data := &dto.ServerShowPageData{
		Server:         dto.ToServerResponse(server),
		Services:       services,
		FirewallRules:  firewallRules,
		Crons:          crons,
		Daemons:        daemons,
		SSHKeys:        sshKeys,
		RuleActions:    dto.GetAllRuleActions(),
		HasLaunchAgent: hasLaunchAgent,
	}

	if latestTask != nil {
		taskResp := dto.ToTaskResponse(latestTask)
		data.LatestTask = &taskResp
	}

	if latestMetric != nil {
		metricResp := dto.ToMetricResponse(latestMetric)
		data.LatestMetric = &metricResp
	}

	return data, nil
}

func (s *Service) broadcastServerUpdate(server *models.Server) {
	s.BroadcastToTeam(server.TeamID, "server.updated", dto.ToServerResponse(server))
}

func (s *Service) dispatchCreateOnProviderJob(server *models.Server, serverProviderID string, sshKeyIDs []string) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCreateOnProviderTask(server.ID, server.TeamID, serverProviderID, nil, sshKeyIDs)
	if err != nil {
		return err
	}

	return s.EnqueueTaskWithOptions(task)
}

func (s *Service) dispatchDeleteJob(server *models.Server) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewDeleteServerTask(server.ID, server.TeamID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

// GenerateSSHKeyPair generates an RSA SSH key pair
func GenerateSSHKeyPair() (privateKeyStr string, publicKeyStr string, err error) {
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

	publicKeyStr = string(ssh.MarshalAuthorizedKey(publicKey))

	return string(privateKeyPEM), publicKeyStr, nil
}

// GetProvisionScript returns the provision script for a server
// This is used for custom servers that need to run the provision script manually
func (s *Service) GetProvisionScript(ctx context.Context, serverID string) (string, error) {
	// Find server (including archived)
	server, err := s.repos.Server().FindByID(ctx, serverID)
	if err != nil {
		return "", err
	}

	if server == nil {
		return "", errors.New("server not found")
	}

	// Only custom servers or archived servers can get provision script
	if server.Provider != types.ProviderCustom && server.ArchivedAt == nil {
		return "", nil
	}

	// Generate the provision script that authorizes our management key
	script := s.generateAuthorizeKeyScript(server)
	return script, nil
}

// generateAuthorizeKeyScript generates a bash script to authorize the management key
func (s *Service) generateAuthorizeKeyScript(server *models.Server) string {
	publicKey := string(server.PublicKey)

	script := `#!/bin/bash
set -e

# Launch Server Provisioning Script
# Server: %s

echo "Authorizing Launch management key..."

# Create .ssh directory if it doesn't exist
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# Add the public key to authorized_keys if not already present
PUBLIC_KEY="%s"

if ! grep -q "$PUBLIC_KEY" ~/.ssh/authorized_keys 2>/dev/null; then
    echo "$PUBLIC_KEY" >> ~/.ssh/authorized_keys
    chmod 600 ~/.ssh/authorized_keys
    echo "Management key authorized successfully."
else
    echo "Management key already authorized."
fi

# Notify the server that provisioning is complete
echo "Provisioning script completed."
`
	return fmt.Sprintf(script, server.Name, publicKey)
}

// RunVulnerabilityAudit runs a security vulnerability audit on a server
func (s *Service) RunVulnerabilityAudit(ctx context.Context, serverID, teamID, userID string, emailRecipient *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	if !server.IsProvisioned() {
		return ErrServerNotProvisioned
	}

	if !server.IsConnected() {
		return ErrServerNotConnected
	}

	task, err := jobs.NewVulnerabilityAuditTask(server.ID, teamID, &userID, emailRecipient)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "vulnerability_audit_started", userID, server, "Vulnerability audit was initiated")

	return s.EnqueueTask(task)
}
