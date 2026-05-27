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

// Transaction runs fn in a database transaction. Matches the
// convention used by repository.Base's Transaction method; the
// certificate repo doesn't embed that base type, so we expose it
// directly. Callers in the service layer use this for multi-step
// mutations that must be atomic (e.g. DeleteWithForce clearing FKs +
// soft-deleting the cert).
func (r *StoredCertificateRepository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}
