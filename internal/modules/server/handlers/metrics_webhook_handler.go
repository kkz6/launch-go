package handlers

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/webhook"
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
	webhook.Base
	repo MetricsWebhookRepository
	hub  broadcast.Broadcaster
}

// NewMetricsWebhookHandler creates a new metrics webhook handler
func NewMetricsWebhookHandler(db *gorm.DB, secretKey string, hub broadcast.Broadcaster, logger *zerolog.Logger) *MetricsWebhookHandler {
	return &MetricsWebhookHandler{
		Base: webhook.NewBase(secretKey, logger),
		repo: &metricsWebhookRepo{db: db},
		hub:  hub,
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
	if !h.VerifySignature(c) {
		return response.Unauthorized(c, "Invalid signature")
	}

	// Parse request body
	req, err := fiberctx.MustParseAndValidate[PulseRequest](c)
	if err != nil {
		h.LogWarn("Failed to parse pulse request", "server_id", serverID)
		return err
	}

	// Validate event type
	if req.Event != "pulse" {
		h.LogWarn("Unknown event type", "event", req.Event, "server_id", serverID)
		return response.Error(c, fiber.StatusBadRequest, "Unknown event type")
	}

	// Find server to get team ID for broadcasting
	server, err := h.repo.FindServerByID(ctx, serverID)
	if err != nil {
		h.LogWarn("Server not found for pulse", "server_id", serverID, "error", err.Error())
		return response.NotFound(c, "Server not found")
	}

	// Parse string values to float64
	diskTotal, err := strconv.ParseFloat(req.Data.DiskTotal, 64)
	if err != nil {
		h.LogWarn("Failed to parse disk_total", "server_id", serverID, "value", req.Data.DiskTotal)
	}
	diskFree, err := strconv.ParseFloat(req.Data.DiskFree, 64)
	if err != nil {
		h.LogWarn("Failed to parse disk_free", "server_id", serverID, "value", req.Data.DiskFree)
	}
	diskUsed, err := strconv.ParseFloat(req.Data.DiskUsed, 64)
	if err != nil {
		h.LogWarn("Failed to parse disk_used", "server_id", serverID, "value", req.Data.DiskUsed)
	}
	memoryTotal, err := strconv.ParseFloat(req.Data.MemoryTotal, 64)
	if err != nil {
		h.LogWarn("Failed to parse memory_total", "server_id", serverID, "value", req.Data.MemoryTotal)
	}
	memoryFree, err := strconv.ParseFloat(req.Data.MemoryFree, 64)
	if err != nil {
		h.LogWarn("Failed to parse memory_free", "server_id", serverID, "value", req.Data.MemoryFree)
	}
	memoryUsed, err := strconv.ParseFloat(req.Data.MemoryUsed, 64)
	if err != nil {
		h.LogWarn("Failed to parse memory_used", "server_id", serverID, "value", req.Data.MemoryUsed)
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
		h.LogError(err, "Failed to save metric", "server_id", serverID)
		return response.InternalError(c, "Failed to save metric")
	}

	h.LogDebug("Pulse received and saved", "server_id", serverID, "load", req.Data.Load)

	// Broadcast to team via WebSocket
	if h.hub != nil {
		payload := broadcast.ServerMetricsPayload{
			ServerID: serverID,
			Load:     req.Data.Load,
			Memory:   broadcast.ResourceMetrics{Total: memoryTotal, Used: memoryUsed, Free: memoryFree},
			Disk:     broadcast.ResourceMetrics{Total: diskTotal, Used: diskUsed, Free: diskFree},
		}
		h.hub.BroadcastToTeam(server.TeamID, broadcast.ServerMetrics, payload.ToMap())
	}

	return response.OK(c, "Pulse received", nil)
}

// GeneratePulseWebhookURL creates a signed URL for the pulse webhook
// This URL is permanent (no expiration) since the agent needs to send metrics indefinitely
func (h *MetricsWebhookHandler) GeneratePulseWebhookURL(baseURL, serverID string) string {
	path := "/webhooks/servers/" + serverID + "/pulse"
	return h.Signer.WithBaseURL(baseURL).PermanentSignedURL(path, nil)
}
