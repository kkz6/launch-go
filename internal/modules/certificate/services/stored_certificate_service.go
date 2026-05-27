package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/certificate/dto"
	"github.com/kkz6/launch-go/internal/modules/certificate/models"
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// StoredCertificateService is the service-layer entry point for the
// certificate module. Create / Update / Delete / Usages live here;
// the parser sub-service does the PEM parsing.
type StoredCertificateService struct {
	repos *repositories.Registry
}

func NewStoredCertificateService(repos *repositories.Registry) *StoredCertificateService {
	return &StoredCertificateService{repos: repos}
}

// ErrDuplicateFingerprint is returned by Create/Update when an alive
// row with the same fingerprint already exists in the team. The
// embedded *models.StoredCertificate is the existing row so handlers
// can surface its name + id to the UI ("Open existing").
type ErrDuplicateFingerprint struct {
	Existing *models.StoredCertificate
}

func (e ErrDuplicateFingerprint) Error() string {
	return "certificate already exists in this team"
}

// Create validates the input, parses metadata from the cert PEM,
// checks fingerprint dedupe, and persists. Returns
// ErrDuplicateFingerprint with the existing row when the cert is
// already saved for the team.
func (s *StoredCertificateService) Create(
	ctx context.Context,
	teamID string,
	userID *string,
	req dto.CreateStoredCertificateRequest,
) (*models.StoredCertificate, error) {
	if err := ValidateKeyMatchesCert(req.Certificate, req.PrivateKey); err != nil {
		return nil, err
	}
	parsed, err := ParseCertificate(req.Certificate)
	if err != nil {
		return nil, err
	}

	// Fingerprint dedupe (team-scoped).
	if existing, lookupErr := s.repos.StoredCertificates.FindByFingerprint(ctx, teamID, parsed.FingerprintSHA256); lookupErr == nil && existing != nil {
		return nil, ErrDuplicateFingerprint{Existing: existing}
	} else if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}

	c := &models.StoredCertificate{
		TeamID:            teamID,
		UserID:            userID,
		Name:              req.Name,
		Notes:             req.Notes,
		Certificate:       req.Certificate,
		PrivateKey:        dbtype.EncryptedString(req.PrivateKey), // encrypts on save
		Domains:           parsed.Domains,
		CommonName:        nilIfEmpty(parsed.CommonName),
		Issuer:            nilIfEmpty(parsed.Issuer),
		NotBefore:         parsed.NotBefore,
		NotAfter:          parsed.NotAfter,
		SerialNumber:      nilIfEmpty(parsed.SerialNumber),
		FingerprintSHA256: nilIfEmpty(parsed.FingerprintSHA256),
	}
	if err := s.repos.StoredCertificates.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
