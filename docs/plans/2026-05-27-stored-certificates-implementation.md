# Stored SSL Certificates — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship a team-scoped SSL-certificate library that PHP sites and
Docker domains can pick from instead of pasting cert PEM inline every
time.

**Architecture:** New `internal/modules/certificate/` Go module with
its own `stored_certificates` table. Existing `certificates` (per-site)
and `docker_application_domains` tables gain a nullable
`stored_certificate_id` FK. PHP SSL dialog and Docker domain dialog
gain a picker that writes the FK. Re-uploading a stored cert fans out
to all referencing sites/domains.

**Tech Stack:** Go 1.22 / GORM / asynq / Fiber / Vue 3 / Nuxt 3 / vee-
validate. Encryption helpers: `dbtype.EncryptedString`. PEM parsing:
`crypto/x509`.

**Predecessor design doc:** `docs/plans/2026-05-27-stored-certificates-design.md`

**Branches:** `feature/stored-certificates` on both `launch-go` and
`launch-nuxt`.

---

## How this plan is structured

Six rollout phases (1-6) line up with the design doc's §10. **Phases 1
and 2 are fully tasked below** — they together produce a working
backend module shipping dark. **Phases 3-6 are sketched** as follow-up
plans; each will be detailed in its own `docs/plans/...` file before
implementation.

Before each task: read `@docs/plans/2026-05-27-stored-certificates-design.md`
for the canonical decisions.

---

# Phase 1 — Migration + Module Skeleton (launch-go)

End state: tables exist, GORM model wired, module registered with the
kernel, no HTTP routes yet, `go test ./... && go vet ./...` green.

## Task 1.1: Create `stored_certificates` migration

**Files:**
- Create: `internal/database/migrations/0046_05_27_000000_create_stored_certificates.go`

> **DB note:** the codebase has been moved to **Postgres** (the local
> DB is restored from `launch_app_new_may26.pg.sql`; `.env` now has
> `DB_DRIVER=postgres`, port 5432). All SQL below uses Postgres idioms:
> `JSONB` for the domains column, partial unique indexes with
> `WHERE deleted_at IS NULL` for soft-delete uniqueness, and functional
> indexes (`LOWER(name)`) where case-insensitivity matters. No
> `ENGINE=`/`CHARSET=`/`COLLATE=` clauses — those are MySQL artefacts.
> Migration `0037` is still the structural template (Register shape,
> Up/Down naming, comment density) — just don't copy its `TEXT` /
> non-partial-index choices, which were MySQL-portable concessions.

**Step 1: Read a recent migration as a template**

Read `internal/database/migrations/0037_05_24_000003_create_registry_credentials.go`
in full — same shape (team-scoped, encrypted fields, soft-delete,
per-team unique name index). Use it as the structural template,
especially the `Register(Migration{ID, Name, Timestamp: time.Date(...), ...})`
shape and the soft-delete-aware unique index pattern.

**Step 2: Write the migration**

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0046_05_27_000000_create_stored_certificates",
		Name:      "Create stored_certificates table",
		Timestamp: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		Up:        createStoredCertificatesUp,
		Down:      createStoredCertificatesDown,
	})
}

