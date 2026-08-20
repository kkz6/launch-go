package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/certificatecheck"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

// SSLService handles business logic for SSL/TLS.
//
// The stored-cert library lookup is optional: when storedCerts is nil
// (test wiring without the certificate module), the stored-cert branch
// returns an explicit error rather than panicking.
type SSLService struct {
	*BaseService
	storedCerts        *certrepos.StoredCertificateRepository
	certificateChecker certificatecheck.Checker
}

// NewSSLService creates a new SSL service
func NewSSLService(deps *ServiceDeps) *SSLService {
	return &SSLService{
		BaseService:        NewBaseService(deps),
		certificateChecker: certificatecheck.New(),
	}
}

// SetStoredCertificateRepository wires the certificate module's
// repository in so UpdateSSL can resolve a picked stored cert into the
// site's own certificates row. Called once at app boot from
// services.NewServices via ServiceDeps.StoredCerts.
func (s *SSLService) SetStoredCertificateRepository(r *certrepos.StoredCertificateRepository) {
	s.storedCerts = r
}

func (s *SSLService) SetCertificateChecker(checker certificatecheck.Checker) {
	s.certificateChecker = checker
}

func (s *SSLService) CheckCertificate(
	ctx context.Context, siteID, serverID, teamID string,
) (certificatecheck.Result, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return certificatecheck.Result{}, err
	}
	if site.TeamID != teamID {
		return certificatecheck.Result{}, fiberutil.NotFound()
	}
	checkedAt := time.Now().UTC()
	switch site.TLSSetting {
	case sitetypes.TLSSettingOff:
		return certificatecheck.Result{
			Host:      site.Address,
			Status:    certificatecheck.StatusNotIssued,
			Reason:    certificatecheck.ReasonHTTPSDisabled,
			Message:   i18n.TContext(ctx, "Public HTTPS is disabled for this site."),
			CheckedAt: checkedAt,
		}, nil
	case sitetypes.TLSSettingInternal:
		return certificatecheck.Result{
			Host:      site.Address,
			Status:    certificatecheck.StatusInvalid,
			Reason:    certificatecheck.ReasonInternalCA,
			Message:   i18n.TContext(ctx, "This site uses Caddy's internal CA, which is not publicly trusted."),
			CheckedAt: checkedAt,
		}, nil
	}
	return certificatecheck.Localize(ctx, s.certificateChecker.Check(ctx, site.Address)), nil
}

