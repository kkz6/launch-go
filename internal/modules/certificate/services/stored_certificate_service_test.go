// internal/modules/certificate/services/stored_certificate_service_test.go
package services_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/certificate/dto"
	"github.com/kkz6/launch-go/internal/modules/certificate/models"
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/modules/certificate/services"
)

// TestMain initialises the global AES-256 key required by
// dbtype.EncryptedString.Value(); without it the first Create call
// would error with "encryption key not set" and every test would fail
// for a reason unrelated to the service behaviour we're testing.
func TestMain(m *testing.M) {
	if err := serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupServiceDB returns an in-memory sqlite DB with the
// stored_certificates table created and the partial unique index
// that the Postgres migration installs in production. SQLite 3.8+
// supports `WHERE` clauses on indexes, so the same shape works here
// — letting us exercise the name-collision path against the real DB
// constraint instead of mocking it.
//
// We hand-roll the CREATE TABLE statement instead of using
// AutoMigrate: the model's Postgres-specific defaults
// (`default:'[]'::jsonb`) don't parse under sqlite. The column set
// below matches the production migration (0046_create_stored_certificates).
func setupServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE stored_certificates (
			id                  char(26) PRIMARY KEY,
			created_at          datetime,
			updated_at          datetime,
			deleted_at          datetime,
			team_id             char(26) NOT NULL,
			user_id             char(26),
			name                varchar(255) NOT NULL,
			notes               text,
			certificate         text NOT NULL,
			private_key         text NOT NULL,
			domains             text NOT NULL DEFAULT '[]',
			common_name         varchar(255),
			issuer              varchar(255),
			not_before          datetime NOT NULL,
			not_after           datetime NOT NULL,
			serial_number       varchar(255),
			fingerprint_sha256  varchar(64)
		)
	`).Error)

	// Match the partial unique index from migration 0046 so name
	// collisions hit a constraint violation just like they would on PG.
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_stored_certs_team_name_alive
		 ON stored_certificates (team_id, lower(name))
		 WHERE deleted_at IS NULL`,
	).Error)

	// Match the fingerprint partial unique index from migration 0046.
	// In production a concurrent insert with the same fingerprint
	// trips this index and the service translates the resulting
	// 23505 into ErrDuplicateFingerprint; mirroring the constraint
	// here keeps the test schema honest even though the single-
	// threaded tests below only exercise the pre-check path.
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_stored_certs_team_fingerprint_alive
		 ON stored_certificates (team_id, fingerprint_sha256)
		 WHERE deleted_at IS NULL AND fingerprint_sha256 IS NOT NULL`,
	).Error)

	// Minimal shells of the foreign tables that Usages / Delete /
	// DeleteWithForce read or write. We don't pull in the real site /
	// docker models — the queries only touch a handful of columns, and
	// reproducing those columns here keeps the test self-contained.
	//
	// Schema notes carried over from production (verified against the
	// live launch DB on 2026-05-27):
	//   - certificates has team_id but NO deleted_at column.
	//   - certificates has NO tls_setting column — tls_setting lives
	//     on sites. So DeleteWithForce updates sites.tls_setting, not
	//     certificates.tls_setting (see the Force tests below).
	//   - docker_application_domains has NO team_id; team scope comes
	//     from docker_applications.team_id or docker_composes.team_id
	//     via the application_id / compose_id FK.
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS sites (
			id          char(26) PRIMARY KEY,
			team_id     char(26) NOT NULL,
			address     varchar(255) NOT NULL,
			tls_setting varchar(32) NOT NULL DEFAULT 'auto'
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS certificates (
			id                    char(26) PRIMARY KEY,
			team_id               char(26) NOT NULL,
			site_id               char(26) NOT NULL,
			stored_certificate_id char(26) NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS docker_applications (
			id      char(26) PRIMARY KEY,
			team_id char(26) NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS docker_composes (
			id      char(26) PRIMARY KEY,
			team_id char(26) NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS docker_application_domains (
			id                    char(26) PRIMARY KEY,
			application_id        char(26) NULL,
			compose_id            char(26) NULL,
			host                  varchar(255) NOT NULL,
			certificate_provider  varchar(32) NOT NULL DEFAULT 'letsencrypt',
			stored_certificate_id char(26) NULL,
			deleted_at            datetime NULL
		)
	`).Error)

	return db
}

