package services

import (
	"strings"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// BaseService provides common service dependencies
type BaseService struct {
	service.Base
	siteRepo        *repositories.SiteRepository
	deploymentRepo  *repositories.DeploymentRepository
	certificateRepo *repositories.CertificateRepository
	queueRepo       *repositories.QueueRepository
	commandRepo     *repositories.CommandRepository
	redirectRepo    *repositories.RedirectRepository
	releaseRepo     *repositories.ReleaseRepository
}

// NewBaseService creates a new base service
func NewBaseService(
	siteRepo *repositories.SiteRepository,
	deploymentRepo *repositories.DeploymentRepository,
	certificateRepo *repositories.CertificateRepository,
	queueRepo *repositories.QueueRepository,
	commandRepo *repositories.CommandRepository,
	redirectRepo *repositories.RedirectRepository,
	releaseRepo *repositories.ReleaseRepository,
	queueClient *queue.Client,
	ws *websocket.Hub,
	logger *zerolog.Logger,
) *BaseService {
	return &BaseService{
		Base:            service.NewBase(queueClient, ws, logger),
		siteRepo:        siteRepo,
		deploymentRepo:  deploymentRepo,
		certificateRepo: certificateRepo,
		queueRepo:       queueRepo,
		commandRepo:     commandRepo,
		redirectRepo:    redirectRepo,
		releaseRepo:     releaseRepo,
	}
}

// Helper functions

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	return s
}

func parseMultilineToSlice(s string) []string {
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}
