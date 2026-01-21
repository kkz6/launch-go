package handlers

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

// MetricsWebhookRepository interface for metrics webhook handler
type MetricsWebhookRepository interface {
	FindServerByID(ctx context.Context, id string) (*models.Server, error)
	CreateMetric(ctx context.Context, metric *models.Metric) error
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

// MetricsWebhookHandler handles incoming metrics from launch-agent
type MetricsWebhookHandler struct {
	repo   MetricsWebhookRepository
	signer *signedurl.Signer
	hub    ws.Broadcaster
	logger *zerolog.Logger
}

// NewMetricsWebhookHandler creates a new metrics webhook handler
func NewMetricsWebhookHandler(db *gorm.DB, secretKey string, hub ws.Broadcaster, logger *zerolog.Logger) *MetricsWebhookHandler {
	return &MetricsWebhookHandler{
		repo:   &metricsWebhookRepo{db: db},
		signer: signedurl.NewSigner(secretKey),
		hub:    hub,
		logger: logger,
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
	if !h.verifySignature(c) {
		return response.Unauthorized(c, "Invalid signature")
	}

	// Parse request body
	var req PulseRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Msg("Failed to parse pulse request")
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Validate event type
	if req.Event != "pulse" {
		h.logger.Warn().Str("event", req.Event).Str("server_id", serverID).Msg("Unknown event type")
		return response.Error(c, fiber.StatusBadRequest, "Unknown event type")
	}

	// Find server to get team ID for broadcasting
	server, err := h.repo.FindServerByID(ctx, serverID)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Msg("Server not found for pulse")
		return response.NotFound(c, "Server not found")
	}

	// Parse string values to float64
	diskTotal, err := strconv.ParseFloat(req.Data.DiskTotal, 64)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskTotal).Msg("Failed to parse disk_total")
	}
	diskFree, err := strconv.ParseFloat(req.Data.DiskFree, 64)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskFree).Msg("Failed to parse disk_free")
	}
	diskUsed, err := strconv.ParseFloat(req.Data.DiskUsed, 64)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskUsed).Msg("Failed to parse disk_used")
	}
	memoryTotal, err := strconv.ParseFloat(req.Data.MemoryTotal, 64)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.MemoryTotal).Msg("Failed to parse memory_total")
	}
	memoryFree, err := strconv.ParseFloat(req.Data.MemoryFree, 64)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.MemoryFree).Msg("Failed to parse memory_free")
	}
	memoryUsed, err := strconv.ParseFloat(req.Data.MemoryUsed, 64)
	if err != nil {
		h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.MemoryUsed).Msg("Failed to parse memory_used")
	}

	// Create metric record
	metric := &models.Metric{
		ServerID:    serverID,
		Load:        req.Data.Load,
		DiskTotal:   diskTotal,
		DiskFree:    diskFree,
		DiskUsed:    diskUsed,
		MemoryTotal: memoryTotal,
		MemoryFree:  memoryFree,
		MemoryUsed:  memoryUsed,
	}

	if err := h.repo.CreateMetric(ctx, metric); err != nil {
		h.logger.Error().Err(err).Str("server_id", serverID).Msg("Failed to save metric")
		return response.InternalError(c, "Failed to save metric")
	}

	h.logger.Debug().
		Str("server_id", serverID).
		Float64("load", req.Data.Load).
		Msg("Pulse received and saved")

	// Broadcast to team via WebSocket
	if h.hub != nil {
		h.hub.BroadcastToTeam(server.TeamID, "server.metrics", map[string]interface{}{
			"server_id": serverID,
			"load":      req.Data.Load,
			"memory": map[string]interface{}{
				"total": memoryTotal,
				"used":  memoryUsed,
				"free":  memoryFree,
			},
			"disk": map[string]interface{}{
				"total": diskTotal,
				"used":  diskUsed,
				"free":  diskFree,
			},
		})
	}

	return response.OK(c, "Pulse received", nil)
}

// verifySignature validates the webhook signature using signedurl package
func (h *MetricsWebhookHandler) verifySignature(c *fiber.Ctx) bool {
	return signedurl.ValidateSignedURL(c, h.signer)
}

// GeneratePulseWebhookURL creates a signed URL for the pulse webhook
// This URL is permanent (no expiration) since the agent needs to send metrics indefinitely
func (h *MetricsWebhookHandler) GeneratePulseWebhookURL(baseURL, serverID string) string {
	path := "/webhooks/servers/" + serverID + "/pulse"
	return h.signer.WithBaseURL(baseURL).PermanentSignedURL(path, nil)
}