// createStoredCertificatesUp lands the team-scoped reusable cert
// library. Same shape registry_credentials / source_controls /
// storage_providers follow: team-scoped, soft-deletable, user_id
// recorded for audit only, `private_key` encrypted at rest via
// dbtype.EncryptedString.
//
// `domains` is JSONB so we can later query "certs covering host X"
// via JSONB containment if needed; dbtype.JSONStringSlice handles
// the GORM scan/value.
//
// Soft-delete uniqueness uses Postgres partial unique indexes
// (WHERE deleted_at IS NULL) — re-using a name after soft-delete is
// allowed because the deleted row is excluded from the index. Same
// pattern applies to the fingerprint dedupe.
func createStoredCertificatesUp(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE stored_certificates (
			id                  CHAR(26)     NOT NULL,
			team_id             CHAR(26)     NOT NULL,
			user_id             CHAR(26)     NULL,
			name                VARCHAR(255) NOT NULL,
			notes               TEXT          NULL,
			certificate         TEXT          NOT NULL,
			private_key         TEXT          NOT NULL,
			domains             JSONB         NOT NULL DEFAULT '[]'::jsonb,
			common_name         VARCHAR(255)  NULL,
			issuer              VARCHAR(255)  NULL,
			not_before          TIMESTAMPTZ   NOT NULL,
			not_after           TIMESTAMPTZ   NOT NULL,
			serial_number       VARCHAR(255)  NULL,
			fingerprint_sha256  VARCHAR(64)   NULL,
			created_at          TIMESTAMPTZ   NULL,
			updated_at          TIMESTAMPTZ   NULL,
			deleted_at          TIMESTAMPTZ   NULL,
			PRIMARY KEY (id),
			CONSTRAINT fk_stored_certs_team FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
		)
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_stored_certs_team ON stored_certificates (team_id)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_stored_certs_deleted ON stored_certificates (deleted_at)`).Error; err != nil {
		return err
	}
	// Per-team unique name, alive rows only. LOWER(name) so users
	// can't sneak in "Acme" vs "acme" as separate certs.
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_stored_certs_team_name_alive
		ON stored_certificates (team_id, LOWER(name))
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return err
	}
	// Per-team fingerprint dedupe, alive rows only.
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_stored_certs_team_fingerprint_alive
		ON stored_certificates (team_id, fingerprint_sha256)
		WHERE deleted_at IS NULL AND fingerprint_sha256 IS NOT NULL
	`).Error; err != nil {
		return err
	}
	// Expiry queries — "what's expiring in the next 30 days for this team".
	if err := db.Exec(`
		CREATE INDEX idx_stored_certs_expiry
		ON stored_certificates (team_id, not_after)
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return err
	}
	return nil
}

func createStoredCertificatesDown(db *gorm.DB) error {
	return db.Exec(`DROP TABLE IF EXISTS stored_certificates`).Error
}
```

**Step 3: Run the migration locally**

```bash
make migrate
```

Expected: migration `0046_05_27_000000_create_stored_certificates`
applied; no errors. (Make target is `migrate`, not `migrate-up` —
verify with `grep ^migrate Makefile` if unsure.)

**Step 4: Verify schema**

```bash
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USERNAME" -d "$DB_DATABASE" \
  -c "\d+ stored_certificates"
```

Expected: 17 columns, 5 indexes (PK + `team`, `deleted`,
`team_name_alive` (partial unique on `LOWER(name)`),
`team_fingerprint_alive` (partial unique), `expiry` (partial)),
one FK to `teams(id) ON DELETE CASCADE`.

**Step 5: Commit**

```bash
git add internal/database/migrations/0046_05_27_000000_create_stored_certificates.go
git commit -m "stored_certificates: migration for new team-scoped table"
```

---

## Task 1.2: Add FK columns to `certificates` and `docker_application_domains`

**Files:**
- Create: `internal/database/migrations/0047_05_27_000001_add_stored_certificate_id_fks.go`

**Step 1: Write the migration**

Postgres supports inline `REFERENCES … ON DELETE …` in `ALTER TABLE
ADD COLUMN`, so the FK column lands in a single statement per table.
Indexes on the FK columns are added separately (for fast joins from
the parent stored_certificate back to its referencing rows).

```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0047_05_27_000001_add_stored_certificate_id_fks",
		Name:      "Add stored_certificate_id FK to certificates and docker_application_domains",
		Timestamp: time.Date(2026, 5, 27, 0, 0, 1, 0, time.UTC),
		Up:        addStoredCertificateFKsUp,
		Down:      addStoredCertificateFKsDown,
	})
}

func addStoredCertificateFKsUp(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE certificates
		ADD COLUMN stored_certificate_id CHAR(26) NULL
		REFERENCES stored_certificates(id) ON DELETE SET NULL
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_certificates_stored_cert ON certificates (stored_certificate_id)`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		ALTER TABLE docker_application_domains
		ADD COLUMN stored_certificate_id CHAR(26) NULL
		REFERENCES stored_certificates(id) ON DELETE SET NULL
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_docker_app_domains_stored_cert ON docker_application_domains (stored_certificate_id)`).Error; err != nil {
		return err
	}

	return nil
}

func addStoredCertificateFKsDown(db *gorm.DB) error {
	// Postgres DROP COLUMN cascades the FK constraint and the index
	// automatically. IF EXISTS keeps Down idempotent across partial
	// migration states.
	_ = db.Exec(`ALTER TABLE certificates DROP COLUMN IF EXISTS stored_certificate_id`).Error
	_ = db.Exec(`ALTER TABLE docker_application_domains DROP COLUMN IF EXISTS stored_certificate_id`).Error
	return nil
}
```

**Step 2: Run + verify**

```bash
make migrate
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USERNAME" -d "$DB_DATABASE" \
  -c "\d certificates" | grep stored_certificate_id
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USERNAME" -d "$DB_DATABASE" \
  -c "\d docker_application_domains" | grep stored_certificate_id
```

Expected: both tables show the new nullable `character(26)` column.

**Step 3: Commit**

```bash
git add internal/database/migrations/0047_05_27_000001_add_stored_certificate_id_fks.go
git commit -m "stored_certificates: FK columns on certificates + docker_application_domains"
```

---

## Task 1.3: Module skeleton — directory layout

**Files:**
- Create: `internal/modules/certificate/module.go`
- Create: `internal/modules/certificate/models/stored_certificate.go`
- Create: `internal/modules/certificate/repositories/stored_certificate_repository.go`
- Create: `internal/modules/certificate/repositories/registry.go`
- Create: `internal/modules/certificate/services/stored_certificate_service.go`
- Create: `internal/modules/certificate/dto/requests.go`
- Create: `internal/modules/certificate/dto/responses.go`
- Create: `internal/modules/certificate/handlers/stored_certificate_handler.go`

**Step 1: Read the registry_credential module as a template**

Read `internal/modules/docker/models/registry_credential.go`,
`.../repositories/registry_credential_repository.go`, and
`.../services/registry_credential_service.go`. Same encrypted-field
pattern, same team-scoped query shape.

**Step 2: Define the model**

`internal/modules/certificate/models/stored_certificate.go`:

```go
package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// StoredCertificate is a team-scoped, reusable TLS certificate +
// private key pair. Picked from a dropdown wherever HTTPS is
// configured (PHP sites, docker domains). Mirrors the shape of
// registry_credentials / source_controls / storage_providers.
//
// `Certificate` is the leaf-PEM (often including chain); never
// secret on its own, kept as plain LONGTEXT for indexing and audit.
// `PrivateKey` is encrypted at rest via dbtype.EncryptedString and
// is the only field that's `json:"-"` — everything else can be
// safely exposed in API responses.
//
// Parsed metadata (domains / not_before / not_after / issuer / serial
// / fingerprint) is server-derived on save by the service layer; the
// user never supplies it. See services/parser.go.
//
// Soft-delete: deleted_at is part of the unique indexes so a removed
// row's name (and fingerprint) can be reused later.
type StoredCertificate struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel

	TeamID string  `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID *string `gorm:"column:user_id;type:char(26)" json:"user_id,omitempty"`

	Name  string  `gorm:"type:varchar(255);not null" json:"name"`
	Notes *string `gorm:"type:text" json:"notes,omitempty"`

	Certificate string                 `gorm:"type:text;not null" json:"certificate"`
	PrivateKey  dbtype.EncryptedString `gorm:"column:private_key;type:text;not null" json:"-"`

	Domains           dbtype.JSONStringSlice `gorm:"type:jsonb;not null;default:'[]'::jsonb" json:"domains"`
	CommonName        *string                `gorm:"column:common_name;type:varchar(255)" json:"common_name,omitempty"`
	Issuer            *string                `gorm:"type:varchar(255)" json:"issuer,omitempty"`
	NotBefore         time.Time              `gorm:"column:not_before;type:timestamptz;not null" json:"not_before"`
	NotAfter          time.Time              `gorm:"column:not_after;type:timestamptz;not null" json:"not_after"`
	SerialNumber      *string                `gorm:"column:serial_number;type:varchar(255)" json:"serial_number,omitempty"`
	FingerprintSHA256 *string                `gorm:"column:fingerprint_sha256;type:varchar(64)" json:"fingerprint_sha256,omitempty"`
}

