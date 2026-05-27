package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TraefikConfigArgs carries everything the YAML template needs. Kept as
// a pure-data struct so RenderTraefikConfig stays a deterministic
// function — easier to unit test, no SSH side-effects.
type TraefikConfigArgs struct {
	// ProjectSlug + AppSlug → composed into the container name and the
	// router/service identifiers inside the YAML.
	ProjectSlug   string
	AppSlug       string
	ContainerName string
	InternalPort  int
	Domains       []models.ApplicationDomain
}

// RenderTraefikConfig produces the YAML Traefik watches for routing
// rules. One file per app at /etc/launch/traefik/dynamic/<project>-<app>.yml.
//
// Layout:
//   - Each domain gets its own HTTP router. If domain.HTTPS is true, the
//     router is duplicated onto websecure with tls.certresolver=letsencrypt
//     and the http router rewrites to https via the redirect middleware.
//   - All routers point to a single service that resolves the container
//     by its docker network name (Traefik runs in the same launch-network).
//
// Returning a string (not []byte) so the SSH heredoc upload is trivial;
// the YAML is ASCII and short enough that the extra copy doesn't matter.
func RenderTraefikConfig(args TraefikConfigArgs) string {
	id := fmt.Sprintf("%s-%s", args.ProjectSlug, args.AppSlug)
	if len(args.Domains) == 0 {
		// No domains means no routers. Emit a minimal placeholder so the
		// file still exists (lets us track existence separately from a
		// deleted file representing a deleted app).
		return fmt.Sprintf("# %s — no domains configured\nhttp: {}\n", id)
	}

	// Collect unique stored-cert ids so the top-level tls.certificates
	// block lists each pair once even if multiple domains share a cert.
	// Order is the insertion order of first-occurrence (matches the
	// domain row order — keeps the rendered YAML stable for tests).
	storedCertIDs := make([]string, 0)
	seenStoredCertIDs := make(map[string]struct{})

	var b strings.Builder
	b.WriteString("# Managed by Launch. Do not edit by hand.\n")
	fmt.Fprintf(&b, "# app: %s\n", id)
	b.WriteString("http:\n")
	b.WriteString("  routers:\n")
	for i, d := range args.Domains {
		host := d.Host
		safeName := fmt.Sprintf("%s-%d", id, i)
		hostRule := fmt.Sprintf("Host(`%s`)", host)
		if d.Path != nil && *d.Path != "" {
			hostRule += fmt.Sprintf(" && PathPrefix(`%s`)", *d.Path)
		}

		// Plain HTTP router. When HTTPS is on we redirect via middleware
		// rather than just leaving it out — Let's Encrypt's HTTP-01
		// challenge still needs to reach Traefik on port 80, so we can't
		// disable web entirely.
		fmt.Fprintf(&b, "    %s-http:\n", safeName)
		fmt.Fprintf(&b, "      rule: %q\n", hostRule)
		b.WriteString("      entryPoints: [web]\n")
		fmt.Fprintf(&b, "      service: %s\n", id)
		if d.HTTPS {
			b.WriteString("      middlewares: [redirect-to-https]\n")

			fmt.Fprintf(&b, "    %s-https:\n", safeName)
			fmt.Fprintf(&b, "      rule: %q\n", hostRule)
			b.WriteString("      entryPoints: [websecure]\n")
			fmt.Fprintf(&b, "      service: %s\n", id)
			b.WriteString("      tls:\n")
			// Stored cert: empty tls block + cert listed at top-level
			// tls.certificates. Traefik picks via SNI. letsencrypt is
			// the default fallback.
			if d.CertificateProvider == "stored" && d.StoredCertificateID != nil && *d.StoredCertificateID != "" {
				cid := *d.StoredCertificateID
				if _, ok := seenStoredCertIDs[cid]; !ok {
					seenStoredCertIDs[cid] = struct{}{}
					storedCertIDs = append(storedCertIDs, cid)
				}
				b.WriteString("        # cert sourced from stored library; see tls.certificates below\n")
			} else {
				b.WriteString("        certresolver: letsencrypt\n")
			}
		}
	}

	b.WriteString("  services:\n")
	fmt.Fprintf(&b, "    %s:\n", id)
	b.WriteString("      loadBalancer:\n")
	b.WriteString("        servers:\n")
	// Traefik reaches the container by its docker DNS name on the shared
	// launch-network. Port is the app's *internal* port; Traefik handles
	// the public 80/443 itself.
	fmt.Fprintf(&b, "          - url: \"http://%s:%d\"\n", args.ContainerName, args.InternalPort)

	// Top-level tls.certificates block: lists every stored cert
	// referenced above. Cert files are materialised on disk by the
	// deploy task at /var/lib/launch/traefik/certs/<id>/{cert.pem,key.pem}
	// (worker writes them before reloading Traefik — see Phase 6).
	if len(storedCertIDs) > 0 {
		b.WriteString("tls:\n")
		b.WriteString("  certificates:\n")
		for _, cid := range storedCertIDs {
			fmt.Fprintf(&b, "    - certFile: /var/lib/launch/traefik/certs/%s/cert.pem\n", cid)
			fmt.Fprintf(&b, "      keyFile: /var/lib/launch/traefik/certs/%s/key.pem\n", cid)
		}
	}

	return b.String()
}

