package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/activity"
)

// ListSSHKeys returns all SSH keys for a team
func (s *Service) ListSSHKeys(ctx context.Context, teamID string) ([]models.SSHKey, error) {
	return s.repos.SSHKey().FindByTeam(ctx, teamID)
}

// ListServerSSHKeys returns all SSH keys attached to a server
func (s *Service) ListServerSSHKeys(ctx context.Context, serverID, teamID string) ([]models.SSHKey, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.SSHKey().FindByServer(ctx, serverID)
}

// CreateSSHKey creates a new SSH key
func (s *Service) CreateSSHKey(ctx context.Context, teamID, userID string, req *dto.CreateSSHKeyRequest) (*models.SSHKey, error) {
	key := &models.SSHKey{
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Description: req.Description,
		IsGlobal:    req.IsGlobal,
	}
	key.TeamID = teamID
	key.UserID = userID

	if err := s.repos.SSHKey().Create(ctx, key); err != nil {
		return nil, err
	}

	activity.LogEvent(ctx, s.repos.DB(), "created", "", key, "SSH key was created")

	return key, nil
}

// AttachSSHKey attaches an SSH key to a server
func (s *Service) AttachSSHKey(ctx context.Context, serverID, teamID, sshKeyID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	key, err := s.repos.SSHKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	attached, err := s.repos.SSHKey().IsAttachedToServer(ctx, serverID, sshKeyID)
	if err != nil {
		return err
	}

	if attached {
		return nil
	}

	if err := s.repos.SSHKey().AttachToServer(ctx, serverID, sshKeyID); err != nil {
		return err
	}

	if server.IsProvisioned() {
		if err := s.dispatchSSHKeyAddJob(server, key); err != nil {
			s.LogError(err, "Failed to dispatch SSH key add job", "server_id", serverID, "ssh_key_id", sshKeyID)
		}
	}

	return nil
}

// DetachSSHKey detaches an SSH key from a server
func (s *Service) DetachSSHKey(ctx context.Context, serverID, teamID, sshKeyID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	key, err := s.repos.SSHKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	if err := s.repos.SSHKey().DetachFromServer(ctx, serverID, sshKeyID); err != nil {
		return err
	}

	if server.IsProvisioned() {
		if err := s.dispatchSSHKeyRemoveJob(server, key); err != nil {
			s.LogError(err, "Failed to dispatch SSH key remove job", "server_id", serverID, "ssh_key_id", sshKeyID)
		}
	}

	return nil
}

// DeleteSSHKey deletes an SSH key
func (s *Service) DeleteSSHKey(ctx context.Context, teamID, sshKeyID string) error {
	key, err := s.repos.SSHKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	if key.TeamID != teamID {
		return ErrSSHKeyNotFound
	}

	activity.LogEvent(ctx, s.repos.DB(), "deleted", "", key, "SSH key was deleted")

	return s.repos.SSHKey().Delete(ctx, sshKeyID)
}

func (s *Service) dispatchSSHKeyAddJob(server *models.Server, key *models.SSHKey) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewAddSSHKeyTask(server.ID, key.ID)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchSSHKeyRemoveJob(server *models.Server, key *models.SSHKey) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewRemoveSSHKeyTask(server.ID, key.ID, false)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}
