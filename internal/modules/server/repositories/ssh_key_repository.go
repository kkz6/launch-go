package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SSHKeyRepository handles SSH key database operations
type SSHKeyRepository struct {
	repository.Base[models.SSHKey]
}

// NewSSHKeyRepository creates a new SSHKeyRepository instance
func NewSSHKeyRepository(db *gorm.DB) *SSHKeyRepository {
	return &SSHKeyRepository{
		Base: repository.NewBase[models.SSHKey](db),
	}
}

// FindByID finds an SSH key by ID
func (r *SSHKeyRepository) FindByID(ctx context.Context, id string) (*models.SSHKey, error) {
	key, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrSSHKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

// FindByServer finds all SSH keys attached to a server
func (r *SSHKeyRepository) FindByServer(ctx context.Context, serverID string) ([]models.SSHKey, error) {
	var keys []models.SSHKey
	err := r.DB.WithContext(ctx).
		Joins("JOIN server_ssh_keys ON server_ssh_keys.ssh_key_id = ssh_keys.id").
		Where("server_ssh_keys.server_id = ?", serverID).
		Find(&keys).Error
	return keys, err
}

// FindGlobal finds all global SSH keys
func (r *SSHKeyRepository) FindGlobal(ctx context.Context) ([]models.SSHKey, error) {
	var keys []models.SSHKey
	err := r.DB.WithContext(ctx).
		Where("is_global = ?", true).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// AttachToServer attaches an SSH key to a server
func (r *SSHKeyRepository) AttachToServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.DB.WithContext(ctx).Create(&models.ServerSSHKey{
		ServerID: serverID,
		SSHKeyID: sshKeyID,
	}).Error
}

// DetachFromServer detaches an SSH key from a server
func (r *SSHKeyRepository) DetachFromServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.DB.WithContext(ctx).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Delete(&models.ServerSSHKey{}).Error
}

// IsAttachedToServer checks if an SSH key is attached to a server
func (r *SSHKeyRepository) IsAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.ServerSSHKey{}).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Count(&count).Error
	return count > 0, err
}
