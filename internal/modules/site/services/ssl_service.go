package services

import (
	"context"
	"errors"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// SSLService handles business logic for SSL/TLS
type SSLService struct {
	*BaseService
}

// NewSSLService creates a new SSL service
func NewSSLService(deps *ServiceDeps) *SSLService {
	return &SSLService{
		BaseService: NewBaseService(deps),
	}
}

// UpdateSSL updates SSL settings for a site
func (s *SSLService) UpdateSSL(ctx context.Context, siteID, serverID, userID string, req *dto.UpdateSSLRequest) error {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}

	tlsSetting := sitetypes.TLSSetting(req.TLSSetting)
	if !tlsSetting.IsValid() {
		return errors.New("invalid TLS setting")
	}

	// Handle custom certificate
	if tlsSetting == sitetypes.TLSSettingCustom && req.PrivateKey != nil && req.Certificate != nil {
		// Deactivate existing certificates
		if err := s.Repos().Certificate().DeactivateAll(ctx, site.ID); err != nil {
			return err
		}

		// Create new certificate
		privateKey := dbtype.EncryptedString("")
		if req.PrivateKey != nil {
			privateKey = dbtype.EncryptedString(*req.PrivateKey)
		}
		cert := &models.Certificate{
			Type:        sitetypes.CertificateTypeCustom,
			PrivateKey:  privateKey,
			Certificate: req.Certificate,
			IsActive:    true,
		}
		cert.SiteID = site.ID
		cert.TeamID = site.TeamID

		cert.Domains = append([]string{site.Address}, site.Aliases...)

		now := time.Now()
		cert.UploadedAt = &now

		if err := s.Repos().Certificate().Create(ctx, cert); err != nil {
			return err
		}

		// Dispatch certificate installation job
		task, err := jobs.NewInstallSSLTask(site.ID, site.Address)
		if err == nil {
			s.EnqueueTask(task)
		}
	}

	// Update TLS setting
	now := time.Now()
	site.TLSSetting = tlsSetting
	site.PendingTLSUpdateSince = &now

	return s.Repos().Site().Update(ctx, site)
}

// ListCertificates returns all certificates for a site
func (s *SSLService) ListCertificates(ctx context.Context, siteID, serverID string) ([]models.Certificate, error) {
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.Repos().Certificate().FindBySite(ctx, siteID)
}
