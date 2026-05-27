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
