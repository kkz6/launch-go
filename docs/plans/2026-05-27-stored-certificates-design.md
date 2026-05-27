# Stored SSL Certificates — design

**Status:** accepted
**Date:** 2026-05-27
**Spans:** `launch-go` (backend, data model, jobs) + `launch-nuxt` (UI)
**Predecessor:** none — net-new feature

---

## 1. Goal

Today every SSL custom-cert path makes the user paste the same PEM
content into a per-site dialog. Two sites with `*.acme.io` means the
private key is pasted twice. Docker application domains have no custom-
cert path at all — `https=true` always implies Let's Encrypt.

This initiative adds a **team-scoped certificate library** — the same
shape as `registry_credentials`, `source_controls`, `storage_providers`
— and threads it through both the PHP site SSL flow and the Docker
domain flow. The user pastes a cert once, names it, then picks it from
a dropdown wherever HTTPS lives.

A re-upload of a stored cert fans out to every site/domain that
references it, so cert rotation becomes one paste instead of N.

---

## 2. Scope

### In scope

- New `stored_certificates` table (team-scoped, encrypted private key,
  parsed metadata: domains/issuer/not_before/not_after/fingerprint).
- New `internal/modules/certificate/` Go module with full CRUD +
  parser + fanout job.
- Library UI in `Settings → Connections` (new "SSL Certificates"
  section between DNS Providers and Docker Registry Credentials).
- 4-option SSL dialog on PHP sites: Let's Encrypt / Off / **Stored
  certificate (picker)** / Custom (paste once). Picker is the new
  affordance; Custom path stays.
- Docker domain dialog gains the same picker when `https=true`. The
  Traefik dynamic-config writer learns to emit a `tls.certificates`
  block when `certificate_provider=stored`.
- Daily expiry-warn job + dashboard banner for certs in the next 30
  days.
- Fanout job re-pushes updated certs to all referencing sites/docker
  servers and broadcasts per-resource updated events so the UI
  reflects.

### Out of scope (deferred)

- Auto-renewal of stored certs (the whole point of letsencrypt is that
  it auto-renews; stored certs are explicitly user-managed).
- ACME DNS-01 challenges for letsencrypt — current HTTP-01 path stays.
- Certificate-signing-request (CSR) generation in our UI — user
  generates the cert with their own CA, we just store it.
- Per-cert rotation policies, key-vault integrations.
- Auto-migrating today's per-site pasted certs into the library —
  opt-in only (see §10).

### Existing functionality we are NOT touching

- The site-scoped `certificates` table stays. It still records what's
  currently deployed per site. We add a single nullable FK column to
  point at the source `stored_certificates` row.
- The `install_caddyfile` site job's on-disk layout
  (`<site>/certificates/<id>/{certificate.cert,private.key}`) is
  unchanged. Only the byte source changes.
- Traefik letsencrypt path is unchanged when the user picks Let's
  Encrypt; we only add a parallel code path for stored certs.

---

## 3. Decisions captured during brainstorming

| Question | Decision |
|---|---|
| Cert scope | Team-scoped (mirrors `registry_credentials`) |
| SSL dialog options | 4: Let's Encrypt / Off / Stored / Custom (paste) |
| Metadata strategy | Parse on save — domains, expiry, issuer, fingerprint |
| Picker filtering | Show all team certs, no strict-match; highlight expiring-soon |
| Library UI location | New section in `Settings → Connections` |
| Migration of today's pasted certs | Opt-in; don't auto-migrate secrets across scopes |
| Cert re-upload behaviour | Fanout job re-pushes to all referencing sites + docker servers |

---

## 4. Data model (launch-go)

### 4.1 New table — `stored_certificates`

