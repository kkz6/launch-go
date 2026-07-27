package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SSLService handles business logic for SSL/TLS.
//
// The stored-cert library lookup is optional: when storedCerts is nil
// (test wiring without the certificate module), the stored-cert branch
// returns an explicit error rather than panicking.
type SSLService struct {
	*BaseService
	storedCerts *certrepos.StoredCertificateRepository
}

// NewSSLService creates a new SSL service
func NewSSLService(deps *ServiceDeps) *SSLService {
	return &SSLService{
		BaseService: NewBaseService(deps),
	}
}

// SetStoredCertificateRepository wires the certificate module's
// repository in so UpdateSSL can resolve a picked stored cert into the
// site's own certificates row. Called once at app boot from
// services.NewServices via ServiceDeps.StoredCerts.
func (s *SSLService) SetStoredCertificateRepository(r *certrepos.StoredCertificateRepository) {
	s.storedCerts = r
}

// UpdateSSL updates SSL settings for a site
func (s *SSLService) UpdateSSL(ctx context.Context, siteID, serverID, teamID, userID string, req *dto.UpdateSSLRequest) error {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}
	if site.TeamID != teamID {
		return fiberutil.NotFound()
	}

	// `stored` is a client-side label only; on the server it folds
	// into the existing `custom` path with the PEMs resolved from the
	// stored_certificates row.
	if req.TLSSetting == "stored" {
		req.TLSSetting = string(sitetypes.TLSSettingCustom)
	}

	tlsSetting := sitetypes.TLSSetting(req.TLSSetting)
	if !tlsSetting.IsValid() {
		return errors.New("invalid TLS setting")
	}

	// Handle stored-cert path: user picked from their team library
	// instead of pasting PEMs inline. Resolve the cert and fall through
	// the same write path as a custom inline paste, with the FK back to
	// the library row recorded on the resulting certificates row.
	if req.StoredCertificateID != nil && *req.StoredCertificateID != "" {
		if s.storedCerts == nil {
			return errors.New("stored certificate library is not configured")
		}
		stored, err := s.storedCerts.FindByID(ctx, site.TeamID, *req.StoredCertificateID)
		if err != nil {
			return err
		}
		if stored.NotAfter.Before(time.Now()) {
			return fmt.Errorf(
				"stored certificate %q expired on %s",
				stored.Name,
				stored.NotAfter.UTC().Format(time.DateOnly),
			)
		}

		if err := s.WithTransaction(ctx, func(tx *gorm.DB) error {
			if err := tx.Model(&models.Certificate{}).
				Where("site_id = ?", site.ID).
				Update("is_active", false).Error; err != nil {
				return err
			}

			certPEM := stored.Certificate
			storedID := stored.ID
			cert := &models.Certificate{
				Type:                sitetypes.CertificateTypeCustom,
				PrivateKey:          stored.PrivateKey, // decrypted-on-Scan, re-encrypted-on-Value
				Certificate:         &certPEM,
				IsActive:            true,
				StoredCertificateID: &storedID,
			}
			cert.SiteID = site.ID
			cert.TeamID = site.TeamID
			cert.Domains = append([]string{site.Address}, site.Aliases...)
			now := time.Now()
			cert.UploadedAt = &now

			if err := tx.Create(cert).Error; err != nil {
				return err
			}

			site.TLSSetting = tlsSetting
			now2 := time.Now()
			site.PendingTLSUpdateSince = &now2
			return tx.Save(site).Error
		}); err != nil {
			return err
		}

		task, err := jobs.NewInstallSSLTask(site.ID, site.Address)
		if err != nil {
			s.LogError(err, "Failed to create install SSL task", "site_id", site.ID)
		} else if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "Failed to enqueue install SSL task", "site_id", site.ID)
		}
		return nil
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
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}
	if site.TeamID != teamID {
		return nil, fiberutil.NotFound()
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