func (StoredCertificate) TableName() string { return "stored_certificates" }
```

**Step 3: Add empty repository, service, DTO, handler, module files**

Repository (CRUD methods only — no business logic yet):

```go
// internal/modules/certificate/repositories/stored_certificate_repository.go
package repositories

import (
	"context"

	"gorm.io/gorm"

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
```

Registry (stub for now):

```go
// internal/modules/certificate/repositories/registry.go
package repositories

import "gorm.io/gorm"

// Registry bundles the certificate module's repositories. Matches the
// `Registry` naming convention used by docker / server / site modules
// (not "Repositories" — distinct from the package name).
type Registry struct {
	StoredCertificates *StoredCertificateRepository
}

func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		StoredCertificates: NewStoredCertificateRepository(db),
	}
}
```

Service skeleton (real implementation lands in Phase 2):

```go
// internal/modules/certificate/services/stored_certificate_service.go
package services

import (
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
)

type StoredCertificateService struct {
	repos *repositories.Registry
}

func NewStoredCertificateService(repos *repositories.Registry) *StoredCertificateService {
	return &StoredCertificateService{repos: repos}
}
```

DTO + handler skeletons (empty structs / no methods yet — Phase 2
fills these). Module wiring uses the existing `builder` pattern; copy
the entry-point structure from
`internal/modules/docker/module.go`'s constructor.

**Step 4: Verify it compiles**

```bash
go build ./...
```

Expected: no output (success).

**Step 5: Commit**

```bash
git add internal/modules/certificate
git commit -m "certificate: module skeleton — model + repo + service stubs"
```

---

## Task 1.4: Register module with kernel (still no routes)

**Files:**
- Modify: `cmd/api/main.go` — add `certificate.NewModule(builder)` to the module-registration block

**Step 1: Read the existing registration block**

`cmd/api/main.go` around the lines that say `Register(serverModule)`,
`Register(dockerModule)`, etc. Add `certModule := certificate.NewModule(builder)`
above the `kernel.Register(...)` chain and `.Register(certModule)`
into that chain.

**Step 2: Verify boot succeeds**

```bash
make run
```

Expected output: the standard boot log; look for a line like `Module
registered: certificate`. Then Ctrl-C.

**Step 3: Commit**

```bash
git add cmd/api/main.go
git commit -m "certificate: register module with kernel (no routes yet)"
```

---

## Task 1.5: Push Phase 1

```bash
go test ./...
go vet ./...
git push origin feature/stored-certificates
```

Expected: all tests green, vet clean. Phase 1 done — the table exists,
the module compiles, but no behaviour is exposed yet.

---

# Phase 2 — Backend CRUD + Parser + Fingerprint (launch-go)

End state: full `POST /api/certificates`, `GET /api/certificates`,
`GET /api/certificates/:id`, `PATCH /api/certificates/:id`,
`DELETE /api/certificates/:id`, `GET /api/certificates/:id/usages`
work end-to-end. Library is shipping dark — UI doesn't expose it yet.

## Task 2.1: Parser service (TDD)

**Files:**
- Create: `internal/modules/certificate/services/parser.go`
- Create: `internal/modules/certificate/services/parser_test.go`
- Create: `internal/modules/certificate/services/testdata/leaf.pem`
- Create: `internal/modules/certificate/services/testdata/leaf.key`
- Create: `internal/modules/certificate/services/testdata/chain.pem`
- Create: `internal/modules/certificate/services/testdata/expired.pem`
- Create: `internal/modules/certificate/services/testdata/expired.key`
- Create: `internal/modules/certificate/services/testdata/mismatched.key`

**Step 1: Generate test fixtures**

Run once locally to produce reproducible PEM fixtures:

```bash
mkdir -p internal/modules/certificate/services/testdata
cd internal/modules/certificate/services/testdata

