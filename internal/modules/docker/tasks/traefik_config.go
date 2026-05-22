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

	var b strings.Builder
	b.WriteString("# Managed by Launch. Do not edit by hand.\n")
	fmt.Fprintf(&b, "# app: %s\n", id)
	b.WriteString("http:\n")
	b.WriteString("  routers:\n")
	hasHTTPS := false
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
			hasHTTPS = true
		}

		if d.HTTPS {
			fmt.Fprintf(&b, "    %s-https:\n", safeName)
			fmt.Fprintf(&b, "      rule: %q\n", hostRule)
			b.WriteString("      entryPoints: [websecure]\n")
			fmt.Fprintf(&b, "      service: %s\n", id)
			b.WriteString("      tls:\n")
			b.WriteString("        certresolver: letsencrypt\n")
		}
	}
	_ = hasHTTPS // placeholder for future TLS-conditional config

	b.WriteString("  services:\n")
	fmt.Fprintf(&b, "    %s:\n", id)
	b.WriteString("      loadBalancer:\n")
	b.WriteString("        servers:\n")
	// Traefik reaches the container by its docker DNS name on the shared
	// launch-network. Port is the app's *internal* port; Traefik handles
	// the public 80/443 itself.
	fmt.Fprintf(&b, "          - url: \"http://%s:%d\"\n", args.ContainerName, args.InternalPort)

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
// The heredoc delimiter is quoted so YAML content with $ signs is not
// expanded by the shell.
func WriteTraefikConfigTask(projectSlug, appSlug, contents string) taskrunner.Task {
	path := TraefikConfigPath(projectSlug, appSlug)
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
sudo mkdir -p /etc/launch/traefik/dynamic
sudo tee %q >/dev/null <<'LAUNCH_TRAEFIK_EOF'
%s
LAUNCH_TRAEFIK_EOF
echo "::LAUNCH::traefik_config::written"
`, path, contents)
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