// TraefikConfigPath returns the canonical on-server path for this app's
// dynamic config file. Centralised so writer and delete share one source
// of truth.
func TraefikConfigPath(projectSlug, appSlug string) string {
	return fmt.Sprintf("/etc/launch/traefik/dynamic/%s-%s.yml", projectSlug, appSlug)
}

// WriteTraefikConfigTask uploads `contents` to TraefikConfigPath(...) via
// SSH. Traefik watches the directory so the new file is picked up
// automatically; no reload needed.
//
// The heredoc delimiter is single-quoted (so YAML `$` references are
// preserved verbatim) AND randomised per call (so a body containing
// the sentinel line can't terminate the heredoc early).
func WriteTraefikConfigTask(projectSlug, appSlug, contents string) taskrunner.Task {
	path := TraefikConfigPath(projectSlug, appSlug)
	sentinel := RandomHeredocSentinel()
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
sudo mkdir -p /etc/launch/traefik/dynamic
sudo tee %q >/dev/null <<'%s'
%s
%s
echo "::LAUNCH::traefik_config::written"
`, path, sentinel, contents, sentinel)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Write Traefik Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeleteTraefikConfigTask removes the file. Used when an application is
// deleted; for "all domains removed" we still keep the file with an
// empty config so the path keeps existing (helps debugging "where is my
// app's config?").
func DeleteTraefikConfigTask(projectSlug, appSlug string) taskrunner.Task {
	path := TraefikConfigPath(projectSlug, appSlug)
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
sudo rm -f %q
echo "::LAUNCH::traefik_config::deleted"
`, path)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Traefik Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// StoredCertMaterial is a single (cert.pem, key.pem) pair the worker
// writes to /var/lib/launch/traefik/certs/<id>/ before reloading
// Traefik's dynamic config. Traefik picks it up via SNI thanks to the
// top-level tls.certificates block emitted by RenderTraefikConfig.
type StoredCertMaterial struct {
	CertificateID string
	CertPEM       string // leaf + chain, plaintext
	KeyPEM        string // private key, plaintext (decrypted on Scan)
}

