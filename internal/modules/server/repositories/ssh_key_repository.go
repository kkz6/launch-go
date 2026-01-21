package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// SshKeyRepository handles SSH key database operations
type SshKeyRepository struct {
	BaseRepository
}

// NewSshKeyRepository creates a new SshKeyRepository instance
func NewSshKeyRepository(db *gorm.DB) *SshKeyRepository {
	return &SshKeyRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new SSH key
func (r *SshKeyRepository) Create(ctx context.Context, key *models.SshKey) error {
	return r.DB().WithContext(ctx).Create(key).Error
}

// FindByID finds an SSH key by ID
func (r *SshKeyRepository) FindByID(ctx context.Context, id string) (*models.SshKey, error) {
	var key models.SshKey
	err := r.DB().WithContext(ctx).First(&key, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSSHKeyNotFound
		}
		return nil, err
	}
	return &key, nil
}

// FindByTeam finds all SSH keys for a team
func (r *SshKeyRepository) FindByTeam(ctx context.Context, teamID string) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.DB().WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// FindByServer finds all SSH keys attached to a server
func (r *SshKeyRepository) FindByServer(ctx context.Context, serverID string) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.DB().WithContext(ctx).
		Joins("JOIN server_ssh_keys ON server_ssh_keys.ssh_key_id = ssh_keys.id").
		Where("server_ssh_keys.server_id = ?", serverID).
		Find(&keys).Error
	return keys, err
}

// FindGlobal finds all global SSH keys
func (r *SshKeyRepository) FindGlobal(ctx context.Context) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.DB().WithContext(ctx).
		Where("is_global = ?", true).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// Update updates an SSH key
func (r *SshKeyRepository) Update(ctx context.Context, key *models.SshKey) error {
	return r.DB().WithContext(ctx).Save(key).Error
}

// Delete deletes an SSH key
func (r *SshKeyRepository) Delete(ctx context.Context, id string) error {
	return r.DB().WithContext(ctx).Delete(&models.SshKey{}, "id = ?", id).Error
}

// AttachToServer attaches an SSH key to a server
func (r *SshKeyRepository) AttachToServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.DB().WithContext(ctx).Create(&models.ServerSshKey{
		ServerID: serverID,
		SshKeyID: sshKeyID,
	}).Error
}

// DetachFromServer detaches an SSH key from a server
func (r *SshKeyRepository) DetachFromServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.DB().WithContext(ctx).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Delete(&models.ServerSshKey{}).Error
}

// IsAttachedToServer checks if an SSH key is attached to a server
func (r *SshKeyRepository) IsAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error) {
	var count int64
	err := r.DB().WithContext(ctx).
		Model(&models.ServerSshKey{}).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Count(&count).Error
	return count > 0, err
}