```
id                  char(26) pk            -- ULID
team_id             char(26) not null, fk teams
user_id             char(26) null          -- creator, audit only
name                varchar(255) not null  -- display label
notes               text null
certificate         longtext not null      -- PEM, leaf + chain
private_key         longtext not null      -- ENCRYPTED at rest (dbtype.EncryptedString)

-- Parsed-on-save metadata (server-derived; never user-supplied):
domains             jsonb not null         -- ["acme.io","*.acme.io"]
common_name         varchar(255) null
issuer              varchar(255) null      -- "Let's Encrypt R3"
not_before          timestamp not null
not_after           timestamp not null
serial_number       varchar(255) null
fingerprint_sha256  varchar(64) null       -- DER fingerprint of leaf cert

created_at          timestamp not null
updated_at          timestamp not null
deleted_at          timestamp null         -- soft-delete

unique (team_id, lower(name)) where deleted_at is null
unique (team_id, fingerprint_sha256) where deleted_at is null
index  (team_id, not_after)              -- expiring-soon queries
```

`private_key` uses `dbtype.EncryptedString` (same helper
`registry_credentials.password` uses). `json:"-"` keeps it off the
wire.

### 4.2 Schema changes to existing tables

```sql
ALTER TABLE certificates
  ADD COLUMN stored_certificate_id char(26) NULL
  REFERENCES stored_certificates(id);

ALTER TABLE docker_application_domains
  ADD COLUMN stored_certificate_id char(26) NULL
  REFERENCES stored_certificates(id);
```

`docker_application_domains.certificate_provider` enum gains a new
value `stored`. Existing rows keep `letsencrypt`.

---

## 5. Backend API surface (launch-go)

New module: `internal/modules/certificate/` with the standard subdirs
(`dto`, `handlers`, `jobs`, `models`, `repositories`, `services`,
`tasks`).

Routes (mounted under `/api/certificates`, team-scoped via the
existing team middleware):

```
GET    /api/certificates                  list (paginated)
POST   /api/certificates                  create
GET    /api/certificates/:id              read (includes parsed metadata)
PATCH  /api/certificates/:id              update
DELETE /api/certificates/:id              soft-delete
GET    /api/certificates/:id/usages       list {kind, name, id}
                                           of sites/domains pointing here
```

### POST/PATCH body

```json
{
  "name": "acme-wildcard",
  "certificate": "-----BEGIN CERTIFICATE-----\n...",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...",
  "notes": "Cloudflare-issued; rotated quarterly"
}
```

### Validation

- `tls.X509KeyPair(certPEM, keyPEM)` — refuses 422 with field error
  `private_key: "does not match the certificate"` on mismatch.
- Parse the leaf with `x509.ParseCertificate` — extract CN, SANs,
  `NotBefore`, `NotAfter`, `Issuer.CommonName`, `SerialNumber`,
  `sha256(rawCert)`. Reject 422 if parsing fails ("not a valid
  certificate PEM").
- Fingerprint dedupe per team: 409 with the existing cert's name + id.
- Expired-already cert: warn (banner in API response `warnings` field)
  but allow — sometimes a user pastes an old cert for testing.

### Cascading on PATCH

When `certificate` or `private_key` changes, the service:
1. Re-parses metadata (refreshing `domains`, `not_after`, etc.).
2. Looks up referencing sites (`certificates.stored_certificate_id`)
   and docker domains (`docker_application_domains.stored_certificate_id`).
3. Dispatches an asynq fanout job `certificate:fanout`.
4. Returns immediately with `pending_redeploys: N`.

### DELETE

Refuses 409 with the `usages` payload if any rows still reference the
cert. `force=true` query param flips referencing sites/domains back to
Let's Encrypt (sets `tls_setting=auto` on sites, `certificate_provider=letsencrypt`
on docker domains) and dispatches redeploy jobs before soft-deleting.

### Expiry warn job

`certificate:warn_expiring` — runs daily, finds rows where `not_after
BETWEEN now() AND now()+30d` and the team hasn't been notified in the
last 7 days. Emits `certificate.expiring_soon` event + uses the
notification module to email/Slack the team.

---

## 6. Site + Docker integration