# valid leaf cert valid for 1 year, SAN = acme.io + *.acme.io
openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout leaf.key -out leaf.pem -days 365 \
    -subj "/CN=acme.io" \
    -addext "subjectAltName=DNS:acme.io,DNS:*.acme.io"

# expired leaf cert
openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout expired.key -out expired.pem -days -1 \
    -subj "/CN=expired.example"

# a second key, not matching leaf.pem, for mismatch test
openssl genrsa -out mismatched.key 2048

# chain.pem = leaf + a fake intermediate (the leaf cert duplicated;
# we just need ParseCertificate to keep working when extra blocks
# are present)
cat leaf.pem leaf.pem > chain.pem

cd -
```

**Step 2: Write the parser test (RED)**

```go
// internal/modules/certificate/services/parser_test.go
package services_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/certificate/services"
)

func mustRead(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(b)
}

func TestParser_LeafCert_ExtractsAllFields(t *testing.T) {
	parsed, err := services.ParseCertificate(mustRead(t, "leaf.pem"))
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"acme.io", "*.acme.io"}, parsed.Domains)
	assert.Equal(t, "acme.io", parsed.CommonName)
	assert.NotEmpty(t, parsed.Issuer)
	// 365 days from the cert's own NotBefore — independent of wall clock,
	// so the assertion stays valid even as the fixture ages.
	expected := parsed.NotBefore.Add(365 * 24 * time.Hour)
	assert.WithinDuration(t, expected, parsed.NotAfter, 1*time.Minute)
	assert.Len(t, parsed.FingerprintSHA256, 64) // hex chars
}

