package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// GetLatestMetric returns the latest metric for a server. Signature
// matches IndexNestedFunc and returns the response DTO.
func (s *Service) GetLatestMetric(ctx context.Context, serverID, teamID string) (dto.MetricResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.MetricResponse{}, err
	}
	metric, err := s.repos.Metric().FindLatestByServer(ctx, serverID)
	if err != nil {
		return dto.MetricResponse{}, err
	}
	if metric == nil {
		return dto.MetricResponse{}, fiberutil.NotFound("No metrics found")
	}
	return dto.ToMetricResponse(metric), nil
}

// GetMetrics returns metrics for a server with optional query bounds.
// Has its own signature (limit + time range) so it does not fit the
// generic IndexNested helper and is wired by a small bespoke handler.
func (s *Service) GetMetrics(ctx context.Context, serverID, teamID string, from, to *time.Time, limit int) ([]dto.MetricResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	metrics, err := s.repos.Metric().FindByServer(ctx, serverID, from, to, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.MetricResponse, len(metrics))
	for i := range metrics {
		out[i] = dto.ToMetricResponse(&metrics[i])
	}
	return out, nil
}
