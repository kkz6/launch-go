# SSH Certificate-Based Authentication — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace launch-go's long-lived shared SSH private keys with short-lived per-session SSH certificates signed by a Launch-owned Certificate Authority — the same pattern DigitalOcean's web console uses.

**Architecture:** A single CA keypair (per Launch installation, not per team — simpler operational model). The CA private key lives in `ssh_certificate_authorities.private_key` encrypted with the same `dbtype.EncryptedString` envelope `servers.private_key` already uses. Each managed server's sshd is configured with `TrustedUserCAKeys /etc/ssh/launch-ca.pub` so it trusts any user cert signed by us. Whenever a Launch user opens a terminal, fires a deploy, or runs a backup, we mint a fresh certificate (15-minute lifetime for interactive, 1-hour for jobs), embed the user's identity in the `KeyId`, and the SSH client uses cert auth instead of the static private key. Result: nothing long-lived sits on the target server beyond the CA public key; every session is auditable by Launch user.

**Tech Stack:** Go `golang.org/x/crypto/ssh` (already vendored — provides `ssh.NewCertSigner` + cert serialization). Postgres for CA storage + audit log. `install_launch_agent.sh` extended for CA install on provision; one-off backfill job for existing servers. No new external infra required for v1 — KMS/Vault for the CA private key is a v2 follow-up documented at the end.

**Rollout posture:** Strict phasing. Phase 1 ships the CA and dual-path SSH client (cert OR key) without behaviour change. Phase 2 flips the terminal WebSocket. Phase 3 flips deploy/backup jobs. Phase 4 removes the long-lived user key entirely (keeps a single root key per server for break-glass). Each phase is an independent PR with a rollback knob (env flag) so we can revert without re-migrating.

**Threat model addressed:**
- ✅ Long-lived shared key sitting on every server (current state)
- ✅ No per-user audit trail on the target (sshd just logs `launcher`)
- ✅ Revoking a Launch user requires touching `authorized_keys` on every server
- ✅ If `APP_KEY` leaks, every fleet server is owned forever
- ⏸️ Out of scope for this plan: moving APP_KEY → KMS (separate work, documented at end)

---

## Pre-flight findings (audited 2026-05-31)

- ✅ `golang.org/x/crypto/ssh` is already in `go.mod` (used by the existing taskrunner). It exposes `ssh.Certificate`, `ssh.NewCertSigner`, `ssh.MarshalAuthorizedKey` — everything cert minting needs. No new deps.
- ✅ `dbtype.EncryptedString` (`internal/pkg/dbtype/`) is the existing Crypt envelope. Reuse it for the CA private key; same pattern as `servers.private_key`.
- ✅ Host key TOFU from PR #55 (just merged) lays the foundation: it added per-server `ServerID` + `HostKey` plumbing through `Connection` → `SSHConfig` → `makeHostKeyCallback`. Cert auth will hook into the same plumbing.
- ✅ Provisioning has `install_launch_agent.sh` (already runs `tee /etc/systemd/system/launch-agent.service`) — adding `TrustedUserCAKeys` to sshd_config is the same shape.
- ✅ Activity logging is in place (`internal/pkg/launch/activity`). Cert mint events ride on it.
- ⚠️ The current `Connection.PrivateKey` field is used by every SSH path in the codebase. We don't remove it in Phase 1 — we add a parallel `Certificate` field that takes precedence when present.

---

## Slicing

