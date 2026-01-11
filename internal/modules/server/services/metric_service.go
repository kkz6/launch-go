package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// GetLatestMetric returns the latest metric for a server
func (s *Service) GetLatestMetric(ctx context.Context, serverID, teamID string) (*models.Metric, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindLatestMetricByServer(ctx, serverID)
}

// GetMetrics returns metrics for a server
func (s *Service) GetMetrics(ctx context.Context, serverID, teamID string, from, to *time.Time, limit int) ([]models.Metric, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindMetricsByServer(ctx, serverID, from, to, limit)
}