func newService(t *testing.T, db *gorm.DB) *services.StoredCertificateService {
	t.Helper()
	return services.NewStoredCertificateService(repositories.NewRegistry(db))
}

// countRows is a small helper that bypasses GORM's default soft-delete
// scope so we can assert "absolutely nothing was inserted" in the
// negative tests.
func countRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Unscoped().Model(&models.StoredCertificate{}).Count(&n).Error)
	return n
}

// makeOtherCertPEM generates a fresh self-signed cert + RSA key
// in PEM form for the name-collision test. We don't care about the
// content beyond "it's a different cert" — the test only needs a
// valid cert+key pair whose fingerprint differs from leaf.pem's so
// the dedupe check passes and we hit the name-unique constraint.
func makeOtherCertPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "other.example"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"other.example"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	return certPEM, keyPEM
}

func TestService_Create_Success(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	certPEM := mustRead(t, "leaf.pem")
	keyPEM := mustRead(t, "leaf.key")

	got, err := svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod",
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	})
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.NotEmpty(t, got.ID, "BeforeCreate should populate a ULID")
	assert.Equal(t, "team-a", got.TeamID)
	assert.Equal(t, "acme prod", got.Name)
	assert.Equal(t, certPEM, got.Certificate)
	// EncryptedString round-trips as the plaintext on the in-memory
	// instance — encryption only kicks in when Value() is called, and
	// we want to confirm the caller's input wasn't mangled.
	assert.Equal(t, keyPEM, string(got.PrivateKey))
	assert.ElementsMatch(t, []string{"acme.io", "*.acme.io"}, []string(got.Domains))
	require.NotNil(t, got.FingerprintSHA256)
	assert.Len(t, *got.FingerprintSHA256, 64)
	assert.False(t, got.NotBefore.IsZero())
	assert.False(t, got.NotAfter.IsZero())

	// Round-trip through the DB to confirm the encrypted column
	// decrypts back to the original key — this is the
	// service+dbtype+serializer integration assertion.
	var reread models.StoredCertificate
	require.NoError(t, db.First(&reread, "id = ?", got.ID).Error)
	assert.Equal(t, keyPEM, string(reread.PrivateKey))
}

func TestService_Create_KeyMismatch_422(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	_, err := svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod",
		Certificate: mustRead(t, "leaf.pem"),
		PrivateKey:  mustRead(t, "mismatched.key"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key does not match certificate")
	assert.Equal(t, int64(0), countRows(t, db), "validation failure must not insert")
}

func TestService_Create_MalformedPEM_422(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	_, err := svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod",
		Certificate: "not a pem",
		PrivateKey:  mustRead(t, "leaf.key"),
	})
	require.Error(t, err)
	assert.Equal(t, int64(0), countRows(t, db), "malformed PEM must not insert")
}

func TestService_Create_DuplicateFingerprint_409(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	certPEM := mustRead(t, "leaf.pem")
	keyPEM := mustRead(t, "leaf.key")

	first, err := svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod",
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	})
	require.NoError(t, err)

	// Same cert content, different name — fingerprint dedupe should
	// fire before the name-unique constraint gets a chance to.
	_, err = svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod renamed",
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	})
	require.Error(t, err)

	var dupErr services.ErrDuplicateFingerprint
	require.True(t, errors.As(err, &dupErr), "expected ErrDuplicateFingerprint, got %T: %v", err, err)
	require.NotNil(t, dupErr.Existing)
	assert.Equal(t, first.ID, dupErr.Existing.ID)
	assert.Equal(t, "acme prod", dupErr.Existing.Name)

	assert.Equal(t, int64(1), countRows(t, db), "duplicate must not create a second row")
}

