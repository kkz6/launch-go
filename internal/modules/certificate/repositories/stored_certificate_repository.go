package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/certificate/dto"
	"github.com/kkz6/launch-go/internal/modules/certificate/models"
)

type StoredCertificateRepository struct {
	db *gorm.DB
}

func NewStoredCertificateRepository(db *gorm.DB) *StoredCertificateRepository {
	return &StoredCertificateRepository{db: db}
}

func (r *StoredCertificateRepository) Create(ctx context.Context, c *models.StoredCertificate) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *StoredCertificateRepository) FindByID(ctx context.Context, teamID, id string) (*models.StoredCertificate, error) {
	var c models.StoredCertificate
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND id = ?", teamID, id).
		First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *StoredCertificateRepository) List(ctx context.Context, teamID string) ([]models.StoredCertificate, error) {
	var out []models.StoredCertificate
	err := r.db.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("not_after ASC").
		Find(&out).Error
	return out, err
}

func (r *StoredCertificateRepository) Update(ctx context.Context, c *models.StoredCertificate) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *StoredCertificateRepository) SoftDelete(ctx context.Context, teamID, id string) error {
	return r.db.WithContext(ctx).
		Where("team_id = ? AND id = ?", teamID, id).
		Delete(&models.StoredCertificate{}).Error
}