func (s *SSLService) RetryCertificate(
	ctx context.Context, siteID, serverID, teamID, userID string,
) error {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}
	if site.TeamID != teamID {
		return fiberutil.NotFound()
	}
	if site.TLSSetting == sitetypes.TLSSettingOff {
		return fiberutil.BadRequest("Enable SSL before retrying certificate provisioning")
	}
	if site.TLSSetting == sitetypes.TLSSettingInternal {
		return fiberutil.BadRequest("Internal TLS certificates are not issued by a public certificate authority")
	}
	if !site.IsInstalled() {
		return fiberutil.BadRequest("Deploy the site before retrying certificate provisioning")
	}
	if err := ensureSiteConfigurationIdle(site); err != nil {
		return err
	}
	if !s.HasQueue() {
		return pkgservice.ErrQueueRequired
	}
	task, err := jobs.NewCertificateRetryCaddyfileTask(site.ID, stringToPtr(userID))
	if err != nil {
		return fmt.Errorf("build certificate retry task: %w", err)
	}
	if err := s.EnqueueTask(task); err != nil {
		return fmt.Errorf("enqueue certificate retry: %w", err)
	}
	return nil
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
	if err := ensureSiteConfigurationIdle(site); err != nil {
		return err
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
	if !s.HasQueue() {
		return pkgservice.ErrQueueRequired
	}

	var replacementCertificate *models.Certificate
	if req.StoredCertificateID != nil && *req.StoredCertificateID != "" {
		if tlsSetting != sitetypes.TLSSettingCustom {
			return fiberutil.BadRequest("A stored certificate requires the custom TLS setting")
		}
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
		certPEM := stored.Certificate
		storedID := stored.ID
		replacementCertificate = &models.Certificate{
			Type:                sitetypes.CertificateTypeCustom,
			PrivateKey:          stored.PrivateKey,
			Certificate:         &certPEM,
			IsActive:            true,
			StoredCertificateID: &storedID,
		}
	}

	if tlsSetting == sitetypes.TLSSettingCustom && replacementCertificate == nil {
		if req.PrivateKey == nil || req.Certificate == nil ||
			strings.TrimSpace(*req.PrivateKey) == "" ||
			strings.TrimSpace(*req.Certificate) == "" {
			return fiberutil.BadRequest(
				"Custom TLS requires both a private key and certificate",
			)
		}
		replacementCertificate = &models.Certificate{
			Type:        sitetypes.CertificateTypeCustom,
			PrivateKey:  dbtype.EncryptedString(*req.PrivateKey),
			Certificate: req.Certificate,
			IsActive:    true,
		}
	}

	task, err := jobs.NewTLSUpdateCaddyfileTask(site.ID, stringToPtr(userID))
	if err != nil {
		return fmt.Errorf("build TLS update task: %w", err)
	}

	previousCertificates, err := s.Repos().Certificate().FindBySite(ctx, site.ID)
	if err != nil {
		return fmt.Errorf("load current certificates: %w", err)
	}
	activeCertificateIDs := make([]string, 0, len(previousCertificates))
	for i := range previousCertificates {
		if previousCertificates[i].IsActive {
			activeCertificateIDs = append(activeCertificateIDs, previousCertificates[i].ID)
		}
	}

	now := time.Now().UTC()
	if replacementCertificate != nil {
		replacementCertificate.SiteID = site.ID
		replacementCertificate.TeamID = site.TeamID
		replacementCertificate.Domains = append([]string{site.Address}, site.Aliases...)
		replacementCertificate.UploadedAt = &now
	}

	if err := s.WithTransaction(ctx, func(tx *gorm.DB) error {
		reservation := tx.Model(&models.Site{}).
			Where(
				"id = ? AND pending_php_version IS NULL "+
					"AND pending_caddyfile_update_since IS NULL "+
					"AND pending_tls_update_since IS NULL",
				site.ID,
			).
			Updates(map[string]any{
				"tls_setting":                            tlsSetting,
				"pending_tls_update_since":               now,
				"pending_tls_previous_setting":           site.TLSSetting,
				"pending_tls_previous_certificate_ids":   dbtype.JSONStringSlice(activeCertificateIDs),
				"pending_tls_replacement_certificate_id": nil,
			})
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return fiberutil.Conflict("A site configuration update is already in progress")
		}

		if replacementCertificate == nil {
			return nil
		}
		if err := tx.Model(&models.Certificate{}).
			Where("site_id = ?", site.ID).
			Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Create(replacementCertificate).Error; err != nil {
			return err
		}
		return tx.Model(&models.Site{}).
			Where("id = ? AND pending_tls_update_since IS NOT NULL", site.ID).
			Update("pending_tls_replacement_certificate_id", replacementCertificate.ID).
			Error
	}); err != nil {
		return err
	}

	if _, err := s.Queue.EnqueueDefault(task); err != nil {
		rollbackErr := s.rollbackTLSReservation(
			ctx,
			site,
			tlsSetting,
			replacementCertificate,
			activeCertificateIDs,
		)
		if rollbackErr != nil {
			return fmt.Errorf(
				"enqueue TLS update: %w (rollback failed: %v)",
				err,
				rollbackErr,
			)
		}
		return fmt.Errorf("enqueue TLS update: %w", err)
	}

	return nil
}

func (s *SSLService) rollbackTLSReservation(
	ctx context.Context,
	site *models.Site,
	targetTLSSetting sitetypes.TLSSetting,
	replacementCertificate *models.Certificate,
	activeCertificateIDs []string,
) error {
	rollbackCtx, cancelRollback := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelRollback()

	return s.WithTransaction(rollbackCtx, func(tx *gorm.DB) error {
		reservation := tx.Model(&models.Site{}).
			Where(
				"id = ? AND tls_setting = ? "+
					"AND pending_tls_update_since IS NOT NULL "+
					"AND pending_caddyfile_update_since IS NULL "+
					"AND pending_php_version IS NULL",
				site.ID,
				targetTLSSetting,
			).
			Updates(map[string]any{
				"tls_setting":                            site.TLSSetting,
				"pending_tls_update_since":               nil,
				"pending_tls_previous_setting":           nil,
				"pending_tls_previous_certificate_ids":   nil,
				"pending_tls_replacement_certificate_id": nil,
			})
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return errors.New("TLS update reservation was lost")
		}

		if replacementCertificate == nil {
			return nil
		}
		if err := tx.Delete(&models.Certificate{}, "id = ?", replacementCertificate.ID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Certificate{}).
			Where("site_id = ?", site.ID).
			Update("is_active", false).Error; err != nil {
			return err
		}
		if len(activeCertificateIDs) == 0 {
			return nil
		}
		return tx.Model(&models.Certificate{}).
			Where("site_id = ? AND id IN ?", site.ID, activeCertificateIDs).
			Update("is_active", true).Error
	})
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
