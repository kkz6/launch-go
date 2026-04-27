package services

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

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

	// Handle custom certificate within a transaction
	if tlsSetting == sitetypes.TLSSettingCustom && req.PrivateKey != nil && req.Certificate != nil {
		if err := s.WithTransaction(ctx, func(tx *gorm.DB) error {
			// Deactivate existing certificates
			if err := tx.Model(&models.Certificate{}).
				Where("site_id = ?", site.ID).
				Update("is_active", false).Error; err != nil {
				return err
			}

			// Create new certificate
			privateKey := dbtype.EncryptedString(*req.PrivateKey)
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

			if err := tx.Create(cert).Error; err != nil {
				return err
			}

			// Update TLS setting within the same transaction
			now2 := time.Now()
			site.TLSSetting = tlsSetting
			site.PendingTLSUpdateSince = &now2

			return tx.Save(site).Error
		}); err != nil {
			return err
		}

		// Dispatch certificate installation job (outside transaction)
		task, err := jobs.NewInstallSSLTask(site.ID, site.Address)
		if err != nil {
			s.LogError(err, "Failed to create install SSL task", "site_id", site.ID)
		} else if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "Failed to enqueue install SSL task", "site_id", site.ID)
		}

		return nil
	}

	// Update TLS setting (non-custom case)
	now := time.Now()
	site.TLSSetting = tlsSetting
	site.PendingTLSUpdateSince = &now

	return s.Repos().Site().Update(ctx, site)
}

// ListCertificates returns all certificates for a site. Signature
// matches IndexDoubleNestedFunc.
func (s *SSLService) ListCertificates(ctx context.Context, siteID, serverID, teamID string) ([]dto.CertificateResponse, error) {
	_ = teamID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}
	certs, err := s.Repos().Certificate().FindBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CertificateResponse, len(certs))
	for i := range certs {
		out[i] = dto.ToCertificateResponse(&certs[i])
	}
	return out, nil
}
