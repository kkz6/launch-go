package services

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/certificate/dto"
	certjobs "github.com/kkz6/launch-go/internal/modules/certificate/jobs"
	"github.com/kkz6/launch-go/internal/modules/certificate/models"
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// FanoutDispatcher is the narrow interface the service uses to enqueue
// the certificate:fanout task after a content-change Update. Defined
// here (not imported from the jobs package) to keep the service ←
// jobs dependency direction one-way (jobs depends on service via the
// repo, never the other way).
type FanoutDispatcher interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// StoredCertificateService is the service-layer entry point for the
// certificate module. Create / Update / Delete / Usages live here;
// the parser sub-service does the PEM parsing.
type StoredCertificateService struct {
	repos *repositories.Registry
	// queue is the asynq client used to enqueue the certificate:fanout
	// task. Nil-safe: when not wired (e.g. test boots without a queue),
	// Update silently skips the dispatch and the caller is informed via
	// the returned pending_redeploys count.
	queue *queue.Client
}

func NewStoredCertificateService(repos *repositories.Registry) *StoredCertificateService {
	return &StoredCertificateService{repos: repos}
}

// SetQueue wires the asynq client used for fanout-on-content-change.
// Called once at boot from the module's constructor.
func (s *StoredCertificateService) SetQueue(q *queue.Client) {
	s.queue = q
}

// List returns all alive stored certs for the team, ordered by
// not_after ASC (soonest expiry first — matches the picker UX).
func (s *StoredCertificateService) List(ctx context.Context, teamID string) ([]models.StoredCertificate, error) {
	return s.repos.StoredCertificates.List(ctx, teamID)
}