func TestParser_ChainPEM_UsesLeafForMetadata(t *testing.T) {
	parsed, err := services.ParseCertificate(mustRead(t, "chain.pem"))
	require.NoError(t, err)
	assert.Contains(t, parsed.Domains, "acme.io")
}

func TestParser_ExpiredCert_StillParses_WarnsCaller(t *testing.T) {
	parsed, err := services.ParseCertificate(mustRead(t, "expired.pem"))
	require.NoError(t, err)
	assert.True(t, parsed.NotAfter.Before(time.Now()))
}

func TestParser_Malformed_ReturnsError(t *testing.T) {
	_, err := services.ParseCertificate("not a pem block")
	assert.Error(t, err)
}

func TestParser_KeyCertMatch_OK(t *testing.T) {
	err := services.ValidateKeyMatchesCert(
		mustRead(t, "leaf.pem"),
		mustRead(t, "leaf.key"),
	)
	assert.NoError(t, err)
}

func TestParser_KeyCertMismatch_Errors(t *testing.T) {
	err := services.ValidateKeyMatchesCert(
		mustRead(t, "leaf.pem"),
		mustRead(t, "mismatched.key"),
	)
	assert.Error(t, err)
}
```

```bash
go test ./internal/modules/certificate/services/... -run TestParser -v
```

Expected: compile error or all-fail (functions don't exist yet).

**Step 3: Implement the parser (GREEN)**

```go
// internal/modules/certificate/services/parser.go
package services

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ParsedCertificate is the metadata the service layer extracts from a
// user-supplied cert PEM. Stored verbatim on stored_certificates so
// the picker can render "name · *.acme.io · expires Mar 12" without
// re-parsing every list call.
type ParsedCertificate struct {
	Domains           []string
	CommonName        string
	Issuer            string
	NotBefore         time.Time
	NotAfter          time.Time
	SerialNumber      string
	FingerprintSHA256 string // hex
}

