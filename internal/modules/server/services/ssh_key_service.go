package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListSshKeys returns all SSH keys for a team
func (s *Service) ListSshKeys(ctx context.Context, teamID string) ([]models.SshKey, error) {
	return s.repo.FindSshKeysByTeam(ctx, teamID)
}

// ListServerSshKeys returns all SSH keys attached to a server
func (s *Service) ListServerSshKeys(ctx context.Context, serverID, teamID string) ([]models.SshKey, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindSshKeysByServer(ctx, serverID)
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

	if key.TeamID != teamID {
		return ErrSshKeyNotFound
	}

	return s.repo.DeleteSshKey(ctx, sshKeyID)
}

func (s *Service) dispatchSshKeyAddJob(server *models.Server, key *models.SshKey) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewAddSshKeyTask(server.ID, key.ID)
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchSshKeyRemoveJob(server *models.Server, key *models.SshKey) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewRemoveSshKeyTask(server.ID, key.ID, false)
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}
