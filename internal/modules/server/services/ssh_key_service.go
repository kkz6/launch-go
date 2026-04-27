package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// ListSSHKeys returns all SSH keys for a team, optionally filtered by
// global status. Carries an extra `globalOnly` flag from a query param so
// it does not fit the generic Index helper; routes wire a small bespoke
// handler.
func (s *Service) ListSSHKeys(ctx context.Context, teamID string, globalOnly bool) ([]dto.SSHKeyResponse, error) {
	var keys []models.SSHKey
	var err error
	if globalOnly {
		keys, err = s.repos.SSHKey().FindGlobalByTeam(ctx, teamID)
	} else {
		keys, err = s.repos.SSHKey().FindByTeam(ctx, teamID)
	}
	if err != nil {
		return nil, err
	}
	out := make([]dto.SSHKeyResponse, len(keys))
	for i := range keys {
		out[i] = dto.ToSSHKeyResponse(&keys[i])
	}
	return out, nil
}

// ListServerSSHKeys returns all SSH keys attached to a server. Signature
// matches IndexNestedFunc.
func (s *Service) ListServerSSHKeys(ctx context.Context, serverID, teamID string) ([]dto.SSHKeyResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	keys, err := s.repos.SSHKey().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SSHKeyResponse, len(keys))
	for i := range keys {
		out[i] = dto.ToSSHKeyResponse(&keys[i])
	}
	return out, nil
}

// CreateSSHKey creates a new SSH key. Signature matches CreateFunc.
func (s *Service) CreateSSHKey(ctx context.Context, teamID, userID string, req *dto.CreateSSHKeyRequest) (dto.SSHKeyResponse, error) {
	key := &models.SSHKey{
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Description: req.Description,
		IsGlobal:    req.IsGlobal,
	}
	key.TeamID = teamID
	key.UserID = userID

	if err := s.repos.SSHKey().Create(ctx, key); err != nil {
		return dto.SSHKeyResponse{}, err
	}

	activity.RecordEvent(ctx, "created", userID, key, "SSH key was created")
	return dto.ToSSHKeyResponse(key), nil
}

// AttachSSHKey attaches an SSH key (by id from the request body) to a
// server. Has its own signature so it does not fit a generic helper.
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

	if err := s.dispatchSSHKeyAddJob(server, key); err != nil {
		s.LogError(err, "Failed to dispatch SSH key add job", "server_id", serverID, "ssh_key_id", sshKeyID)
	}
	return nil
}

// DetachSSHKey detaches an SSH key from a server. Signature matches
// DeleteNestedFunc: (ctx, id=sshKeyID, parentID=serverID, teamID, userID).
func (s *Service) DetachSSHKey(ctx context.Context, sshKeyID, serverID, teamID, userID string) error {
	_ = userID
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

	if err := s.dispatchSSHKeyRemoveJob(server, key); err != nil {
		s.LogError(err, "Failed to dispatch SSH key remove job", "server_id", serverID, "ssh_key_id", sshKeyID)
	}
	return nil
}

// DeleteSSHKey deletes an SSH key. Signature matches DeleteFunc:
// (ctx, id=sshKeyID, teamID, userID).
func (s *Service) DeleteSSHKey(ctx context.Context, sshKeyID, teamID, userID string) error {
	key, err := s.repos.SSHKey().FindByID(ctx, sshKeyID)
	if err != nil {
		return err
	}

	if key.TeamID != teamID {
		return fiberutil.NotFound()
	}

	activity.RecordEvent(ctx, "deleted", userID, key, "SSH key was deleted")
	return s.repos.SSHKey().Delete(ctx, sshKeyID)
}

func (s *Service) dispatchSSHKeyAddJob(server *models.Server, key *models.SSHKey) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewAddSSHKeyTask(server.ID, key.ID)
	})
}

func (s *Service) dispatchSSHKeyRemoveJob(server *models.Server, key *models.SSHKey) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewRemoveSSHKeyTask(server.ID, key.ID, false)
	})
}