// ParseCertificate decodes the first leaf cert from a PEM blob and
// extracts metadata. PEM blobs containing chain entries are accepted
// — we only inspect the first CERTIFICATE block.
func ParseCertificate(pemContent string) (*ParsedCertificate, error) {
	block, _ := pem.Decode([]byte(pemContent))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a valid PEM-encoded certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	domains := uniqueStrings(append([]string{cert.Subject.CommonName}, cert.DNSNames...))
	// Filter empty CN (some certs only have SANs)
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		if strings.TrimSpace(d) != "" {
			out = append(out, d)
		}
	}

	fingerprint := sha256.Sum256(cert.Raw)

	return &ParsedCertificate{
		Domains:           out,
		CommonName:        cert.Subject.CommonName,
		Issuer:            cert.Issuer.CommonName,
		NotBefore:         cert.NotBefore.UTC(),
		NotAfter:          cert.NotAfter.UTC(),
		SerialNumber:      cert.SerialNumber.String(),
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
	}, nil
}

// ValidateKeyMatchesCert returns nil if the private key matches the
// certificate's public key (i.e. tls.X509KeyPair would succeed).
func ValidateKeyMatchesCert(certPEM, keyPEM string) error {
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		return fmt.Errorf("private key does not match certificate: %w", err)
	}
	return nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
```

```bash
go test ./internal/modules/certificate/services/... -run TestParser -v
```

Expected: all six tests PASS.

**Step 4: Commit**

```bash
git add internal/modules/certificate/services/parser.go internal/modules/certificate/services/parser_test.go internal/modules/certificate/services/testdata
git commit -m "certificate: parser service — x509 metadata extraction + key-cert match"
```

---

## Task 2.2: Service Create — validation + fingerprint dedupe (TDD)

**Files:**
- Modify: `internal/modules/certificate/services/stored_certificate_service.go`
- Create: `internal/modules/certificate/services/stored_certificate_service_test.go`
- Create: `internal/modules/certificate/dto/requests.go` (full content)
- Create: `internal/modules/certificate/dto/responses.go` (full content)

**Step 1: Write the Create test (RED)**

```go
// internal/modules/certificate/services/stored_certificate_service_test.go
package services_test

// ... (uses testhelpers.NewSQLiteDB() or the existing in-mem test DB
// helper that other module tests use — read internal/modules/docker/
// services/registry_credential_service_test.go for the pattern)
```

Tests to cover:
- `TestService_Create_Success` — happy path; row written; fingerprint set.
- `TestService_Create_KeyMismatch_422` — error returned, no row.
- `TestService_Create_MalformedPEM_422` — error returned, no row.
- `TestService_Create_DuplicateFingerprint_409` — second create with same
  cert content returns the existing row's id in the error.
- `TestService_Create_NameCollision_409` — second create with same name
  (different cert) returns 409.

```bash
go test ./internal/modules/certificate/services/... -run TestService_Create -v
```

Expected: all FAIL.

**Step 2: Implement Create**

```go
// internal/modules/certificate/services/stored_certificate_service.go
package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/certificate/dto"
	"github.com/kkz6/launch-go/internal/modules/certificate/models"
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
)

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
		PrivateKey:        []byte(req.PrivateKey), // EncryptedString encrypts on save
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
```

DTO:

```go
// internal/modules/certificate/dto/requests.go
package dto

type CreateStoredCertificateRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Notes       *string `json:"notes" validate:"omitempty,max=2000"`
	Certificate string  `json:"certificate" validate:"required"`
	PrivateKey  string  `json:"private_key" validate:"required"`
}

type UpdateStoredCertificateRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=255"`
	Notes       *string `json:"notes" validate:"omitempty,max=2000"`
	Certificate *string `json:"certificate" validate:"omitempty"`
	PrivateKey  *string `json:"private_key" validate:"omitempty"`
}
```

**Step 3: Run tests**

```bash
go test ./internal/modules/certificate/services/... -run TestService_Create -v
```

Expected: all five tests PASS.

**Step 4: Commit**

```bash
git add internal/modules/certificate/services/stored_certificate_service.go internal/modules/certificate/services/stored_certificate_service_test.go internal/modules/certificate/dto
git commit -m "certificate: service Create with validation + fingerprint dedupe"
```

---

## Task 2.3: Service Update / Delete / DeleteWithForce / Usages

Tests + implementation for:
- `Update` — re-parses metadata if cert/key changes; preserves
  fingerprint dedupe; returns `pending_redeploys` count if cert
  content changed (fanout dispatched in Phase 6).
- `Delete` — refuses if usages > 0; returns `ErrInUse` carrying the
  usage list.
- `DeleteWithForce` — flips referencing rows back to letsencrypt /
  auto + soft-deletes the stored cert. (The redeploy itself is a
  later phase — for now we just clear the FK.)
- `Usages(ctx, teamID, certID)` — joins
  `certificates.stored_certificate_id` and
  `docker_application_domains.stored_certificate_id` to produce
  `[]dto.CertificateUsage{Kind, Name, ID}` results.

Each method gets a dedicated TDD task (write test → fail → implement
→ pass → commit). Three commits in this block:
- `certificate: service Update with content re-parse`
- `certificate: service Delete + DeleteWithForce`
- `certificate: service Usages query`

---

## Task 2.4: HTTP handler + routes

**Files:**
- Modify: `internal/modules/certificate/handlers/stored_certificate_handler.go`
- Modify: `internal/modules/certificate/module.go` (route registration)

**Step 1: Write handler smoke test**

Integration test under `cmd/api` (read `internal/modules/docker/handlers/registry_credential_handler_test.go` as a template) verifying the full request → response round-trip for POST + GET.

**Step 2: Implement handlers**

Method-per-route, mapping service errors to HTTP statuses:

| Service error | HTTP |
|---|---|
| `services.ErrDuplicateFingerprint` | 409, body `{ "existing": {id,name} }` |
| `validator.ValidationErrors` | 422, body `{ "errors": {...} }` |
| `gorm.ErrRecordNotFound` | 404 |
| `services.ErrInUse` | 409, body `{ "usages": [...] }` |
| default | 500 |

**Step 3: Register routes in `module.go`**

```go
func (m *Module) RegisterTeamRoutes(router fiber.Router, authMiddleware fiber.Handler) {
    grp := router.Group("/certificates", authMiddleware)
    grp.Get("/", m.handler.List)
    grp.Post("/", m.handler.Create)
    grp.Get("/:id", m.handler.Get)
    grp.Patch("/:id", m.handler.Update)
    grp.Delete("/:id", m.handler.Delete)
    grp.Get("/:id/usages", m.handler.Usages)
}
```

(Follow the existing module interface in
`internal/pkg/app/kernel.go` — look at how `dockerModule` is wired.)

**Step 4: Smoke test**

```bash
make run &
sleep 2
TOKEN=$(some auth helper)
curl -sX POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke","certificate":"...","private_key":"..."}' \
  http://localhost:8080/api/certificates | jq
