package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CertificateFile is one cert+key pair to be materialised on disk
// before the Caddyfile referencing it is reloaded. Used by
// WriteSiteCertificatesTask.
type CertificateFile struct {
	// CertificateID is the Certificate row's ULID — used as the
	// directory name under <site_path>/certificates/.
	CertificateID string
	// SitePath is the absolute path the site lives at on the server
	// (e.g. /home/<user>/<site>). The cert files land at
	// <SitePath>/certificates/<CertificateID>/{certificate.cert,private.key}.
	SitePath string
	// SiteUser is the unix account that owns the site directory.
	// We chown the cert files to this user so Caddy (running as
	// the site user via its launch-managed service) can read them.
	SiteUser string
	// CertPEM is the leaf + chain certificate, plaintext.
	CertPEM string
	// KeyPEM is the private key, plaintext. The caller is
	// responsible for decryption (dbtype.EncryptedString does that
	// on Scan).
	KeyPEM string
}

// WriteSiteCertificatesTask uploads (certificate.cert, private.key)
// pairs to <site_path>/certificates/<cert_id>/ for each entry in
// files. Used by InstallCaddyfileJob and the update path so the
// custom-TLS branch of generateTLSSnippet can reference the file
// paths produced here.
//
// Files are written 0600 with chown to <SiteUser> so Caddy can read
// them while running unprivileged. The parent dir is 0700.
//
// Idempotent: re-writing the same content is a no-op for Caddy
// (mtime-based reloads trigger a hot-reload that just re-reads the
// same bytes). Returns a no-op task when files is empty so callers
// can wire this unconditionally before the Caddyfile write.
//
// Heredoc sentinels are randomised per file so a malicious cert
// containing the sentinel string can't terminate the heredoc early.
func WriteSiteCertificatesTask(files []CertificateFile) taskrunner.Task {
	if len(files) == 0 {
		return taskrunner.NewBaseTask(
			taskrunner.WithName("Write Site Certificates (no-op)"),
			taskrunner.WithScript("#!/usr/bin/env bash\necho '::LAUNCH::site_certs::skipped'\n"),
			taskrunner.WithTimeoutSeconds(5),
		)
	}

	var script strings.Builder
	script.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")

	for _, f := range files {
		dir := fmt.Sprintf("%s/certificates/%s", f.SitePath, f.CertificateID)
		certPath := dir + "/certificate.cert"
		keyPath := dir + "/private.key"
		owner := f.SiteUser
		if owner == "" {
			owner = "root"
		}

		certSentinel := randomCertHeredocSentinel()
		keySentinel := randomCertHeredocSentinel()

		fmt.Fprintf(&script, "sudo install -d -o %s -g %s -m 0700 %q\n", owner, owner, dir)
		fmt.Fprintf(&script, "sudo tee %q >/dev/null <<'%s'\n%s\n%s\n", certPath, certSentinel, f.CertPEM, certSentinel)
		fmt.Fprintf(&script, "sudo chown %s:%s %q\n", owner, owner, certPath)
		fmt.Fprintf(&script, "sudo chmod 0600 %q\n", certPath)
		fmt.Fprintf(&script, "sudo tee %q >/dev/null <<'%s'\n%s\n%s\n", keyPath, keySentinel, f.KeyPEM, keySentinel)
		fmt.Fprintf(&script, "sudo chown %s:%s %q\n", owner, owner, keyPath)
		fmt.Fprintf(&script, "sudo chmod 0600 %q\n", keyPath)
	}
	script.WriteString("echo '::LAUNCH::site_certs::written'\n")

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Write Site Certificates"),
		taskrunner.WithScript(script.String()),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// randomCertHeredocSentinel produces an unguessable EOF marker that
// can't appear in a pasted PEM (PEM is base64 + dashes; the
// "LAUNCH_CERT_" prefix plus 16 hex chars is outside that alphabet).
func randomCertHeredocSentinel() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "LAUNCH_CERT_" + hex.EncodeToString(b[:])
}
