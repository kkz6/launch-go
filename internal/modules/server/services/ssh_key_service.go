package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
)

// ListSshKeys returns all SSH keys for a team
func (s *Service) ListSshKeys(ctx context.Context, teamID string) ([]models.SshKey, error) {
	return s.repos.SshKey().FindByTeam(ctx, teamID)
}

// ListServerSshKeys returns all SSH keys attached to a server
func (s *Service) ListServerSshKeys(ctx context.Context, serverID, teamID string) ([]models.SshKey, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.SshKey().FindByServer(ctx, serverID)
}

// CreateSshKey creates a new SSH key
func (s *Service) CreateSshKey(ctx context.Context, teamID, userID string, req *dto.CreateSshKeyRequest) (*models.SshKey, error) {
	key := &models.SshKey{
		TeamID:      teamID,
		UserID:      userID,
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Description: req.Description,
		IsGlobal:    req.IsGlobal,
	}

	if err := s.repos.SshKey().Create(ctx, key); err != nil {
		return nil, err
	}

	activity.New(s.repos.DB()).
		WithContext(ctx).
		UseLog("server").
		On(key).
		WithEvent("created").
		Log("SSH key was created")

	return key, nil
}

// AttachSshKey attaches an SSH key to a server
func (s *Service) AttachSshKey(ctx context.Context, serverID, teamID, sshKeyID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	key, err := s.repos.SshKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	attached, err := s.repos.SshKey().IsAttachedToServer(ctx, serverID, sshKeyID)
	if err != nil {
		return err
	}

	if attached {
		return nil
	}

	if err := s.repos.SshKey().AttachToServer(ctx, serverID, sshKeyID); err != nil {
		return err
	}

	if server.IsProvisioned() {
		if err := s.dispatchSshKeyAddJob(server, key); err != nil {
			s.LogError(err, "Failed to dispatch SSH key add job", "server_id", serverID, "ssh_key_id", sshKeyID)
		}
	}

	return nil
}

// DetachSshKey detaches an SSH key from a server
func (s *Service) DetachSshKey(ctx context.Context, serverID, teamID, sshKeyID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	key, err := s.repos.SshKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	if err := s.repos.SshKey().DetachFromServer(ctx, serverID, sshKeyID); err != nil {
		return err
	}

	if server.IsProvisioned() {
		if err := s.dispatchSshKeyRemoveJob(server, key); err != nil {
			s.LogError(err, "Failed to dispatch SSH key remove job", "server_id", serverID, "ssh_key_id", sshKeyID)
		}
	}

	return nil
}

// DeleteSshKey deletes an SSH key
func (s *Service) DeleteSshKey(ctx context.Context, teamID, sshKeyID string) error {
	key, err := s.repos.SshKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	if key.TeamID != teamID {
		return ErrSSHKeyNotFound
	}

	activity.New(s.repos.DB()).
		WithContext(ctx).
		UseLog("server").
		On(key).
		WithEvent("deleted").
		Log("SSH key was deleted")

	return s.repos.SshKey().Delete(ctx, sshKeyID)
}

func (s *Service) dispatchSshKeyAddJob(server *models.Server, key *models.SshKey) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewAddSshKeyTask(server.ID, key.ID)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchSshKeyRemoveJob(server *models.Server, key *models.SshKey) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewRemoveSshKeyTask(server.ID, key.ID, false)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}