func TestService_Create_NameCollision_409(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	_, err := svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod",
		Certificate: mustRead(t, "leaf.pem"),
		PrivateKey:  mustRead(t, "leaf.key"),
	})
	require.NoError(t, err)

	// Different cert (different fingerprint) but identical name —
	// the partial unique index on (team_id, lower(name)) WHERE
	// deleted_at IS NULL should reject this.
	otherCert, otherKey := makeOtherCertPEM(t)
	_, err = svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme prod",
		Certificate: otherCert,
		PrivateKey:  otherKey,
	})
	require.Error(t, err)
	// Sqlite surfaces "UNIQUE constraint failed" for this case; we
	// don't bind to that exact string, just confirm one row.
	assert.Equal(t, int64(1), countRows(t, db), "name collision must not create a second row")
}

// seedCert creates a stored cert via the service using the test
// fixture pair. Tests that need a referenceable cert call this rather
// than re-running the parse path inline.
func seedCert(t *testing.T, svc *services.StoredCertificateService, teamID, name string) *models.StoredCertificate {
	t.Helper()
	c, err := svc.Create(context.Background(), teamID, nil, dto.CreateStoredCertificateRequest{
		Name:        name,
		Certificate: mustRead(t, "leaf.pem"),
		PrivateKey:  mustRead(t, "leaf.key"),
	})
	require.NoError(t, err)
	return c
}

// insertSiteRef inserts the minimum row pair (sites + certificates)
// that points a site's cert at the given stored cert. Returns the
// site id.
func insertSiteRef(t *testing.T, db *gorm.DB, teamID, certStoredID, address string) string {
	t.Helper()
	siteID := "site_" + address // sqlite doesn't care about ulid shape; uniqueness is enough
	// Pad to 26 chars so char(26) PK doesn't truncate something else
	// like another site row with a prefix collision.
	for len(siteID) < 26 {
		siteID += "x"
	}
	siteID = siteID[:26]
	require.NoError(t, db.Exec(
		`INSERT INTO sites (id, team_id, address, tls_setting) VALUES (?, ?, ?, 'manual')`,
		siteID, teamID, address,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO certificates (id, team_id, site_id, stored_certificate_id) VALUES (?, ?, ?, ?)`,
		siteID+"-c", teamID, siteID, certStoredID,
	).Error)
	return siteID
}

// insertDomainRef inserts a docker_application_domains row pointing
// at the given stored cert, scoped via a parent docker_applications
// row with the given team_id. Returns the domain id.
func insertDomainRef(t *testing.T, db *gorm.DB, teamID, certStoredID, host string) string {
	t.Helper()
	appID := "app_" + host
	for len(appID) < 26 {
		appID += "x"
	}
	appID = appID[:26]
	domID := "dom_" + host
	for len(domID) < 26 {
		domID += "x"
	}
	domID = domID[:26]
	require.NoError(t, db.Exec(
		`INSERT INTO docker_applications (id, team_id) VALUES (?, ?)`,
		appID, teamID,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO docker_application_domains (id, application_id, host, certificate_provider, stored_certificate_id)
		 VALUES (?, ?, ?, 'stored', ?)`,
		domID, appID, host, certStoredID,
	).Error)
	return domID
}

func TestService_Usages_Empty(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	got, err := svc.Usages(ctx, "team-a", c.ID)
	require.NoError(t, err)
	assert.Empty(t, got, "fresh cert with no refs should return no usages")
}

func TestService_Usages_OneSiteOneDomain(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	siteID := insertSiteRef(t, db, "team-a", c.ID, "acme.io")
	domID := insertDomainRef(t, db, "team-a", c.ID, "api.acme.io")

	got, err := svc.Usages(ctx, "team-a", c.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// We don't pin order; check by kind.
	bykind := map[string]dto.CertificateUsage{}
	for _, u := range got {
		bykind[u.Kind] = u
	}
	require.Contains(t, bykind, "site")
	require.Contains(t, bykind, "docker_domain")
	assert.Equal(t, siteID, bykind["site"].ID)
	assert.Equal(t, "acme.io", bykind["site"].Name)
	assert.Equal(t, domID, bykind["docker_domain"].ID)
	assert.Equal(t, "api.acme.io", bykind["docker_domain"].Name)
}

func TestService_Update_NameOnly(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")
	origFP := c.FingerprintSHA256

	newName := "acme renamed"
	got, pending, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Name: &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, pending, "name-only update should not flag any redeploys")
	assert.Equal(t, "acme renamed", got.Name)
	// Metadata untouched.
	assert.Equal(t, origFP, got.FingerprintSHA256)
}

