package services

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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
//
// Callers should use errors.As(err, &target) to access .Existing;
// errors.Is is not meaningful here (the error carries per-call state).
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
		// Translate a PG unique-violation on the fingerprint partial
		// index into ErrDuplicateFingerprint. This closes the TOCTOU
		// between FindByFingerprint and Create: two concurrent
		// uploads of the same cert can both pass the pre-check, and
		// the second insert then races into 23505. Re-run the
		// fingerprint lookup so the caller still gets a structured
		// 409 with the existing row, not a raw DB error that the
		// handler would map to 500.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_stored_certs_team_fingerprint_alive" {
			if existing, lookupErr := s.repos.StoredCertificates.FindByFingerprint(ctx, teamID, parsed.FingerprintSHA256); lookupErr == nil && existing != nil {
				return nil, ErrDuplicateFingerprint{Existing: existing}
			}
		}
		return nil, err
	}
	return c, nil
}

// Usages returns the resources (sites + docker domains) that
// currently reference the stored cert. Used by the /usages endpoint
// and by Delete to decide whether to short-circuit into ErrInUse.
func (s *StoredCertificateService) Usages(ctx context.Context, teamID, certID string) ([]dto.CertificateUsage, error) {
	return s.repos.StoredCertificates.Usages(ctx, teamID, certID)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