func (r *StoredCertificateRepository) FindByFingerprint(ctx context.Context, teamID, fingerprint string) (*models.StoredCertificate, error) {
	var c models.StoredCertificate
	err := r.db.WithContext(ctx).
		Where("team_id = ? AND fingerprint_sha256 = ?", teamID, fingerprint).
		First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Usages returns the list of sites and docker domains that reference
// the given stored cert. Returns an empty slice when nothing
// references the cert.
//
// Team scoping is column-based for the two cases:
//   - The site-scoped `certificates` table carries its own `team_id`
//     column, so we filter on it directly.
//   - `docker_application_domains` does NOT have a `team_id` column;
//     it inherits team scope from its parent docker_applications /
//     docker_composes row. We join through whichever parent is set on
//     the row (application_id and compose_id are mutually exclusive in
//     practice) and filter team_id on the parent's column.
//
// Note: this method reads from two tables outside the certificate
// module's own schema. We do it here (rather than asking the site /
// docker modules to provide cross-cutting queries) so the certificate
// module owns the usage view end-to-end. The trade-off is the SQL
// names site/docker columns by hand — keep an eye on schema drift.
func (r *StoredCertificateRepository) Usages(ctx context.Context, teamID, certID string) ([]dto.CertificateUsage, error) {
	out := make([]dto.CertificateUsage, 0)

	// Sites — join certificates → sites via certificates.site_id.
	// The `certificates` table has no `deleted_at` column (it's
	// hard-deleted on cascade from sites), so no soft-delete filter.
	type siteRow struct {
		ID      string
		Address string
	}
	var sites []siteRow
	if err := r.db.WithContext(ctx).
		Table("certificates").
		Select("sites.id AS id, sites.address AS address").
		Joins("JOIN sites ON sites.id = certificates.site_id").
		Where("certificates.stored_certificate_id = ? AND certificates.team_id = ?", certID, teamID).
		Scan(&sites).Error; err != nil {
		return nil, err
	}
	for _, s := range sites {
		out = append(out, dto.CertificateUsage{Kind: "site", ID: s.ID, Name: s.Address})
	}

	// Docker domains — scope via the parent application/compose
	// row's team_id. A domain row has either application_id OR
	// compose_id set (never both), so a LEFT JOIN to each and a
	// coalesced team check covers both cases.
	type domRow struct {
		ID   string
		Host string
	}
	var doms []domRow
	if err := r.db.WithContext(ctx).
		Table("docker_application_domains").
		Select("docker_application_domains.id AS id, docker_application_domains.host AS host").
		Joins("LEFT JOIN docker_applications ON docker_applications.id = docker_application_domains.application_id").
		Joins("LEFT JOIN docker_composes ON docker_composes.id = docker_application_domains.compose_id").
		Where("docker_application_domains.stored_certificate_id = ?", certID).
		Where("docker_application_domains.deleted_at IS NULL").
		Where("COALESCE(docker_applications.team_id, docker_composes.team_id) = ?", teamID).
		Scan(&doms).Error; err != nil {
		return nil, err
	}
	for _, d := range doms {
		out = append(out, dto.CertificateUsage{Kind: "docker_domain", ID: d.ID, Name: d.Host})
	}

	return out, nil
}

// UsageDispatchRef is the dispatch-flavoured projection of a single
// resource that references a stored cert. Carries enough context for
// the fanout job to enqueue the appropriate downstream task:
//
//   - SiteID + Address: the site:install_ssl payload needs both.
//   - ApplicationID + ServerID: the docker:sync_traefik_config payload
//     dedupes per application_id.
//   - ComposeID + ServerID: same for docker:sync_compose_traefik_config.
//
// Exactly one of (SiteID, ApplicationID, ComposeID) is set per ref.
type UsageDispatchRef struct {
	Kind          string // "site" | "docker_application" | "docker_compose"
	SiteID        string
	Address       string
	ApplicationID string
	ComposeID     string
	ServerID      string
	TeamID        string
}

// UsageDispatchRefs returns the list of resources referencing the
// stored cert, with enough context for the fanout job to dispatch the
// right downstream task per resource. Same scoping rules as Usages —
// team-scoped via certificates.team_id and the docker
// application/compose parent's team_id.
func (r *StoredCertificateRepository) UsageDispatchRefs(ctx context.Context, teamID, certID string) ([]UsageDispatchRef, error) {
	out := make([]UsageDispatchRef, 0)

	// Sites — every certificate row carries the site_id; the SSL
	// install job uses the site address as the cert-target.
	type siteRow struct {
		ID       string
		Address  string
		ServerID string
	}
	var sites []siteRow
	if err := r.db.WithContext(ctx).
		Table("certificates").
		Select("sites.id AS id, sites.address AS address, sites.server_id AS server_id").
		Joins("JOIN sites ON sites.id = certificates.site_id").
		Where("certificates.stored_certificate_id = ? AND certificates.team_id = ?", certID, teamID).
		Scan(&sites).Error; err != nil {
		return nil, err
	}
	for _, s := range sites {
		out = append(out, UsageDispatchRef{
			Kind:     "site",
			SiteID:   s.ID,
			Address:  s.Address,
			ServerID: s.ServerID,
			TeamID:   teamID,
		})
	}

	// Docker domains — emit one ref per *parent* (application or
	// compose), deduped: the Traefik sync job is per-app or per-
	// compose, not per-domain. A cert that's referenced by 3 domains
	// on the same app produces a single sync dispatch.
	type domRow struct {
		ApplicationID *string
		ComposeID     *string
		// docker_applications / docker_composes both carry server_id —
		// COALESCE handles whichever side is populated.
		ServerID string
	}
	var doms []domRow
	if err := r.db.WithContext(ctx).
		Table("docker_application_domains").
		Select("docker_application_domains.application_id, docker_application_domains.compose_id, COALESCE(docker_applications.server_id, docker_composes.server_id) AS server_id").
		Joins("LEFT JOIN docker_applications ON docker_applications.id = docker_application_domains.application_id").
		Joins("LEFT JOIN docker_composes ON docker_composes.id = docker_application_domains.compose_id").
		Where("docker_application_domains.stored_certificate_id = ?", certID).
		Where("docker_application_domains.deleted_at IS NULL").
		Where("COALESCE(docker_applications.team_id, docker_composes.team_id) = ?", teamID).
		Scan(&doms).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	for _, d := range doms {
		if d.ApplicationID != nil && *d.ApplicationID != "" {
			key := "app:" + *d.ApplicationID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, UsageDispatchRef{
				Kind:          "docker_application",
				ApplicationID: *d.ApplicationID,
				ServerID:      d.ServerID,
				TeamID:        teamID,
			})
		} else if d.ComposeID != nil && *d.ComposeID != "" {
			key := "compose:" + *d.ComposeID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, UsageDispatchRef{
				Kind:      "docker_compose",
				ComposeID: *d.ComposeID,
				ServerID:  d.ServerID,
				TeamID:    teamID,
			})
		}
	}

	return out, nil
}

// Transaction runs fn in a database transaction. Matches the
// convention used by repository.Base's Transaction method; the
// certificate repo doesn't embed that base type, so we expose it
// directly. Callers in the service layer use this for multi-step
// mutations that must be atomic (e.g. DeleteWithForce clearing FKs +
// soft-deleting the cert).
func (r *StoredCertificateRepository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}
