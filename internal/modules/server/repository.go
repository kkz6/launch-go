package server

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, server *Server) error {
	return r.db.WithContext(ctx).Create(server).Error
}

func (r *Repository) FindByID(ctx context.Context, id string) (*Server, error) {
	var server Server
	err := r.db.WithContext(ctx).
		Preload("Sites").
		First(&server, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *Repository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*Server, error) {
	var server Server
	err := r.db.WithContext(ctx).
		Preload("Sites").
		First(&server, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *Repository) FindAllByTeam(ctx context.Context, teamID string) ([]Server, error) {
	var servers []Server
	err := r.db.WithContext(ctx).
		Preload("Sites").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&servers).Error
	return servers, err
}

func (r *Repository) Update(ctx context.Context, server *Server) error {
	return r.db.WithContext(ctx).Save(server).Error
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status ServerStatus) error {
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *Repository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Server{}, "id = ?", id).Error
}

// Database methods
func (r *Repository) CreateDatabase(ctx context.Context, database *Database) error {
	return r.db.WithContext(ctx).Create(database).Error
}

func (r *Repository) FindDatabasesByServer(ctx context.Context, serverID string) ([]Database, error) {
	var databases []Database
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Find(&databases).Error
	return databases, err
}

func (r *Repository) DeleteDatabase(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Database{}, "id = ?", id).Error
}

// SSH Key methods
func (r *Repository) CreateSSHKey(ctx context.Context, key *SSHKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *Repository) FindSSHKeysByServer(ctx context.Context, serverID string) ([]SSHKey, error) {
	var keys []SSHKey
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Find(&keys).Error
	return keys, err
}

func (r *Repository) DeleteSSHKey(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&SSHKey{}, "id = ?", id).Error
}

// Firewall Rule methods
func (r *Repository) CreateFirewallRule(ctx context.Context, rule *FirewallRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *Repository) FindFirewallRulesByServer(ctx context.Context, serverID string) ([]FirewallRule, error) {
	var rules []FirewallRule
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Find(&rules).Error
	return rules, err
}

func (r *Repository) DeleteFirewallRule(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&FirewallRule{}, "id = ?", id).Error
}

// Cron Job methods
func (r *Repository) CreateCronJob(ctx context.Context, job *CronJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *Repository) FindCronJobsByServer(ctx context.Context, serverID string) ([]CronJob, error) {
	var jobs []CronJob
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Find(&jobs).Error
	return jobs, err
}

func (r *Repository) DeleteCronJob(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&CronJob{}, "id = ?", id).Error
}