### 6.1 PHP site SSL — `PATCH /api/servers/:serverId/sites/:siteId/ssl`

Request body gains `stored_certificate_id`. When set:
- `tls_setting` is coerced to `custom`.
- Service fetches the cert + key from `stored_certificates`.
- Updates the site's `certificates` row with the resolved PEMs AND
  `stored_certificate_id` (so cascading works later).
- `install_caddyfile` job runs as today — no change to on-disk layout.

When `stored_certificate_id` is null and `tls_setting=custom`, the
existing paste-inline path runs unchanged.

### 6.2 Docker domain — `POST/PATCH /api/.../docker/.../domains/:id`

Request body gains `stored_certificate_id`. When set:
- `certificate_provider` is coerced to `stored`.
- Domain row stores the FK.

`tasks/traefik_config.go` writer (current branches on
`certificate_provider`):

```
certificate_provider=letsencrypt  → emit `certresolver: letsencrypt`
                                     (current behaviour)
certificate_provider=stored       → resolve stored_certificate_id → cert + key
                                     → write to /var/lib/launch/traefik/certs/<cert_id>/{cert.pem,key.pem}
                                       (idempotent: fingerprint-checked, only re-written on change)
                                     → emit `tls.certificates` block:
                                         tls:
                                           certificates:
                                             - certFile: /var/lib/launch/traefik/certs/<cert_id>/cert.pem
                                               keyFile:  /var/lib/launch/traefik/certs/<cert_id>/key.pem
```

### 6.3 Pre-deploy expiry guard

Both the site `install_caddyfile` job and the docker `traefik_config`
writer check `not_after > now()` before applying a stored cert. If
expired, the job fails fast with a clear error: *"stored certificate
'<name>' expired N days ago — re-upload or switch to Let's Encrypt"*.

---

## 7. Frontend (launch-nuxt)

### 7.1 Library — `Settings → Connections → SSL Certificates`

New section in `ConnectionsTab.vue` between DNS Providers and Docker
Registry Credentials. Empty-state pattern matches the standardised
bordered-card with icon + message + CTA we landed earlier this
session.

Components under `components/settings/`:
- `AddCertificate.vue` — Sheet with `name`, `certificate` (monospace
  textarea), `private_key` (monospace textarea, never readable back
  after save), `notes`. Live client-side parse preview using
  `pkijs` (or hand-rolled regex + minimal ASN.1) shows
  domains/issuer/expiry as the user pastes — server re-parses on save.
- `EditCertificate.vue` — same shape; warns *"N sites + M docker
  domains use this cert. Saving will redeploy them."* before submit
  if cert/key changed.

### 7.2 Shared picker — `components/shared/CertificatePicker.vue`

Combobox showing `name · primary-SAN · expires {date}`. Expiring-soon
items (<30d) get a warning tint. Used in both the PHP SSL dialog and
the Docker domain dialog.

### 7.3 PHP site SSL dialog — `components/site/UpdateSsl.vue`

Add a 4th radio (`stored`) between `auto` and `custom`. Selecting it
hides the two PEM textareas and renders `<CertificatePicker>` instead.
The submit handler sends `stored_certificate_id` (and omits the
`certificate` / `private_key` body fields).

### 7.4 Docker domain dialog — `components/application/CreateDomain.vue`

Add an `HTTPS source` segment that appears when `https=true`:
- Let's Encrypt (default)
- Stored certificate → renders `<CertificatePicker>`

### 7.5 Expiring-soon banner

`useCertificateAlerts` composable polls
`GET /api/certificates?expiring_in=30d` on dashboard mount. If results,
shows a banner above the dashboard: *"2 certificates expire in the
next 30 days — review"*. Click → routes to Connections tab with the
SSL Certificates section scrolled into view.

---

## 8. WebSocket events

Per CLAUDE.md broadcast contract — both API path (Mixin) and worker
path (RedisBroadcaster) must call `EnsureRoutingField`. Each event
must appear in `useChannelEvents.ts` allow-list AND in CLAUDE.md
"Known event names".

