package handlers

import (
	"context"
	"math"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/metrics"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// MetricsWebhookRepository interface for metrics webhook handler
type MetricsWebhookRepository interface {
	FindServerByID(ctx context.Context, id string) (*models.Server, error)
	CreateMetric(ctx context.Context, metric *models.Metric) error
	UpdateServerFields(ctx context.Context, serverID string, fields map[string]any) error
}

// metricsWebhookRepo implements MetricsWebhookRepository
type metricsWebhookRepo struct {
	db *gorm.DB
}

func (r *metricsWebhookRepo) FindServerByID(ctx context.Context, id string) (*models.Server, error) {
	var server models.Server
	err := r.db.WithContext(ctx).First(&server, "id = ?", id).Error
	return &server, err
}

func (r *metricsWebhookRepo) CreateMetric(ctx context.Context, metric *models.Metric) error {
	return r.db.WithContext(ctx).Create(metric).Error
}

func (r *metricsWebhookRepo) UpdateServerFields(ctx context.Context, serverID string, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&models.Server{}).Where("id = ?", serverID).Updates(fields).Error
}

// MetricsWebhookHandler handles incoming metrics from launch-agent
type MetricsWebhookHandler struct {
	signer *signedurl.Signer
	logger *zerolog.Logger
	repo   MetricsWebhookRepository
	hub    broadcast.Broadcaster
}

// NewMetricsWebhookHandler creates a new metrics webhook handler
func NewMetricsWebhookHandler(db *gorm.DB, secretKey string, hub broadcast.Broadcaster, logger *zerolog.Logger) *MetricsWebhookHandler {
	return &MetricsWebhookHandler{
		signer: signedurl.NewSigner(secretKey),
		logger: logger,
		repo:   &metricsWebhookRepo{db: db},
		hub:    hub,
	}
}

// PulseRequest represents the incoming pulse data from launch-agent
type PulseRequest struct {
	Event string    `json:"event"`
	Data  PulseData `json:"data"`
}

// PulseData represents the metrics data from launch-agent
type PulseData struct {
	Load        float64 `json:"Load"`
	DiskTotal   string  `json:"DiskTotal"`
	DiskFree    string  `json:"DiskFree"`
	DiskUsed    string  `json:"DiskUsed"`
	MemoryTotal string  `json:"MemoryTotal"`
	MemoryFree  string  `json:"MemoryFree"`
	MemoryUsed  string  `json:"MemoryUsed"`
}

// ReceivePulse handles incoming pulse/metrics data from launch-agent
func (h *MetricsWebhookHandler) ReceivePulse(c *fiber.Ctx) error {
	serverID := c.Params("id")
	ctx := c.Context()

	// Verify signed URL
	if !signedurl.ValidateSignedURL(c, h.signer) {
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	// Parse request body
	req, err := fiberctx.MustParseAndValidate[PulseRequest](c)
	if err != nil {
		h.logger.Warn().Str("server_id", serverID).Msg("Failed to parse pulse request")
		return err
	}

	// Validate event type
	if req.Event != "pulse" {
		h.logger.Warn().Str("event", req.Event).Str("server_id", serverID).Msg("Unknown event type")
		return fiberctx.Error(c, fiber.StatusBadRequest, "Unknown event type")
	}

	// Find server to get team ID for broadcasting
	server, err := h.repo.FindServerByID(ctx, serverID)
	if err != nil {
		h.logger.Warn().Str("server_id", serverID).Str("error", err.Error()).Msg("Server not found for pulse")
		return fiberctx.RespondNotFound(c, "Server not found")
	}

	// Parse string values to float64 using metrics parser
	parser := metrics.NewParser(*h.logger, serverID)

	// Create metric record
	metric := &models.Metric{
		ServerID:    serverID,
		Load:        req.Data.Load,
		DiskTotal:   parser.ParseFloat(req.Data.DiskTotal, "disk_total"),
		DiskFree:    parser.ParseFloat(req.Data.DiskFree, "disk_free"),
		DiskUsed:    parser.ParseFloat(req.Data.DiskUsed, "disk_used"),
		MemoryTotal: parser.ParseFloat(req.Data.MemoryTotal, "memory_total"),
		MemoryFree:  parser.ParseFloat(req.Data.MemoryFree, "memory_free"),
		MemoryUsed:  parser.ParseFloat(req.Data.MemoryUsed, "memory_used"),
	}

	if err := h.repo.CreateMetric(ctx, metric); err != nil {
		h.logger.Error().Err(err).Str("server_id", serverID).Msg("Failed to save metric")
		return fiberctx.RespondInternalError(c, "Failed to save metric")
	}

	h.logger.Debug().Str("server_id", serverID).Float64("load", req.Data.Load).Msg("Pulse received and saved")

	// Backfill server hardware details from metrics if missing
	h.backfillServerDetails(ctx, server, metric)

	// Broadcast to team via WebSocket
	if h.hub != nil {
		payload := broadcast.ServerMetricsPayload{
			ServerID: serverID,
			Load:     req.Data.Load,
			Memory:   broadcast.ResourceMetrics{Total: metric.MemoryTotal, Used: metric.MemoryUsed, Free: metric.MemoryFree},
			Disk:     broadcast.ResourceMetrics{Total: metric.DiskTotal, Used: metric.DiskUsed, Free: metric.DiskFree},
		}
		h.hub.BroadcastToTeam(server.TeamID, broadcast.ServerMetrics, payload.ToMap())
	}

	return fiberctx.OK(c, "Pulse received", nil)
}

// backfillServerDetails updates server hardware fields from metric data if they are missing
func (h *MetricsWebhookHandler) backfillServerDetails(ctx context.Context, server *models.Server, metric *models.Metric) {
	updates := make(map[string]any)

	if server.MemoryInMB == nil && metric.MemoryTotal > 0 {
		memMB := int(math.Round(metric.MemoryTotal / (1024 * 1024)))
		updates["memory_in_mb"] = memMB
	}

	if server.StorageInGB == nil && metric.DiskTotal > 0 {
		diskGB := int(math.Round(metric.DiskTotal / (1024 * 1024 * 1024)))
		updates["storage_in_gb"] = diskGB
	}

	if len(updates) == 0 {
		return
	}

	if err := h.repo.UpdateServerFields(ctx, server.ID, updates); err != nil {
		h.logger.Error().Err(err).Str("server_id", server.ID).Msg("Failed to backfill server details from metrics")
	}
}

// GeneratePulseWebhookURL creates a signed URL for the pulse webhook
// This URL is permanent (no expiration) since the agent needs to send metrics indefinitely
func (h *MetricsWebhookHandler) GeneratePulseWebhookURL(baseURL, serverID string) string {
	path := "/webhooks/servers/" + serverID + "/pulse"
	return h.signer.WithBaseURL(baseURL).PermanentSignedURL(path, nil)
}