func TestService_Update_NotesOnly(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	notes := "rotated 2026-05"
	got, pending, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Notes: &notes,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, pending)
	require.NotNil(t, got.Notes)
	assert.Equal(t, "rotated 2026-05", *got.Notes)
}

func TestService_Update_CertAndKey_RefreshesMetadata(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")
	origFP := c.FingerprintSHA256
	origNotAfter := c.NotAfter
	origDomains := []string(c.Domains)

	newCert, newKey := makeOtherCertPEM(t)
	got, pending, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Certificate: &newCert,
		PrivateKey:  &newKey,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, pending, "no refs yet — pending should be zero even on content change")

	// Metadata should reflect the new cert.
	require.NotNil(t, got.FingerprintSHA256)
	require.NotNil(t, origFP)
	assert.NotEqual(t, *origFP, *got.FingerprintSHA256, "fingerprint must refresh")
	assert.NotEqual(t, origNotAfter.UTC(), got.NotAfter.UTC(), "not_after must refresh")
	assert.NotEqual(t, origDomains, []string(got.Domains), "domains must refresh")
	assert.Equal(t, newCert, got.Certificate)
}

func TestService_Update_PartialCertOnly_Errors(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	newCert, _ := makeOtherCertPEM(t)
	_, _, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Certificate: &newCert,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrPartialCertKeyUpdate)
}

func TestService_Update_PartialKeyOnly_Errors(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	_, newKey := makeOtherCertPEM(t)
	_, _, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		PrivateKey: &newKey,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, services.ErrPartialCertKeyUpdate)
}

func TestService_Update_KeyMismatch_Errors(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	// New cert paired with a non-matching key (the mismatched test
	// fixture is paired with leaf.pem, not the freshly generated cert).
	newCert, _ := makeOtherCertPEM(t)
	mismatched := mustRead(t, "mismatched.key")
	_, _, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Certificate: &newCert,
		PrivateKey:  &mismatched,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key does not match certificate")
}

func TestService_Update_DuplicateFingerprintWithDifferentRow(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	// Cert A — uses the leaf.pem fixture.
	a := seedCert(t, svc, "team-a", "acme prod")

	// Cert B — uses an unrelated generated pair.
	otherCert, otherKey := makeOtherCertPEM(t)
	b, err := svc.Create(ctx, "team-a", nil, dto.CreateStoredCertificateRequest{
		Name:        "acme staging",
		Certificate: otherCert,
		PrivateKey:  otherKey,
	})
	require.NoError(t, err)

	// Now try to update B's content to match A. Should fail with
	// ErrDuplicateFingerprint pointing at A.
	leafCert := mustRead(t, "leaf.pem")
	leafKey := mustRead(t, "leaf.key")
	_, _, err = svc.Update(ctx, "team-a", b.ID, dto.UpdateStoredCertificateRequest{
		Certificate: &leafCert,
		PrivateKey:  &leafKey,
	})
	require.Error(t, err)

	var dupErr services.ErrDuplicateFingerprint
	require.True(t, errors.As(err, &dupErr), "expected ErrDuplicateFingerprint, got %T: %v", err, err)
	require.NotNil(t, dupErr.Existing)
	assert.Equal(t, a.ID, dupErr.Existing.ID)
}

func TestService_Update_PendingRedeploysCount(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	// Two site refs + one docker domain ref → 3 pending redeploys.
	insertSiteRef(t, db, "team-a", c.ID, "one.example")
	insertSiteRef(t, db, "team-a", c.ID, "two.example")
	insertDomainRef(t, db, "team-a", c.ID, "api.example")

	newCert, newKey := makeOtherCertPEM(t)
	_, pending, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Certificate: &newCert,
		PrivateKey:  &newKey,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, pending)
}