**Team channel** new events:
- `certificate.created` / `certificate.updated` / `certificate.deleted`
- `certificate.expiring_soon` (from the daily warn job)

**Existing channels** new events (formalised):
- `site.ssl_updated` — emitted by `install_caddyfile` on success.
- `application.tls_updated` — emitted by `traefik_config` task on
  success.

---

## 9. Error handling

| Scenario | Response |
|---|---|
| Cert PEM unparseable | 422 `certificate: "not a valid PEM-encoded certificate"` |
| Key doesn't match cert | 422 `private_key: "does not match the certificate"` |
| Duplicate fingerprint (per team) | 409 `{existing: {id, name}}` — UI offers "Open existing" |
| Name collision (per team, alive only) | 409 `name: "already in use by another stored certificate"` |
| Already-expired cert on save | 200 with `warnings: ["certificate expired N days ago"]` (allowed for testing) |
| Delete with live references | 409 `{usages: [{kind, name, id}, ...]}` — UI shows count + force option |
| Stored cert expired at deploy time | Deploy job fails fast with named error before pushing |

---

## 10. Rollout phases

Six steps, each its own PR on the feature branch. Each step ends with
`go test ./... && go vet ./... && npm run test` green plus a manual
smoke test.

1. **Migration + module skeleton.** Adds the `stored_certificates`
   table, the two FK columns on `certificates` and
   `docker_application_domains`, and the `internal/modules/certificate/`
   skeleton (model, repository, empty service). No routes exposed.
2. **Backend CRUD + parser + fingerprint dedupe.** Full service +
   handler + routes. Validation. Tests against PEM fixtures (real cert
   + chain, mismatched key, expired, malformed). Ship dark — UI
   doesn't expose it yet.
3. **Settings → Connections SSL Certificates section.** Library UI
   (Add/Edit/Delete). Users can create stored certs but nothing
   references them yet.
4. **PHP site SSL dialog wires picker** + `install_caddyfile` job
   pulls bytes from stored cert when FK set. End-to-end.
5. **Docker domain wires picker** + `tasks/traefik_config.go` handles
   `certificate_provider=stored`. End-to-end on docker side.
6. **Fanout job** (re-upload re-pushes to all referencing
   sites/domains) + **expiry-warn job** + **dashboard banner**.

### Migration of today's pasted certs

Not auto-migrated. Existing per-site `certificates` rows stay as-is.
Users wanting reuse open Connections → SSL Certificates → Add and
paste again. Rationale: moving customer secrets between scopes
silently is the kind of thing that bites you. Future tooling could
offer "promote this site's cert to the library" as a one-click
action if real demand surfaces.

---

## 11. Testing

- **Unit (Go)**
  - `certificate/services/parser_test.go` — PEM fixtures: valid leaf-
    only, valid leaf+chain, expired, future-not-before, malformed,
    EC vs RSA keys, mismatched key.
  - `certificate/services/service_test.go` — fingerprint dedupe,
    name collision, soft-delete with live refs, force-delete
    cascading, fanout dispatch.
- **Unit (Vue)**
  - `CertificatePicker.spec.ts` — empty state, expiring-soon
    highlight, keyboard navigation.
  - `AddCertificate.spec.ts` — client-side parse preview, validation
    error display.
- **Integration**
  - `cmd/api` route smoke test for CRUD round-trip.
  - Fanout job integration test using `httptest` + an SSH stub
    server.
- **Manual E2E gates** (covered by the rollout phase verification)
  - (a) Save a `*.acme.io` cert, use it on two sites, re-upload,
    confirm both sites end up serving the new cert without manual
    re-deploy.
  - (b) Save a cert, use it on a Docker domain, confirm Traefik
    serves it.
  - (c) Force-delete with-references, confirm referencing sites flip
    to Let's Encrypt and redeploy.
