package repositories

import "context"

// ServerResourceCounts supplies the same resource guards used by server deletion.
func (r *Registry) ServerResourceCounts(ctx context.Context, serverID string) (sites, projects int64, err error) {
	if err = r.db.WithContext(ctx).Table("sites").Where("server_id = ?", serverID).Count(&sites).Error; err != nil {
		return
	}
	err = r.db.WithContext(ctx).Table("docker_projects").Where("server_id = ? AND deleted_at IS NULL", serverID).Count(&projects).Error
	return
}