// Get returns a single stored cert by id within the team. Returns
// gorm.ErrRecordNotFound (caught by the handler as 404) when missing.
func (s *StoredCertificateService) Get(ctx context.Context, teamID, id string) (*models.StoredCertificate, error) {
	return s.repos.StoredCertificates.FindByID(ctx, teamID, id)
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

// ErrPartialCertKeyUpdate is returned by Update when the request
// changes exactly one of (Certificate, PrivateKey). The pair must
// be updated together to preserve the match — accepting a partial
// update would leave the row with a key that no longer signs the
// cert. Handlers should map this to a 422.
var ErrPartialCertKeyUpdate = errors.New("certificate and private_key must be updated together")

// ErrInUse is returned by Delete when the cert is still referenced
// by at least one site or docker domain. Carries the usage list so
// the handler can prompt for confirmation ("used by 3 sites — delete
// anyway?") and the caller can decide whether to switch to
// DeleteWithForce.
type ErrInUse struct {
	Usages []dto.CertificateUsage
}

func (e ErrInUse) Error() string {
	return "certificate is still in use"
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
	// Parse the cert FIRST so a malformed PEM surfaces
	// ErrInvalidCertificatePEM rather than the less specific
	// ErrPrivateKeyMismatch that tls.X509KeyPair would emit when it
	// can't find PEM data in the cert blob.
	parsed, err := ParseCertificate(req.Certificate)
	if err != nil {
		return nil, err
	}
	if err := ValidateKeyMatchesCert(req.Certificate, req.PrivateKey); err != nil {
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

// Update applies the request to an existing stored cert. Name/notes
// can be updated independently. The cert/key pair must be updated
// together — supplying only one side is ErrPartialCertKeyUpdate.
//
// When the cert content changes the parsed metadata (domains, dates,
// fingerprint, issuer, serial, common name) is refreshed in lock-step
// so the picker shows the new shape on the next list call. A
// fingerprint dedupe runs on content changes (excluding the current
// row), and the same TOCTOU window handled by Create is closed by the
// partial unique DB index in production.
//
// Returns the updated row plus a `pending_redeploys` count: the
// number of resources currently pointing at this cert. The count is
// 0 when only name/notes change. In Phase 6 the API handler will use
// this number to fan out site:install_ssl / docker:redeploy jobs;
// for now the service just reports the count.
func (s *StoredCertificateService) Update(
	ctx context.Context,
	teamID, id string,
	req dto.UpdateStoredCertificateRequest,
) (*models.StoredCertificate, int, error) {
	c, err := s.repos.StoredCertificates.FindByID(ctx, teamID, id)
	if err != nil {
		return nil, 0, err
	}

	contentChange, err := normalizeContentChange(req.Certificate, req.PrivateKey)
	if err != nil {
		return nil, 0, err
	}

	// No-op content guard: if the caller resent the cert + key that
	// are byte-identical to what we already have on the row, treat
	// this as a metadata-only Update. Without this, every PATCH that
	// re-sends the unchanged content would re-parse, re-dedupe,
	// rewrite the metadata columns, and (worst of all) report a non-
	// zero pending_redeploys, which Phase 6 will turn into a fan-out
	// of spurious site:install_ssl / docker:redeploy jobs.
	//
	// EncryptedString decrypts on Scan, so string(c.PrivateKey) here
	// is the plaintext PEM already in memory — direct comparison is
	// correct.
	if contentChange != nil &&
		contentChange.cert == c.Certificate &&
		contentChange.key == string(c.PrivateKey) {
		contentChange = nil
	}

	if req.Name != nil {
		c.Name = *req.Name
	}
	if req.Notes != nil {
		c.Notes = req.Notes
	}

	if contentChange != nil {
		// Parse first so malformed PEM surfaces ErrInvalidCertificatePEM
		// rather than the less-specific ErrPrivateKeyMismatch that
		// X509KeyPair would emit on a missing cert block.
		parsed, err := ParseCertificate(contentChange.cert)
		if err != nil {
			return nil, 0, err
		}
		if err := ValidateKeyMatchesCert(contentChange.cert, contentChange.key); err != nil {
			return nil, 0, err
		}

		// Fingerprint dedupe — only collides if the matching alive
		// row is a DIFFERENT cert in the same team. The current row
		// is excluded by id; the partial-unique DB index will still
		// catch the rare TOCTOU race between this check and Save.
		if existing, lookupErr := s.repos.StoredCertificates.FindByFingerprint(ctx, teamID, parsed.FingerprintSHA256); lookupErr == nil && existing != nil {
			if existing.ID != c.ID {
				return nil, 0, ErrDuplicateFingerprint{Existing: existing}
			}
		} else if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return nil, 0, lookupErr
		}

		c.Certificate = contentChange.cert
		c.PrivateKey = dbtype.EncryptedString(contentChange.key)
		c.Domains = parsed.Domains
		c.CommonName = nilIfEmpty(parsed.CommonName)
		c.Issuer = nilIfEmpty(parsed.Issuer)
		c.NotBefore = parsed.NotBefore
		c.NotAfter = parsed.NotAfter
		c.SerialNumber = nilIfEmpty(parsed.SerialNumber)
		c.FingerprintSHA256 = nilIfEmpty(parsed.FingerprintSHA256)
	}

	if err := s.repos.StoredCertificates.Update(ctx, c); err != nil {
		// Mirror Create's TOCTOU translation: a concurrent Update
		// could land between the dedupe pre-check and Save, tripping
		// the partial unique index. Surface the structured error so
		// the handler can show "this cert already exists" instead of
		// a 500.
		var pgErr *pgconn.PgError
		if contentChange != nil && errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_stored_certs_team_fingerprint_alive" {
			if fp := c.FingerprintSHA256; fp != nil {
				if existing, lookupErr := s.repos.StoredCertificates.FindByFingerprint(ctx, teamID, *fp); lookupErr == nil && existing != nil && existing.ID != c.ID {
					return nil, 0, ErrDuplicateFingerprint{Existing: existing}
				}
			}
		}
		return nil, 0, err
	}

	pendingRedeploys := 0
	if contentChange != nil {
		usages, err := s.repos.StoredCertificates.Usages(ctx, teamID, c.ID)
		if err != nil {
			return nil, 0, err
		}
		pendingRedeploys = len(usages)

		// Dispatch the fanout task — the worker walks the same usage
		// list (re-fetched, so a domain added/removed between this
		// call and the job firing is correctly included/excluded) and
		// enqueues install_ssl per site + sync_traefik_config per
		// docker resource. Nil-safe when the queue isn't wired (tests).
		if s.queue != nil && pendingRedeploys > 0 {
			task, taskErr := certjobs.NewFanoutCertificateTask(c.ID, teamID)
			if taskErr == nil {
				_, _ = s.queue.Enqueue(task)
			}
		}
	}

	return c, pendingRedeploys, nil
}

// certContentChange bundles the new cert + key when both are present
// on an Update request. nil means the caller isn't touching the
// content (name/notes only). All-or-nothing is enforced by
// normalizeContentChange.
type certContentChange struct {
	cert string
	key  string
}

// normalizeContentChange enforces the rule that cert and key must
// change together. Returns nil when neither is supplied.
func normalizeContentChange(certPtr, keyPtr *string) (*certContentChange, error) {
	if certPtr == nil && keyPtr == nil {
		return nil, nil
	}
	if certPtr == nil || keyPtr == nil {
		return nil, ErrPartialCertKeyUpdate
	}
	return &certContentChange{cert: *certPtr, key: *keyPtr}, nil
}

// Usages returns the resources (sites + docker domains) that
// currently reference the stored cert. Used by the /usages endpoint
// and by Delete to decide whether to short-circuit into ErrInUse.
//
// Returns gorm.ErrRecordNotFound (caught by the handler as 404) when
// the cert id doesn't exist in the team — without this pre-check the
// endpoint would return 200 with [] for unknown ids, letting the UI
// act on stale state.
func (s *StoredCertificateService) Usages(ctx context.Context, teamID, certID string) ([]dto.CertificateUsage, error) {
	if _, err := s.repos.StoredCertificates.FindByID(ctx, teamID, certID); err != nil {
		return nil, err
	}
	return s.repos.StoredCertificates.Usages(ctx, teamID, certID)
}

// Delete soft-deletes the stored cert if nothing references it.
// Returns ErrInUse (with the populated usage list) when at least one
// site or docker domain still points at it — the caller should
// prompt the user and call DeleteWithForce to cascade-clear the
// references.
func (s *StoredCertificateService) Delete(ctx context.Context, teamID, id string) error {
	if _, err := s.repos.StoredCertificates.FindByID(ctx, teamID, id); err != nil {
		return err // propagates gorm.ErrRecordNotFound to the handler
	}
	usages, err := s.repos.StoredCertificates.Usages(ctx, teamID, id)
	if err != nil {
		return err
	}
	if len(usages) > 0 {
		return ErrInUse{Usages: usages}
	}
	return s.repos.StoredCertificates.SoftDelete(ctx, teamID, id)
}

// DeleteWithForce clears every reference to the cert across the
// site + docker domain tables and then soft-deletes the cert itself.
// All three steps run in a single transaction so a partial cascade
// can't leave a dangling stored_certificate_id pointing at a
// soft-deleted row.
//
// Behaviour per referencing kind:
//   - `certificates` rows have their stored_certificate_id NULLed.
//     The parent `sites` row's tls_setting is reset to 'auto' so the
//     next deploy installs Let's Encrypt. (tls_setting lives on
//     sites, NOT certificates — verified against the live schema.)
//   - `docker_application_domains` rows have stored_certificate_id
//     NULLed and certificate_provider reset to 'letsencrypt'.
//
// The team_id check is enforced on the link table for the site path
// (certificates.team_id) and via the parent table join for the
// docker path (docker_application_domains has no team_id of its own).
func (s *StoredCertificateService) DeleteWithForce(ctx context.Context, teamID, id string) error {
	if _, err := s.repos.StoredCertificates.FindByID(ctx, teamID, id); err != nil {
		return err // propagates gorm.ErrRecordNotFound to the handler
	}
	return s.repos.StoredCertificates.Transaction(ctx, func(tx *gorm.DB) error {
		// Reset tls_setting on the parent sites for any site cert
		// that points at this stored cert. We do this BEFORE
		// clearing the FK so the subquery still finds the rows.
		if err := tx.Exec(
			`UPDATE sites SET tls_setting = 'auto'
			 WHERE id IN (
			   SELECT site_id FROM certificates
			   WHERE stored_certificate_id = ? AND team_id = ?
			 )`,
			id, teamID,
		).Error; err != nil {
			return err
		}
		// Clear the FK on the certificates link rows.
		if err := tx.Exec(
			`UPDATE certificates SET stored_certificate_id = NULL
			 WHERE stored_certificate_id = ? AND team_id = ?`,
			id, teamID,
		).Error; err != nil {
			return err
		}
		// Clear FK + reset provider on docker domains. Scope via the
		// parent application/compose team_id since docker_application_domains
		// has no team_id column.
		//
		// The inner subquery also filters `d.deleted_at IS NULL` so we
		// don't touch soft-deleted domain rows — those rows are hidden
		// from the read path (Usages), and clearing their FK now would
		// mean that if they're ever restored they'd point at a stored
		// cert that's itself soft-deleted moments later. Leaving the
		// dangling FK in place keeps the historical state intact; the
		// soft-deleted cert is still resolvable via Unscoped if anyone
		// ever needs to reconstruct what was wired up.
		if err := tx.Exec(
			`UPDATE docker_application_domains
			 SET stored_certificate_id = NULL, certificate_provider = 'letsencrypt'
			 WHERE stored_certificate_id = ?
			   AND id IN (
			     SELECT d.id FROM docker_application_domains d
			     LEFT JOIN docker_applications a ON a.id = d.application_id
			     LEFT JOIN docker_composes     c ON c.id = d.compose_id
			     WHERE d.stored_certificate_id = ?
			       AND d.deleted_at IS NULL
			       AND COALESCE(a.team_id, c.team_id) = ?
			   )`,
			id, id, teamID,
		).Error; err != nil {
			return err
		}
		// Finally soft-delete the stored cert itself.
		return tx.Where("team_id = ? AND id = ?", teamID, id).
			Delete(&models.StoredCertificate{}).Error
	})
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
