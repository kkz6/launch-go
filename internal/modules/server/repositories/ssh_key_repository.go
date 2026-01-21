package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// SSHKeyRepository handles SSH key database operations
type SSHKeyRepository struct {
	repository.Base[models.SshKey]
}

// NewSSHKeyRepository creates a new SSHKeyRepository instance
func NewSSHKeyRepository(db *gorm.DB) *SSHKeyRepository {
	return &SSHKeyRepository{
		Base: repository.NewBase[models.SshKey](db),
	}
}

// Create creates a new SSH key
func (r *SSHKeyRepository) Create(ctx context.Context, key *models.SshKey) error {
	return r.Base.Create(ctx, key)
}

// FindByID finds an SSH key by ID
func (r *SSHKeyRepository) FindByID(ctx context.Context, id string) (*models.SshKey, error) {
	key, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrSSHKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

// FindByTeam finds all SSH keys for a team
func (r *SSHKeyRepository) FindByTeam(ctx context.Context, teamID string) ([]models.SshKey, error) {
	return r.Base.FindByTeam(ctx, teamID)
}

// FindByServer finds all SSH keys attached to a server
func (r *SSHKeyRepository) FindByServer(ctx context.Context, serverID string) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.DB.WithContext(ctx).
		Joins("JOIN server_ssh_keys ON server_ssh_keys.ssh_key_id = ssh_keys.id").
		Where("server_ssh_keys.server_id = ?", serverID).
		Find(&keys).Error
	return keys, err
}

// FindGlobal finds all global SSH keys
func (r *SSHKeyRepository) FindGlobal(ctx context.Context) ([]models.SshKey, error) {
	var keys []models.SshKey
	err := r.DB.WithContext(ctx).
		Where("is_global = ?", true).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// Update updates an SSH key
func (r *SSHKeyRepository) Update(ctx context.Context, key *models.SshKey) error {
	return r.Base.Update(ctx, key)
}

// Delete deletes an SSH key
func (r *SSHKeyRepository) Delete(ctx context.Context, id string) error {
	return r.Base.Delete(ctx, id)
}

// AttachToServer attaches an SSH key to a server
func (r *SSHKeyRepository) AttachToServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.DB.WithContext(ctx).Create(&models.ServerSshKey{
		ServerID: serverID,
		SshKeyID: sshKeyID,
	}).Error
}

// DetachFromServer detaches an SSH key from a server
func (r *SSHKeyRepository) DetachFromServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.DB.WithContext(ctx).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Delete(&models.ServerSshKey{}).Error
}

// IsAttachedToServer checks if an SSH key is attached to a server
func (r *SSHKeyRepository) IsAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.ServerSshKey{}).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Count(&count).Error
	return count > 0, err
}