| # | Slice | Touches | Verification gate |
|---|---|---|---|
| A | CA model + repo + bootstrap | `internal/modules/ssh_ca/` (new module), migrations | `go test ./...` green; admin CLI mints CA on first boot |
| B | Cert signing service | `internal/pkg/sshcert/` (new pkg) | unit tests round-trip a cert through `ssh.ParseAuthorizedKey` |
| C | SSH client gains cert auth path | `internal/pkg/taskrunner/ssh_client.go`, `connection.go` | with-cert path verified against an in-process sshd test fixture |
| D | Provisioning installs CA pub key | `install_launch_agent.sh` + new `install_ssh_ca.sh` step | fresh provision lands `/etc/ssh/launch-ca.pub` + `TrustedUserCAKeys` |
| E | Backfill job for existing servers | new `server:install_ssh_ca` task + admin endpoint | manual run against one server flips it to cert-trust |
| F | Audit log table + cert-mint recording | migration + service-side hook | every mint creates a row; user, server, principals, validity, key_id |
| G | Terminal WS uses cert auth | `internal/modules/websocket/handlers/terminal.go` | interactive shell opens with `Authentication succeeded (publickey-cert)` |
| H | Deploy/backup jobs use cert auth | `internal/modules/site/jobs/`, `internal/modules/backup/jobs/` | live deploy completes via cert; backup run uploads to S3 |
| I | Remove per-user long-lived key | `Server.ConnectionAsUser` returns cert-only; keep root key for break-glass | every interactive path passes; one root key stays per server |
| J | UI: CA settings page + audit log viewer | launch-nuxt | view CA fingerprint, rotate button, audit table |
| K | KRL (Key Revocation List) hook | `ssh_ca` service + sshd_config addition | revoke endpoint; revoked cert fails in next session |

Slices A–C are foundational (no behaviour change). D–E flip provisioning. F adds audit. G is the first user-visible behaviour change (interactive terminal). H extends to jobs. I retires the legacy path. J is the operator UX. K is the late-stage hardening.

Each slice ships as its own PR off `feature/ssh-cert-auth`. Phases A–F = MVP for terminal-only cert auth. Phase G–I = full migration. Phase J–K = polish + revocation.

---

## Slice A — CA model + repo + bootstrap

**Files:**
- Create: `internal/database/migrations/0056_06_01_120000_create_ssh_certificate_authorities.go`
- Create: `internal/modules/ssh_ca/module.go`
- Create: `internal/modules/ssh_ca/models/authority.go`
- Create: `internal/modules/ssh_ca/repositories/repository.go`
- Create: `internal/modules/ssh_ca/services/service.go`
- Create: `cmd/launchctl/sshca.go` (admin CLI subcommand)
- Test: `internal/modules/ssh_ca/services/service_test.go`

**Step 1: Migration — table schema**

The table holds **one row, always** — singleton. Columns:
- `id` (ulid)
- `private_key` (longtext, encrypted) — PEM-encoded ed25519 private key
- `public_key` (text) — `ssh-ed25519 AAAA…` for `/etc/ssh/launch-ca.pub`
- `fingerprint` (varchar 100) — SHA256 fingerprint, displayed in UI
- `key_algo` (varchar 50) — `"ed25519"` (room for future RSA)
- `created_at`, `updated_at`

```go
func createSSHCertificateAuthoritiesUp(db *gorm.DB) error {
    return db.Exec(`CREATE TABLE IF NOT EXISTS ssh_certificate_authorities (
        id CHAR(26) PRIMARY KEY,
        private_key LONGTEXT NOT NULL,
        public_key TEXT NOT NULL,
        fingerprint VARCHAR(100) NOT NULL,
        key_algo VARCHAR(50) NOT NULL DEFAULT 'ed25519',
        created_at TIMESTAMP NULL,
        updated_at TIMESTAMP NULL
    )`).Error
}
```

**Step 2: Write the failing test for `Service.Bootstrap`**

`Bootstrap` must:
- If no row exists → generate ed25519 keypair, encrypt private, store row, return CA.
- If row exists → return existing CA.

```go
func TestBootstrap_CreatesNew(t *testing.T) {
    svc := newServiceWithMemoryDB(t)
    ca, err := svc.Bootstrap(context.Background())
    require.NoError(t, err)
    assert.NotEmpty(t, ca.PublicKey)
    assert.Contains(t, ca.PublicKey, "ssh-ed25519 ")
    assert.NotEmpty(t, ca.Fingerprint)
}

func TestBootstrap_Idempotent(t *testing.T) {
    svc := newServiceWithMemoryDB(t)
    a, _ := svc.Bootstrap(context.Background())
    b, _ := svc.Bootstrap(context.Background())
    assert.Equal(t, a.ID, b.ID)
    assert.Equal(t, a.PublicKey, b.PublicKey)
}
```

**Step 3: Run test, expect fail**