```

**Step 5: Commit + push**

```bash
git add internal/modules/certificate
git commit -m "certificate: HTTP handler + routes (POST/GET/PATCH/DELETE + /usages)"
git push origin feature/stored-certificates
```

---

## Task 2.5: Phase 2 closeout

```bash
go test ./...
go vet ./...
```

Both green. Open a PR draft for visibility but don't merge yet — Phase
3 (UI) needs the routes available locally.

---

# Phase 3 — Settings UI (launch-nuxt) — sketch

Detailed plan to be written as
`docs/plans/2026-05-28-stored-certificates-ui.md` once Phase 2 lands.
High-level tasks:

1. `services/certificateService.ts` — CRUD client mirroring `dockerService.registryCredentials`.
2. `components/settings/AddCertificate.vue` — Sheet with name/cert/key/notes; client-side parse preview using minimal JS x509 lib (or just postpone preview to server response).
3. `components/settings/EditCertificate.vue` — similar; "N usages" warning if cert/key fields touched.
4. `ConnectionsTab.vue` — add `SSL Certificates` section between DNS Providers and Docker Registry Credentials. Empty-state pattern matches the standardised one.
5. Spec tests for AddCertificate, EditCertificate.
6. Manual E2E: save, list, edit (name only), edit (cert content with warning), delete (refuse with usages payload, then force).

---

# Phase 4 — PHP Site SSL Wire-Up — sketch

Detailed plan: `docs/plans/2026-05-29-stored-certificates-php-sites.md`.

Backend:
1. `internal/modules/site/dto/requests.go` — add `stored_certificate_id` to update-SSL body.
2. `services/ssl_service.go` — resolve picked stored cert → write
   `certificates` row with `stored_certificate_id` + resolved PEMs.
3. `jobs/install_caddyfile.go` — pre-deploy expiry guard.
4. Tests for each.

Frontend:
1. `components/shared/CertificatePicker.vue` — combobox.
2. `components/site/UpdateSsl.vue` — add 4th radio option + picker.
3. Spec tests.

---

# Phase 5 — Docker Domain Wire-Up — sketch

Detailed plan: `docs/plans/2026-05-30-stored-certificates-docker-domains.md`.

Backend:
1. Add `stored` to `certificate_provider` enum.
2. `tasks/traefik_config.go` — branch: resolve stored cert → write
   files under `/var/lib/launch/traefik/certs/<id>/` → emit
   `tls.certificates` block.
3. `dto/requests.go` + service for create/update domain — accept
   `stored_certificate_id`.
4. Tests.

Frontend:
1. `components/application/CreateDomain.vue` — HTTPS source segment
   + picker re-use.
2. Spec tests.

---

# Phase 6 — Fanout, Expiry Warn, Banner — sketch

Detailed plan: `docs/plans/2026-05-31-stored-certificates-fanout-warn.md`.

Backend:
1. `certificate:fanout` asynq job — dispatched from
   `StoredCertificateService.Update` when cert/key content changes.
   Iterates referencing sites/domains, dispatches the appropriate
   redeploy job per resource.
2. `certificate:warn_expiring` daily asynq job — finds certs
   expiring in next 30 days, dedupes per team via a
   `notified_at` shadow column or a notification-log query.
3. Notification module emit + `certificate.expiring_soon` event
   broadcast.
4. CLAUDE.md updated with new event names per the broadcast
   contract.

Frontend:
1. `composables/useCertificateAlerts.ts` — polls
   `/api/certificates?expiring_in=30d` on dashboard mount.
2. Dashboard banner component (if any certs expiring).
3. `useChannelEvents.ts` allow-list updated with `certificate.*`.

---

# Cross-cutting reminders

- **Encryption**: `private_key` MUST go through
  `dbtype.EncryptedString`. Test that round-tripping (write → read)
  preserves the value.
- **Soft-delete uniqueness**: per-team unique indexes on `name` and
  `fingerprint_sha256` are `WHERE deleted_at IS NULL`. Verify by
  deleting + recreating with the same name/cert in tests.
- **Broadcast contract**: per
  `@CLAUDE.md` ("WebSocket Broadcasting"), every state change visible
  to the UI must broadcast on team channel with
  `EnsureRoutingField`. Add `certificate.*` events to
  `useChannelEvents.ts` allow-list and CLAUDE.md "Known event names".
- **Commit cadence**: each task ends with a commit. Each phase ends
  with a `git push origin feature/stored-certificates`.
- **YAGNI**: no per-cert rotation policies, no key-vault integration,
  no CSR generation in this initiative. The design doc §2 (Out of
  scope) is authoritative — reject scope creep there.
