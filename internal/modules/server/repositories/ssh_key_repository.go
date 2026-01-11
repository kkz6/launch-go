package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateSshKey creates a new SSH key
func (r *Repository) CreateSshKey(ctx context.Context, key *models.SshKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// FindSshKeyByID finds an SSH key by ID
func (r *Repository) FindSshKeyByID(ctx context.Context, id string) (*models.SshKey, error) {
	var key models.SshKey
	err := r.db.WithContext(ctx).First(&key, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSshKeyNotFound
		}

		return nil, err
	}

	return &key, nil
}

// FindSshKeysByTeam finds all SSH keys for a team
func (r *Repository) FindSshKeysByTeam(ctx context.Context, teamID string) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.db.WithContext(ctx).
		Where("team_id = ? OR is_global = ?", teamID, true).
		Order("created_at DESC").
		Find(&keys).Error

	return keys, err
}

// FindSshKeysByServer finds all SSH keys attached to a server
func (r *Repository) FindSshKeysByServer(ctx context.Context, serverID string) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.db.WithContext(ctx).
		Joins("JOIN server_ssh_keys ON server_ssh_keys.ssh_key_id = ssh_keys.id").
		Where("server_ssh_keys.server_id = ?", serverID).
		Find(&keys).Error

	return keys, err
}

// FindGlobalSshKeys finds all global SSH keys
func (r *Repository) FindGlobalSshKeys(ctx context.Context) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.db.WithContext(ctx).
		Where("is_global = ?", true).
		Order("created_at DESC").
		Find(&keys).Error

	return keys, err
}

// UpdateSshKey updates an SSH key
func (r *Repository) UpdateSshKey(ctx context.Context, key *models.SshKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

// DeleteSshKey deletes an SSH key
func (r *Repository) DeleteSshKey(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.SshKey{}, "id = ?", id).Error
}

// AttachSshKeyToServer attaches an SSH key to a server
func (r *Repository) AttachSshKeyToServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.db.WithContext(ctx).Create(&models.ServerSshKey{
		ServerID: serverID,
		SshKeyID: sshKeyID,
	}).Error
}

// DetachSshKeyFromServer detaches an SSH key from a server
func (r *Repository) DetachSshKeyFromServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.db.WithContext(ctx).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Delete(&models.ServerSshKey{}).Error
}

// IsSshKeyAttachedToServer checks if an SSH key is attached to a server
func (r *Repository) IsSshKeyAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ServerSshKey{}).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Count(&count).Error

	return count > 0, err
}
