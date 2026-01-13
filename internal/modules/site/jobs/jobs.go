// Package jobs provides async job definitions for site operations
package jobs

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Handler handles site job execution
type Handler struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewHandler creates a new site job handler
func NewHandler(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// RegisterHandlers registers all site job handlers
func (h *Handler) RegisterHandlers(mux *asynq.ServeMux) {
	// TODO: Register job handlers
}

// HandleDeploy handles deploy jobs
func (h *Handler) HandleDeploy(ctx context.Context, t *asynq.Task) error {
	// TODO: Implement
	return nil
}

// HandleDeployZeroDowntime handles zero-downtime deploy jobs
func (h *Handler) HandleDeployZeroDowntime(ctx context.Context, t *asynq.Task) error {
	// TODO: Implement
	return nil
}

// HandleRollback handles rollback jobs
func (h *Handler) HandleRollback(ctx context.Context, t *asynq.Task) error {
	// TODO: Implement
	return nil
}

// HandleInstallSSL handles install SSL jobs
func (h *Handler) HandleInstallSSL(ctx context.Context, t *asynq.Task) error {
	// TODO: Implement
	return nil
}

// Task type constants
const (
	TypeDeploy             = "site:deploy"
	TypeDeployZeroDowntime = "site:deploy_zero_downtime"
	TypeRollback           = "site:rollback"
	TypeInstallSSL         = "site:install_ssl"
)

// NewDeployTask creates a deploy job
func NewDeployTask(siteID, deploymentID string, userID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeDeploy, map[string]any{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"user_id":       userID,
	})
}

// NewDeployZeroDowntimeTask creates a zero-downtime deploy job
func NewDeployZeroDowntimeTask(siteID, deploymentID string, userID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeDeployZeroDowntime, map[string]any{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"user_id":       userID,
	})
}

// NewRollbackTask creates a rollback job
func NewRollbackTask(siteID, deploymentID, targetDeploymentID, userID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRollback, map[string]any{
		"site_id":              siteID,
		"deployment_id":        deploymentID,
		"target_deployment_id": targetDeploymentID,
		"user_id":              userID,
	})
}

// NewInstallSSLTask creates an install SSL job
func NewInstallSSLTask(siteID, address string) (*asynq.Task, error) {
	return jobs.NewTask(TypeInstallSSL, map[string]any{
		"site_id": siteID,
		"address": address,
	})
}