func TestService_Update_NotFound(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	newName := "anything"
	_, _, err := svc.Update(ctx, "team-a", "01HZZZZZZZZZZZZZZZZZZZZZZZ", dto.UpdateStoredCertificateRequest{
		Name: &newName,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "expected ErrRecordNotFound, got %v", err)
}

func TestService_Usages_TeamScoped(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	// Cert lives in team-a; the referencing rows live in team-b.
	c := seedCert(t, svc, "team-a", "acme prod")
	_ = insertSiteRef(t, db, "team-b", c.ID, "acme.io")
	_ = insertDomainRef(t, db, "team-b", c.ID, "api.acme.io")

	// Asking team-a for usages of its cert finds nothing — the refs
	// are scoped to a different team.
	got, err := svc.Usages(ctx, "team-a", c.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestService_Delete_NotInUse_Succeeds(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	require.NoError(t, svc.Delete(ctx, "team-a", c.ID))

	// Subsequent FindByID via the repository should return NotFound
	// because GORM's default scope hides soft-deleted rows.
	_, err := svc.Usages(ctx, "team-a", c.ID) // safe call — no error path here
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.StoredCertificate{}).Where("id = ?", c.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "soft-delete should hide the row from default scope")

	// Unscoped, the row should still exist with deleted_at set.
	var unscoped int64
	require.NoError(t, db.Unscoped().Model(&models.StoredCertificate{}).Where("id = ?", c.ID).Count(&unscoped).Error)
	assert.Equal(t, int64(1), unscoped, "soft-delete keeps the row in storage")
}

func TestService_Delete_InUse_ReturnsErrInUse(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")
	insertSiteRef(t, db, "team-a", c.ID, "acme.io")
	insertDomainRef(t, db, "team-a", c.ID, "api.acme.io")

	err := svc.Delete(ctx, "team-a", c.ID)
	require.Error(t, err)

	var inUse services.ErrInUse
	require.True(t, errors.As(err, &inUse), "expected ErrInUse, got %T: %v", err, err)
	assert.Len(t, inUse.Usages, 2)

	// Cert is still alive.
	var count int64
	require.NoError(t, db.Model(&models.StoredCertificate{}).Where("id = ?", c.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "in-use delete must not soft-delete the cert")
}

func TestService_DeleteWithForce_ClearsFKsAndSoftDeletes(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")
	siteID := insertSiteRef(t, db, "team-a", c.ID, "acme.io")
	domID := insertDomainRef(t, db, "team-a", c.ID, "api.acme.io")

	require.NoError(t, svc.DeleteWithForce(ctx, "team-a", c.ID))

	// Cert is soft-deleted.
	var aliveCount int64
	require.NoError(t, db.Model(&models.StoredCertificate{}).Where("id = ?", c.ID).Count(&aliveCount).Error)
	assert.Equal(t, int64(0), aliveCount, "DeleteWithForce should soft-delete the stored cert")

	// certificates.stored_certificate_id should be NULL on the
	// referencing row, and the parent site's tls_setting back to
	// 'auto'.
	var certStoredID *string
	require.NoError(t, db.Raw(
		`SELECT stored_certificate_id FROM certificates WHERE site_id = ?`, siteID,
	).Row().Scan(&certStoredID))
	assert.Nil(t, certStoredID, "site cert link should be cleared")

	var tlsSetting string
	require.NoError(t, db.Raw(
		`SELECT tls_setting FROM sites WHERE id = ?`, siteID,
	).Row().Scan(&tlsSetting))
	assert.Equal(t, "auto", tlsSetting, "site tls_setting must be reset to auto")

	// docker_application_domains: stored_certificate_id cleared,
	// certificate_provider reset to letsencrypt.
	var domStoredID *string
	var provider string
	require.NoError(t, db.Raw(
		`SELECT stored_certificate_id, certificate_provider FROM docker_application_domains WHERE id = ?`, domID,
	).Row().Scan(&domStoredID, &provider))
	assert.Nil(t, domStoredID, "docker domain cert link should be cleared")
	assert.Equal(t, "letsencrypt", provider, "docker domain provider must reset to letsencrypt")
}

func TestService_Delete_WrongTeam_NotFound(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")

	// team-b tries to delete team-a's cert. Should not delete the
	// row; SoftDelete being a no-op or returning ErrRecordNotFound
	// are both acceptable. We just verify the row stays alive.
	_ = svc.Delete(ctx, "team-b", c.ID) // ignore error: no-op is allowed

	var count int64
	require.NoError(t, db.Model(&models.StoredCertificate{}).Where("id = ?", c.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "wrong-team delete must not soft-delete the cert")
}

// TestService_Update_NoOpContentChange covers the case where the
// caller re-sends the EXACT same cert + key on a PATCH. The service
// must recognise this as a no-op and skip the re-parse / dedupe /
// metadata-refresh path. Crucially pending_redeploys must be 0 — if
// it weren't, Phase 6 would fan out site:install_ssl / docker:redeploy
// jobs to every linked resource for what is, semantically, a metadata
// edit (e.g. the user only meant to change the Name field but the
// frontend re-sent the full form).
func TestService_Update_NoOpContentChange(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")
	// Snapshot the metadata that a re-parse would rewrite. We assert
	// against these copies after Update to confirm the metadata
	// columns weren't touched.
	origFP := c.FingerprintSHA256
	origNotBefore := c.NotBefore
	origNotAfter := c.NotAfter
	origDomains := append([]string(nil), []string(c.Domains)...) // copy, not pointer alias

	// Two referencing rows: if Update mistakenly entered the content-
	// change branch, pendingRedeploys would come back as 2.
	insertSiteRef(t, db, "team-a", c.ID, "acme.io")
	insertDomainRef(t, db, "team-a", c.ID, "api.acme.io")

	// Re-send the EXACT same cert + key bytes that the row currently
	// holds. The service should detect "nothing changed" and short-
	// circuit the content-change branch.
	sameCert := c.Certificate
	sameKey := string(c.PrivateKey)
	got, pending, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Certificate: &sameCert,
		PrivateKey:  &sameKey,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, pending,
		"no-op content change must report zero pending redeploys; "+
			"non-zero here would trigger spurious Phase 6 fanout")

	// Metadata columns unchanged — proves the re-parse path was
	// skipped. We compare ElementsMatch on Domains rather than slice
	// identity because GORM's JSON serializer may reallocate the
	// backing slice even when the contents are equal.
	require.NotNil(t, got.FingerprintSHA256)
	require.NotNil(t, origFP)
	assert.Equal(t, *origFP, *got.FingerprintSHA256, "fingerprint must not change on no-op")
	assert.True(t, origNotBefore.Equal(got.NotBefore), "not_before must not change on no-op")
	assert.True(t, origNotAfter.Equal(got.NotAfter), "not_after must not change on no-op")
	assert.ElementsMatch(t, origDomains, []string(got.Domains), "domains must not change on no-op")
	assert.Equal(t, c.Certificate, got.Certificate, "certificate PEM unchanged")
}

// TestService_Update_NameAndContent_BothApplied covers the case where
// the caller sends BOTH a name change and a new cert+key pair in a
// single PATCH. The service must:
//   - apply the new name,
//   - refresh metadata from the new cert (fingerprint changes),
//   - report pendingRedeploys correctly from the existing usage refs.
//
// This is the "rotation while renaming" path — common enough that we
// want it covered as a unit, not just inferred from the single-field
// tests above.
func TestService_Update_NameAndContent_BothApplied(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	c := seedCert(t, svc, "team-a", "acme prod")
	origFP := c.FingerprintSHA256

	// Single site ref → pendingRedeploys should be 1 on a real
	// content change.
	insertSiteRef(t, db, "team-a", c.ID, "acme.io")

	renamed := "renamed"
	newCert, newKey := makeOtherCertPEM(t)
	got, pending, err := svc.Update(ctx, "team-a", c.ID, dto.UpdateStoredCertificateRequest{
		Name:        &renamed,
		Certificate: &newCert,
		PrivateKey:  &newKey,
	})
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name, "name change must be applied")
	require.NotNil(t, got.FingerprintSHA256)
	require.NotNil(t, origFP)
	assert.NotEqual(t, *origFP, *got.FingerprintSHA256, "fingerprint must refresh from new cert")
	assert.Equal(t, 1, pending, "one site ref → one pending redeploy")

	// Persistence check: re-fetch via the service and confirm both
	// the name and the new fingerprint round-tripped through the DB.
	usages, err := svc.Usages(ctx, "team-a", c.ID)
	require.NoError(t, err)
	require.Len(t, usages, 1)

	var reread models.StoredCertificate
	require.NoError(t, db.First(&reread, "id = ?", c.ID).Error)
	assert.Equal(t, "renamed", reread.Name)
	require.NotNil(t, reread.FingerprintSHA256)
	assert.Equal(t, *got.FingerprintSHA256, *reread.FingerprintSHA256,
		"new fingerprint must be persisted")
}

// TestService_DeleteWithForce_OtherTeamRefsUntouched proves the
// team_id filter in DeleteWithForce's UPDATEs is enforced. We seed
// the corruption case directly: rows in team-b that point at a stored
// cert in team-a (which the service layer would never allow, but the
// DB schema doesn't enforce same-team via FK — only an app-layer
// invariant). When team-a force-deletes their cert, team-b's stale
// references must stay exactly where they were.
func TestService_DeleteWithForce_OtherTeamRefsUntouched(t *testing.T) {
	db := setupServiceDB(t)
	svc := newService(t, db)
	ctx := context.Background()

	// Cert lives in team-a.
	c := seedCert(t, svc, "team-a", "acme prod")

	// Cross-team site ref: certificates row in team-b that points
	// at team-a's stored cert. This simulates the corruption case
	// the service-layer team filter is meant to defend against.
	otherSiteID := insertSiteRef(t, db, "team-b", c.ID, "acme.io")
	otherCertRowID := otherSiteID + "-c"

	// Cross-team docker domain ref similarly.
	otherDomID := insertDomainRef(t, db, "team-b", c.ID, "api.acme.io")

	require.NoError(t, svc.DeleteWithForce(ctx, "team-a", c.ID))

	// team-a's stored cert is soft-deleted (the good behaviour).
	var aliveCount int64
	require.NoError(t, db.Model(&models.StoredCertificate{}).Where("id = ?", c.ID).Count(&aliveCount).Error)
	assert.Equal(t, int64(0), aliveCount, "team-a stored cert should be soft-deleted")

	// team-b's site cert ref is UNTOUCHED — stored_certificate_id
	// still points at the (now soft-deleted) cert, tls_setting
	// still 'manual'. The asymmetry here is intentional: team-a's
	// force-delete must not silently mutate rows owned by team-b.
	var storedID *string
	require.NoError(t, db.Raw(
		`SELECT stored_certificate_id FROM certificates WHERE id = ?`, otherCertRowID,
	).Row().Scan(&storedID))
	require.NotNil(t, storedID, "team-b certificates row must not be cleared by team-a delete")
	assert.Equal(t, c.ID, *storedID)

	var tlsSetting string
	require.NoError(t, db.Raw(
		`SELECT tls_setting FROM sites WHERE id = ?`, otherSiteID,
	).Row().Scan(&tlsSetting))
	assert.Equal(t, "manual", tlsSetting,
		"team-b sites.tls_setting must be unchanged (still 'manual', not reset to 'auto')")

	// team-b's docker domain ref is UNTOUCHED — stored_certificate_id
	// still set, certificate_provider still 'stored'.
	var domStoredID *string
	var domProvider string
	require.NoError(t, db.Raw(
		`SELECT stored_certificate_id, certificate_provider FROM docker_application_domains WHERE id = ?`, otherDomID,
	).Row().Scan(&domStoredID, &domProvider))
	require.NotNil(t, domStoredID, "team-b docker domain ref must not be cleared by team-a delete")
	assert.Equal(t, c.ID, *domStoredID)
	assert.Equal(t, "stored", domProvider,
		"team-b docker domain certificate_provider must be unchanged (still 'stored')")
}