```
go test ./internal/modules/ssh_ca/services/ -run TestBootstrap -count 1
```
Expected: FAIL (Service doesn't exist yet).

**Step 4: Implement `Service.Bootstrap`**

```go
package services

func (s *Service) Bootstrap(ctx context.Context) (*models.Authority, error) {
    existing, err := s.repo.FindOne(ctx)
    if err == nil && existing != nil {
        return existing, nil
    }
    if err != nil && !repository.IsNotFound(err) {
        return nil, err
    }
    // Generate ed25519 keypair
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return nil, err
    }
    sshPub, err := ssh.NewPublicKey(pub)
    if err != nil {
        return nil, err
    }
    pemPriv, err := ssh.MarshalPrivateKey(priv, "launch-ca")
    if err != nil {
        return nil, err
    }
    ca := &models.Authority{
        PublicKey:   string(ssh.MarshalAuthorizedKey(sshPub)),
        Fingerprint: ssh.FingerprintSHA256(sshPub),
        KeyAlgo:     "ed25519",
    }
    ca.PrivateKey.Set(string(pem.EncodeToMemory(pemPriv)))
    return s.repo.Create(ctx, ca)
}
```

**Step 5: Run tests, expect pass**

```
go test ./internal/modules/ssh_ca/services/ -run TestBootstrap -count 1
```
Expected: PASS.

**Step 6: Wire `Bootstrap` to API/worker boot**

In `cmd/api/main.go` and `cmd/worker/main.go`, after module registration:
```go
sshCAModule := ssh_ca.NewModule(builder)
if _, err := sshCAModule.Service().Bootstrap(ctx); err != nil {
    a.logger.Fatal().Err(err).Msg("Failed to bootstrap SSH CA")
}
```

Process refuses to start if CA bootstrap fails. There is **always exactly one CA** by the time any other code runs.

**Step 7: Admin CLI subcommand — show fingerprint**

`cmd/launchctl/sshca.go`:
```go
launchctl sshca show
# Outputs:
# Fingerprint: SHA256:abcd1234...
# Public key:  ssh-ed25519 AAAA…
# Created:     2026-05-31T21:00:00Z
```

Operators paste the public key into a known-good file when they want to audit; no mystery about what they trust.

**Step 8: Commit**

```bash
git add internal/database/migrations/0056_06_01_120000_create_ssh_certificate_authorities.go \
        internal/modules/ssh_ca/ cmd/launchctl/sshca.go \
        cmd/api/main.go cmd/worker/main.go
git commit -m "feat(ssh-ca): bootstrap SSH certificate authority on first boot"
```

---

## Slice B — Cert signing service

**Files:**
- Create: `internal/pkg/sshcert/signer.go`
- Test: `internal/pkg/sshcert/signer_test.go`

**Step 1: Define the signer interface**

```go
type CertRequest struct {
    UserID     string        // Launch user — embedded in cert KeyId for audit
    ServerID   string        // For activity log; not validated against cert content
    Principals []string      // Linux usernames the cert is valid for ("launcher", "root")
    Validity   time.Duration // 15min for interactive, 1h for jobs
    PubKey     ssh.PublicKey // The client's ephemeral pubkey we sign
}

type Signer interface {
    Sign(ctx context.Context, req CertRequest) (*ssh.Certificate, error)
}
```

**Step 2: Write the failing test**

```go
func TestSigner_RoundTrip(t *testing.T) {
    ca := bootstrapCA(t)
    signer := NewSigner(ca)
    // Generate an ephemeral client keypair (what the SSH client will use)
    _, clientPriv, _ := ed25519.GenerateKey(rand.Reader)
    clientSigner, _ := ssh.NewSignerFromKey(clientPriv)
    cert, err := signer.Sign(context.Background(), CertRequest{
        UserID:     "user-abc",
        ServerID:   "srv-xyz",
        Principals: []string{"launcher"},
        Validity:   15 * time.Minute,
        PubKey:     clientSigner.PublicKey(),
    })
    require.NoError(t, err)
    // Cert should be valid right now
    assert.LessOrEqual(t, cert.ValidAfter, uint64(time.Now().Unix()))
    assert.Greater(t, cert.ValidBefore, uint64(time.Now().Unix()))
    // KeyId carries user — sshd will log this
    assert.Equal(t, "launch-user:user-abc", cert.KeyId)
    // Verify the cert signature against the CA public key
    err = (&ssh.CertChecker{}).CheckCert("launcher", cert)
    require.NoError(t, err)
}
```

**Step 3: Run test, expect fail.**

**Step 4: Implement Signer**

```go
func (s *signer) Sign(ctx context.Context, req CertRequest) (*ssh.Certificate, error) {
    nonce := make([]byte, 32)
    rand.Read(nonce)
    cert := &ssh.Certificate{
        Key:             req.PubKey,
        Serial:          uint64(time.Now().UnixNano()),
        CertType:        ssh.UserCert,
        KeyId:           fmt.Sprintf("launch-user:%s", req.UserID),
        ValidPrincipals: req.Principals,
        ValidAfter:      uint64(time.Now().Add(-30 * time.Second).Unix()),
        ValidBefore:     uint64(time.Now().Add(req.Validity).Unix()),
        Permissions: ssh.Permissions{
            CriticalOptions: map[string]string{},
            Extensions: map[string]string{
                // Stops X11/port-forwarding by default — interactive shells only need pty + shell.
                "permit-pty": "",
            },
        },
        Nonce: nonce,
    }
    if err := cert.SignCert(rand.Reader, s.caSigner); err != nil {
        return nil, err
    }
    return cert, nil
}
```

**Step 5: Run tests, expect pass.**

**Step 6: Commit**

```bash
git add internal/pkg/sshcert/
git commit -m "feat(sshcert): cert signing service backed by the CA"
```

---

## Slice C — SSH client gains cert auth path

**Files:**
- Modify: `internal/pkg/taskrunner/connection.go`
- Modify: `internal/pkg/taskrunner/ssh_client.go`
- Test: `internal/pkg/taskrunner/cert_auth_test.go`

**Step 1: Add Certificate field to Connection + SSHConfig**

```go
type Connection struct {
    // ... existing fields ...
    // Certificate is the wire-format SSH cert + signer. When set, takes
    // precedence over PrivateKey. Lifetime is bound to whoever
    // populated it — typically minted just before the connect.
    Certificate *ssh.Certificate
    CertSigner  ssh.Signer // The ephemeral key the cert was signed for
}
```

**Step 2: Build a `ssh.AuthMethod` from cert + ephemeral key**

In `NewSSHClient`:
```go
if cfg.Certificate != nil && cfg.CertSigner != nil {
    certSigner, err := ssh.NewCertSigner(cfg.Certificate, cfg.CertSigner)
    if err != nil {
        return nil, fmt.Errorf("build cert signer: %w", err)
    }
    authMethods = append(authMethods, ssh.PublicKeys(certSigner))
}
```

Cert path **is in addition to** the existing PrivateKey path — both registered as auth methods. SSH negotiates highest priority. During Phase 1 we leave PrivateKey as a fallback so any path that hasn't been migrated yet still works.

**Step 3: Test — cert auth against an in-process sshd**

`golang.org/x/crypto/ssh` ships a server-side helper. Stand up a tiny sshd that:
- Trusts our CA public key
- Closes the channel immediately on success (we just need to know auth worked)

```go
func TestSSHClient_CertAuth(t *testing.T) {
    ca := setupCA(t)
    sshd := newTestServerTrustingCA(t, ca.PublicSigner())
    defer sshd.Close()

    cert, ephSigner := mintCertForTest(t, ca, []string{"launcher"})
    conn := &Connection{
        Host: sshd.Host, Port: sshd.Port, User: "launcher",
        Certificate: cert, CertSigner: ephSigner,
    }
    client, err := conn.NewSSHClient()
    require.NoError(t, err)
    require.NoError(t, client.Connect())
    defer client.Close()
}
```

**Step 4: Run test, expect pass.**

**Step 5: Commit**

```bash
git add internal/pkg/taskrunner/
git commit -m "feat(taskrunner): SSH client supports cert-based auth alongside legacy private key"
```

---

## Slice D — Provisioning installs CA pub key

**Files:**
- Modify: `internal/modules/server/tasks/templates/software/install_launch_agent.sh`
  *(or, cleaner: new dedicated `install_ssh_ca.sh` that runs alongside)*
- Create: `internal/modules/server/tasks/templates/provision/install_ssh_ca.sh`
- Modify: `internal/modules/server/tasks/templates/install_launch_agent.go` (Go side that fills in templates with CA pub key)

**Step 1: Provisioning shell template**

```bash
#!/usr/bin/env bash
set -euo pipefail

CA_PUB="{{ .CAPublicKey }}"

# Write the CA public key. World-readable (0644) is correct for sshd.
echo "$CA_PUB" | sudo tee /etc/ssh/launch-ca.pub > /dev/null
sudo chown root:root /etc/ssh/launch-ca.pub
sudo chmod 0644 /etc/ssh/launch-ca.pub

# Tell sshd to trust certs signed by this CA. Idempotent — drop a
# fragment file under sshd_config.d so we don't fight with other
# package management.
sudo mkdir -p /etc/ssh/sshd_config.d
cat <<EOF | sudo tee /etc/ssh/sshd_config.d/50-launch-ca.conf > /dev/null
TrustedUserCAKeys /etc/ssh/launch-ca.pub
EOF

# Reload sshd. We don't restart — restart drops existing sessions.
sudo systemctl reload ssh || sudo systemctl reload sshd
```

**Step 2: Run during provision**

The existing provision flow calls a chain of scripts. Add this one **after** SSH security hardening but **before** dropping the bootstrap root key (the order is critical — if we add the CA trust then immediately drop the only working key, a buggy CA install would lock us out). The provision step:
1. Install CA pub key
2. Verify the file is in place + sshd config valid (`sshd -t`)
3. Reload sshd
4. Only THEN drop the bootstrap root key

**Step 3: Golden test for the template**

```go
func TestInstallSSHCA_Template(t *testing.T) {
    out := renderInstallSSHCAScript("ssh-ed25519 AAAATEST")
    assert.Contains(t, out, "ssh-ed25519 AAAATEST")
    assert.Contains(t, out, "TrustedUserCAKeys /etc/ssh/launch-ca.pub")
    assert.Contains(t, out, "sshd -t")
}
```

**Step 4: Manual verify on a real provision**

Run provision against a fresh DigitalOcean droplet. After completion:
```bash
ssh launcher@<host> "cat /etc/ssh/launch-ca.pub /etc/ssh/sshd_config.d/50-launch-ca.conf"
```
Expected: the CA pub key (matches `launchctl sshca show`) + the `TrustedUserCAKeys` line.

**Step 5: Commit**

```bash
git add internal/modules/server/tasks/templates/
git commit -m "feat(provision): install launch CA pub key + sshd trust on new servers"
```

---

## Slice E — Backfill job for existing servers

**Files:**
- Create: `internal/modules/server/jobs/install_ssh_ca.go`
- Modify: `internal/modules/server/jobs/register.go`
- Test: `internal/modules/server/jobs/install_ssh_ca_test.go`

The provisioning step from D only fires for new servers. Existing servers need a one-time backfill — dispatch a job per server, each runs the same script over the existing SSH connection (legacy private key auth, since cert auth isn't usable yet on these boxes).

**Step 1: Job that runs the install script over SSH**

```go
type InstallSSHCAJob struct {
    Deps    *JobDeps
    Payload InstallSSHCAPayload
}

type InstallSSHCAPayload struct {
    ServerID string `json:"server_id"`
}

func (j *InstallSSHCAJob) Handle(ctx context.Context) error {
    server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
    if err != nil {
        return err
    }
    ca, err := j.Deps.SSHCAService.Bootstrap(ctx)
    if err != nil {
        return err
    }
    script := renderInstallSSHCAScript(ca.PublicKey)
    task := taskrunner.NewBaseTask(
        taskrunner.WithName("Install Launch SSH CA"),
        taskrunner.WithScript(script),
        taskrunner.WithTimeoutSeconds(60),
    )
    result, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
    if err != nil {
        return err
    }
    if !result.IsSuccessful() {
        return fmt.Errorf("install ssh ca failed: %s", result.GetOutput())
    }
    return nil
}
```

**Step 2: Admin endpoint to enqueue per-server or fleet-wide**

```
POST /admin/ssh-ca/install/:serverId       — one server
POST /admin/ssh-ca/install                  — entire team's fleet
```

The endpoint is gated to team admins. It enqueues `InstallSSHCAJob` per server.

**Step 3: Manual run against the dev fleet**

```
POST /admin/ssh-ca/install
```

Expected: per-server async job; check the status console for completion. Verify on one server via SSH that the `launch-ca.pub` file is now there.

**Step 4: Commit**

```bash
git add internal/modules/server/jobs/install_ssh_ca.go internal/modules/server/jobs/register.go
git commit -m "feat(ssh-ca): backfill job to install CA on existing servers"
```

---

## Slice F — Audit log table + cert-mint recording

**Files:**
- Create: `internal/database/migrations/0057_06_02_000000_create_ssh_cert_audit.go`
- Create: `internal/modules/ssh_ca/models/audit.go`
- Modify: `internal/pkg/sshcert/signer.go` (call audit recorder on every Sign)
- Test: `internal/modules/ssh_ca/services/audit_test.go`

**Step 1: Audit table**

```go
CREATE TABLE IF NOT EXISTS ssh_cert_audits (
    id CHAR(26) PRIMARY KEY,
    user_id CHAR(26) NOT NULL,
    team_id CHAR(26) NOT NULL,
    server_id CHAR(26),
    serial BIGINT NOT NULL,                -- cert serial, matches sshd_log "Accepted publickey for ... CertID 12345"
    key_id VARCHAR(255) NOT NULL,          -- "launch-user:01abc…"
    principals VARCHAR(500) NOT NULL,      -- comma-joined ("launcher,root")
    valid_after TIMESTAMP NOT NULL,
    valid_before TIMESTAMP NOT NULL,
    purpose VARCHAR(50) NOT NULL,          -- "terminal" | "deploy" | "backup" | "command"
    created_at TIMESTAMP NULL,
    INDEX idx_audit_user (user_id, created_at DESC),
    INDEX idx_audit_server (server_id, created_at DESC)
);
```

**Step 2: Recorder hook**

`Signer.Sign` calls a `Recorder.Record(certAudit)` after a successful sign. The recorder is injected — tests use a no-op or in-memory implementation, prod writes to the audit table.

**Step 3: Test that every Sign produces exactly one audit row**

```go
func TestSign_RecordsAudit(t *testing.T) {
    rec := &fakeRecorder{}
    signer := NewSigner(ca, rec)
    signer.Sign(ctx, validReq)
    assert.Len(t, rec.records, 1)
    assert.Equal(t, "launch-user:user-abc", rec.records[0].KeyId)
}
```

**Step 4: Commit**

```bash
git add internal/database/migrations/0057_*.go internal/modules/ssh_ca/ internal/pkg/sshcert/
git commit -m "feat(ssh-ca): record every cert mint to ssh_cert_audits"
```

---

## Slice G — Terminal WebSocket uses cert auth

**Files:**
- Modify: `internal/modules/websocket/handlers/terminal.go`

This is the first user-visible behaviour change. Before each shell session:
1. Generate an ephemeral ed25519 keypair (in-memory, never persisted).
2. Mint a cert via `Signer.Sign` with `UserID = current Launch user`, `Principals = ["launcher"]` (or `["root"]` if the user picked root), `Validity = 15m`, `Purpose = "terminal"`.
3. Pass cert + ephemeral signer through `Connection.Certificate` + `Connection.CertSigner`.
4. SSH connects with cert auth.
5. When the session ends, the ephemeral key + cert just go out of scope.

**Step 1: Modify the terminal handler**

In `handleTerminalSession` (the function that builds the SSHClient), before the `sshClient.Connect()`:

```go
ca, err := h.sshCAService.Bootstrap(ctx)
if err != nil {
    return fmt.Errorf("load ssh ca: %w", err)
}
ephPub, ephPriv, _ := ed25519.GenerateKey(rand.Reader)
ephSigner, _ := ssh.NewSignerFromKey(ephPriv)
cert, err := h.certSigner.Sign(ctx, sshcert.CertRequest{
    UserID:     authedUserID,
    ServerID:   server.ID,
    Principals: []string{conn.User},
    Validity:   15 * time.Minute,
    PubKey:     ephSigner.PublicKey(),
    Purpose:    "terminal",
})
if err != nil {
    return fmt.Errorf("mint cert: %w", err)
}
conn.Certificate = cert
conn.CertSigner = ephSigner
// Keep PrivateKey populated as Phase-1 fallback — when both are
// present, cert auth wins in NewSSHClient's authMethods order.
```

**Step 2: Verify in DevTools**

Open the terminal on a server that's had the CA backfill run. Connect — expect the SSH negotiation logs to show `Authentication succeeded (publickey-cert)` instead of `(publickey)`. On the target server, `tail -1 /var/log/auth.log` should show:
```
sshd[12345]: Accepted publickey for launcher … ID launch-user:01abc… (serial 1234567890)
```

The `launch-user:01abc…` is the Launch user ID embedded in the cert. **This is the per-user audit trail the security review called out.**

**Step 3: Add WebSocket close-frame on cert expiry**

If the session is still open when the cert expires (>15 min interactive shell), sshd's behaviour is to keep the session alive — the cert is only checked at auth time, not per-frame. We don't need to do anything special for v1.

**Step 4: Commit**

```bash
git add internal/modules/websocket/handlers/terminal.go
git commit -m "feat(terminal): use short-lived SSH cert per session"
```

---

## Slice H — Deploy / backup / command jobs use cert auth

**Files:**
- Modify: `internal/modules/site/jobs/deploy_site.go` (and related deploy job entry points)
- Modify: `internal/modules/backup/jobs/run_manual_backup.go` (and scheduled backup)
- Modify: `internal/modules/server/jobs/*.go` (server-task dispatcher)

Pattern is the same as Slice G — mint a cert just before `sshClient.Connect()` — but with `Validity = 1 * time.Hour` (longer-lived because deploys can take a while) and `Purpose = "deploy" | "backup" | "command"`.

**Step 1: Refactor the SSH client builder helper**

To avoid copy-pasting the cert-mint preamble in 12 places, put it behind a method:

```go
func (s *ServiceDeps) DialWithCert(ctx context.Context, server *models.Server, purpose string, validity time.Duration) (*taskrunner.SSHClient, error) {
    conn := server.ConnectionAsRoot()
    // mint cert, populate conn.Certificate + CertSigner, dial
    ...
}
```

Then every job calls `s.DialWithCert(ctx, server, "deploy", 1*time.Hour)` instead of building Connections by hand.

**Step 2: Migrate one job at a time, behind an env flag**

`LAUNCH_SSH_CERT_AUTH=true` flips the cert path on. Default off in Phase H. Once each job is verified on the live stack, set the flag in prod and remove the flag in a follow-up PR.

**Step 3: Live verify**

- Deploy a site → verify deploy completes
- Run a manual backup → verify backup uploads to S3
- Verify per-job audit rows in `ssh_cert_audits` exist with the right `purpose` field

**Step 4: Commit per job**

One PR per job type for easier rollback.

---

## Slice I — Remove per-user long-lived key

**Files:**
- Modify: `internal/modules/server/models/server.go` (ConnectionAsUser stops setting PrivateKey for non-root users)
- Migration to scrub `authorized_keys` on the target server (one-off via job)

**Step 1: Identify the break-glass path**

Keep `Server.RootUsername()` + `ConnectionAsRoot` using the long-lived key. That's the emergency escape hatch — if cert auth breaks for any reason, an admin can still reach the box. The long-lived key lives only in the `root` user's `authorized_keys`.

**Step 2: Strip the long-lived key from non-root users**

A migration job that connects (using the existing root key) and:
```bash
sudo -u launcher bash -c 'sed -i "/launch-user-key/d" ~/.ssh/authorized_keys'
```
removes the per-user long-lived public key from `launcher`. From now on `launcher` can only be reached via cert auth.

**Step 3: Make ConnectionAsUser cert-only**

```go
func (s *Server) ConnectionAsUser(username ...string) *taskrunner.Connection {
    // PrivateKey is intentionally NOT populated for non-root users —
    // cert auth is the only way in. Root path retains it as
    // break-glass.
    return &taskrunner.Connection{
        ServerID: s.ID,
        HostKey:  s.HostKey,
        // Certificate + CertSigner are populated by the caller right
        // before dial.
    }
}
```

**Step 4: Verify**

After this lands, removing a Launch user immediately stops any cert mint for them. Their old certs expire within 15min for interactive, 1h for jobs. No `authorized_keys` edits needed.

---

## Slice J — UI: CA settings page + audit log viewer

**Files (launch-nuxt):**
- Create: `pages/admin/ssh-ca/index.vue`
- Create: `components/admin/SSHCAStatus.vue`
- Create: `components/admin/SSHCertAuditTable.vue`

**Slice J's surface:**

1. **CA status panel** — shows the CA fingerprint (full pubkey on click), creation date, "Rotate CA" button (regenerates the keypair and triggers a fleet-wide backfill).
2. **Audit log table** — paginated view of `ssh_cert_audits` with filters: user, server, date range, purpose. Each row links to the user + server detail pages.

This is mostly straightforward UI — TabStrip patterns from PR #23 apply.

---

## Slice K — KRL (Key Revocation List)

**Files:**
- Create: `internal/modules/ssh_ca/services/revocation.go`
- New endpoint: `POST /admin/ssh-ca/revoke`

Short-lived certs make explicit revocation unnecessary for the common case — a cert that's about to expire in 5 minutes doesn't need a KRL entry. But for "kill access NOW" (employee fired, key suspected compromised in the last 5 minutes), we want a hook.

**Approach:**
1. Add `ssh_cert_revocations` table — stores revoked cert serials.
2. Periodic job builds a KRL file (`ssh-keygen -k -f /etc/ssh/launch-krl …`) and pushes it to every server's sshd.
3. sshd_config gets `RevokedKeys /etc/ssh/launch-krl`.
4. Admin endpoint to add a serial to the revocation list.

**Defer to v2** — only implement if there's a real "we got hit, kill it now" requirement. For early SaaS, short-lived certs are revocation enough.

---

## Phase boundaries (for shipping)

| Phase | Slices | Outcome |
|---|---|---|
| Phase 1 (foundation) | A, B, C | CA exists, signer works, SSH client knows how to use certs. No user-visible change. |
| Phase 2 (provision) | D, E | New servers trust the CA at provision time. Existing servers get backfilled. |
| Phase 3 (audit) | F | Every cert mint logged. |
| Phase 4 (terminal flips) | G | Web terminal uses cert auth. **First user-visible win.** |
| Phase 5 (jobs flip) | H | Deploy + backup + command jobs use cert auth. |
| Phase 6 (key retire) | I | Long-lived per-user key removed; root break-glass only. |
| Phase 7 (UI + revocation) | J, K | Operator UX + emergency kill switch. |

Ship Phase 1–4 as MVP — that's the bulk of the security win. Phases 5–7 follow when there's runway.

---

## Out-of-scope follow-ups (don't put in this plan)

These should each be their own design doc later:

- **CA private key → KMS/Vault.** Right now the CA private key sits in `ssh_certificate_authorities.private_key` encrypted with `APP_KEY`. That's a known issue — same blast-radius problem as the original server private keys. Moving to KMS removes `APP_KEY` from the trust path. Separate plan.
- **Per-team CAs.** Single fleet CA is simpler operationally. A per-team CA isolates blast radius but adds complexity (CA rotation per team, backfill per team). Revisit if multi-tenant isolation requirements grow.
- **OpenID Connect SSO → cert mint.** Right now Launch's own user system maps to cert KeyId. If/when SSO comes in, the cert mint should pull the SSO user identity directly so it survives a Launch user record deletion.

---

## Execution notes for whoever picks this up

- **Don't skip the env-flag in Slice H.** Real deploys + backups break in subtle ways under new auth; the rollback flag has saved every SaaS PaaS who's done this migration.
- **Test the cert-expiry edge case in Slice G manually.** Open a terminal, leave it idle for >15min, try to type. Existing sessions should stay alive (sshd only checks at auth). New tabs should require fresh mint.
- **Backfill (Slice E) before flipping anything in Slice G.** If you flip the terminal to cert-only before every server trusts the CA, you'll lock out exactly the servers that need the migration most.
- Each slice's PR title prefix: `feat(ssh-ca):`.

