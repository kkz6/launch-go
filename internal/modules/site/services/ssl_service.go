package services

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// SSLService handles business logic for SSL/TLS
type SSLService struct {
	*BaseService
}

// NewSSLService creates a new SSL service
func NewSSLService(
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
) *SSLService {
	return &SSLService{
		BaseService: NewBaseService(
			siteRepo,
			deploymentRepo,
			certificateRepo,
			queueRepo,
			commandRepo,
			redirectRepo,
			releaseRepo,
			queueClient,
			ws,
			logger,
		),
	}
}

// UpdateSSL updates SSL settings for a site
func (s *SSLService) UpdateSSL(ctx context.Context, siteID, serverID, userID string, req *dto.UpdateSSLRequest) error {
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}

	tlsSetting := enums.TlsSetting(req.TlsSetting)
	if !tlsSetting.IsValid() {
		return errors.New("invalid TLS setting")
	}

	// Handle custom certificate
	if tlsSetting == enums.TlsSettingCustom && req.PrivateKey != nil && req.Certificate != nil {
		// Deactivate existing certificates
		if err := s.certificateRepo.DeactivateAll(ctx, site.ID); err != nil {
			return err
		}

		// Create new certificate
		cert := &models.Certificate{
			SiteID:      site.ID,
			Type:        enums.CertificateTypeCustom,
			PrivateKey:  req.PrivateKey,
			Certificate: req.Certificate,
			IsActive:    true,
		}

		domains := []string{site.Address}
		domains = append(domains, site.GetAliases()...)

		if err := cert.SetDomains(domains); err != nil {
			return err
		}

		now := time.Now()
		cert.UploadedAt = &now

		if err := s.certificateRepo.Create(ctx, cert); err != nil {
			return err
		}

		// Dispatch certificate installation job
		task, err := jobs.NewInstallSSLTask(site.ID, site.Address)
		if err == nil && s.queue != nil {
			s.queue.EnqueueDefault(task)
		}
	}

	// Update TLS setting
	now := time.Now()
	site.TlsSetting = tlsSetting
	site.PendingTlsUpdateSince = &now

	return s.siteRepo.Update(ctx, site)
}

// ListCertificates returns all certificates for a site
func (s *SSLService) ListCertificates(ctx context.Context, siteID, serverID string) ([]models.Certificate, error) {
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.certificateRepo.FindBySite(ctx, siteID)
}