// WriteStoredCertificatesTask uploads each cert+key pair to the docker
// server under /var/lib/launch/traefik/certs/<id>/. Files are written
// 0600 (root-owned) so they're not world-readable; the parent dir is
// 0700.
//
// The task is idempotent — re-writing the same content is a no-op as
// far as Traefik is concerned (it watches the file mtime; identical
// content with a newer mtime triggers a hot-reload which is harmless).
// We don't fingerprint-skip server-side; the caller dedupes by
// certificate id in the Render step, so writes are already bounded.
//
// Returns a no-op task when len(materials) == 0 so callers can wire
// this unconditionally before the YAML write.
func WriteStoredCertificatesTask(materials []StoredCertMaterial) taskrunner.Task {
	if len(materials) == 0 {
		return taskrunner.NewBaseTask(
			taskrunner.WithName("Write Stored Certificates (no-op)"),
			taskrunner.WithScript("#!/usr/bin/env bash\necho '::LAUNCH::stored_certs::skipped'\n"),
			taskrunner.WithTimeoutSeconds(5),
		)
	}

	var script strings.Builder
	script.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	script.WriteString("sudo install -d -m 0700 /var/lib/launch/traefik/certs\n")
	for _, m := range materials {
		dir := fmt.Sprintf("/var/lib/launch/traefik/certs/%s", m.CertificateID)
		certPath := dir + "/cert.pem"
		keyPath := dir + "/key.pem"
		// Per-file random heredoc sentinel — the PEM content can contain
		// any printable ASCII, so a fixed sentinel could collide with a
		// pasted certificate. The sentinel includes random hex so a
		// hostile cert can't terminate the heredoc.
		certSentinel := RandomHeredocSentinel()
		keySentinel := RandomHeredocSentinel()

		fmt.Fprintf(&script, "sudo install -d -m 0700 %q\n", dir)
		fmt.Fprintf(&script, "sudo tee %q >/dev/null <<'%s'\n%s\n%s\n", certPath, certSentinel, m.CertPEM, certSentinel)
		fmt.Fprintf(&script, "sudo chmod 0600 %q\n", certPath)
		fmt.Fprintf(&script, "sudo tee %q >/dev/null <<'%s'\n%s\n%s\n", keyPath, keySentinel, m.KeyPEM, keySentinel)
		fmt.Fprintf(&script, "sudo chmod 0600 %q\n", keyPath)
	}
	script.WriteString("echo '::LAUNCH::stored_certs::written'\n")

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Write Stored Certificates"),
		taskrunner.WithScript(script.String()),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// --- Compose-side Traefik rendering --------------------------------
//
// Per-compose dynamic-config file lives at
// `/etc/launch/traefik/dynamic/compose-<project>-<compose>.yml`. The
// `compose-` prefix namespaces these away from application files,
// which use `<project>-<app>.yml` — otherwise an application and a
// compose stack with the same name inside the same project would
// collide on disk. (The DB allows the collision today; the filename
// is what we have to disambiguate.)
//
// The renderer differs from the application path in two ways:
//
//   - One file may carry routes to MULTIPLE services (a stack runs
//     N containers). We emit one `service:` block per (service, port)
//     tuple and route each domain's router to the matching service.
//   - The service URL targets the container compose creates by
//     default: `<project>-<compose>-<service>-1`. That assumes the
//     operator hasn't overridden `container_name:` in their YAML;
//     if they have, the resolved name won't match and the routes
//     won't work — the dialog UI calls this out.

// ComposeTraefikConfigArgs is the pure-data input to the compose
// renderer. Same shape rationale as TraefikConfigArgs — deterministic
// function, no I/O, easy to unit-test.
type ComposeTraefikConfigArgs struct {
	ProjectSlug string
	ComposeSlug string
	// ProjectName is the compose project name passed via `-p` flag.
	// We use it to compute container names: `<ProjectName>-<service>-1`.
	ProjectName string
	Domains     []models.ApplicationDomain
}

// RenderComposeTraefikConfig produces the YAML body for a compose
// stack's dynamic-config file. Empty domain set → minimal placeholder
// (so the file's existence still tracks the stack's lifetime).
//
// Routers are named `compose-<project>-<compose>-<index>`; services
// are named `compose-<project>-<compose>-<service>` so two domains
// targeting the same service share one service block.
func RenderComposeTraefikConfig(args ComposeTraefikConfigArgs) string {
	id := fmt.Sprintf("compose-%s-%s", args.ProjectSlug, args.ComposeSlug)
	if len(args.Domains) == 0 {
		return fmt.Sprintf("# %s — no domains configured\nhttp: {}\n", id)
	}

	// Group domains by (service, port) so we emit one service block
	// per unique destination. Routers reference the service by name.
	type svcKey struct {
		service string
		port    int
	}
	type svcInfo struct {
		key  svcKey
		name string
	}
	svcs := map[svcKey]svcInfo{}
	svcOrder := []svcKey{} // deterministic emit order
	for _, d := range args.Domains {
		if d.ServiceName == nil || d.ContainerPort == nil {
			// Defensive: service layer validation rejects these, but
			// skip rather than emit invalid YAML if a stale row slips
			// through.
			continue
		}
		key := svcKey{service: *d.ServiceName, port: *d.ContainerPort}
		if _, ok := svcs[key]; !ok {
			svcs[key] = svcInfo{
				key:  key,
				name: fmt.Sprintf("%s-%s", id, *d.ServiceName),
			}
			svcOrder = append(svcOrder, key)
		}
	}

	// Collect unique stored-cert ids (same approach as the application
	// writer above) so the top-level tls.certificates block lists each
	// cert once.
	storedCertIDs := make([]string, 0)
	seenStoredCertIDs := make(map[string]struct{})

	var b strings.Builder
	b.WriteString("# Managed by Launch. Do not edit by hand.\n")
	fmt.Fprintf(&b, "# compose: %s\n", id)
	b.WriteString("http:\n")
	b.WriteString("  routers:\n")
	for i, d := range args.Domains {
		if d.ServiceName == nil || d.ContainerPort == nil {
			continue
		}
		host := d.Host
		safeName := fmt.Sprintf("%s-%d", id, i)
		svcName := fmt.Sprintf("%s-%s", id, *d.ServiceName)
		hostRule := fmt.Sprintf("Host(`%s`)", host)
		if d.Path != nil && *d.Path != "" {
			hostRule += fmt.Sprintf(" && PathPrefix(`%s`)", *d.Path)
		}

		fmt.Fprintf(&b, "    %s-http:\n", safeName)
		fmt.Fprintf(&b, "      rule: %q\n", hostRule)
		b.WriteString("      entryPoints: [web]\n")
		fmt.Fprintf(&b, "      service: %s\n", svcName)
		if d.HTTPS {
			b.WriteString("      middlewares: [redirect-to-https]\n")
		}

		if d.HTTPS {
			fmt.Fprintf(&b, "    %s-https:\n", safeName)
			fmt.Fprintf(&b, "      rule: %q\n", hostRule)
			b.WriteString("      entryPoints: [websecure]\n")
			fmt.Fprintf(&b, "      service: %s\n", svcName)
			b.WriteString("      tls:\n")
			if d.CertificateProvider == "stored" && d.StoredCertificateID != nil && *d.StoredCertificateID != "" {
				cid := *d.StoredCertificateID
				if _, ok := seenStoredCertIDs[cid]; !ok {
					seenStoredCertIDs[cid] = struct{}{}
					storedCertIDs = append(storedCertIDs, cid)
				}
				b.WriteString("        # cert sourced from stored library; see tls.certificates below\n")
			} else {
				b.WriteString("        certresolver: letsencrypt\n")
			}
		}
	}

	b.WriteString("  services:\n")
	for _, key := range svcOrder {
		info := svcs[key]
		fmt.Fprintf(&b, "    %s:\n", info.name)
		b.WriteString("      loadBalancer:\n")
		b.WriteString("        servers:\n")
		// Compose v2's default container naming: `<project>-<service>-<idx>`.
		// `<idx>` starts at 1 and increments per replica. We target
		// `-1` because non-replicated services are the common case and
		// Traefik will load-balance across all matching container DNS
		// names if multiple exist on the network — but that's a future
		// problem; for now we point at the first replica.
		fmt.Fprintf(&b, "          - url: \"http://%s-%s-1:%d\"\n",
			args.ProjectName, key.service, key.port,
		)
	}

	if len(storedCertIDs) > 0 {
		b.WriteString("tls:\n")
		b.WriteString("  certificates:\n")
		for _, cid := range storedCertIDs {
			fmt.Fprintf(&b, "    - certFile: /var/lib/launch/traefik/certs/%s/cert.pem\n", cid)
			fmt.Fprintf(&b, "      keyFile: /var/lib/launch/traefik/certs/%s/key.pem\n", cid)
		}
	}

	return b.String()
}

// ComposeTraefikConfigPath returns the canonical on-server path for
// a compose stack's dynamic config file. Centralised so writer +
// delete + read share one source of truth.
func ComposeTraefikConfigPath(projectSlug, composeSlug string) string {
	return fmt.Sprintf(
		"/etc/launch/traefik/dynamic/compose-%s-%s.yml",
		projectSlug, composeSlug,
	)
}

// ComposeTraefikConfigFilename returns just the basename (no
// directory) — used by the per-compose Traefik card endpoint which
// feeds filenames into WriteTraefikDynamicFile.
func ComposeTraefikConfigFilename(projectSlug, composeSlug string) string {
	return fmt.Sprintf("compose-%s-%s.yml", projectSlug, composeSlug)
}

// WriteComposeTraefikConfigTask uploads `contents` to
// ComposeTraefikConfigPath(...) via SSH. Mirrors WriteTraefikConfigTask
// but writes to the compose-namespaced path. Traefik watches the dir
// so no reload is needed.
func WriteComposeTraefikConfigTask(projectSlug, composeSlug, contents string) taskrunner.Task {
	path := ComposeTraefikConfigPath(projectSlug, composeSlug)
	sentinel := RandomHeredocSentinel()
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
sudo mkdir -p /etc/launch/traefik/dynamic
sudo tee %q >/dev/null <<'%s'
%s
%s
echo "::LAUNCH::traefik_config::written"
`, path, sentinel, contents, sentinel)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Write Compose Traefik Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeleteComposeTraefikConfigTask removes the per-compose dynamic
// file. Used when the stack is deleted.
func DeleteComposeTraefikConfigTask(projectSlug, composeSlug string) taskrunner.Task {
	path := ComposeTraefikConfigPath(projectSlug, composeSlug)
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
sudo rm -f %q
echo "::LAUNCH::traefik_config::deleted"
`, path)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Compose Traefik Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
