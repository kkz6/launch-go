package formatter

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// GenerateLBCaddyfile generates a per-upstream Caddyfile for a load balancer.
// Each upstream gets its own file at /etc/caddy/upstreams/{upstream_id}.caddy
func GenerateLBCaddyfile(upstream *models.LoadBalancerUpstream, backends []models.LoadBalancerBackend) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Upstream: %s\n", upstream.Address))
	sb.WriteString(fmt.Sprintf("# Policy: %s\n", upstream.LBPolicy))
	sb.WriteString(fmt.Sprintf("# Backends: %d\n\n", len(backends)))

	// Domain block
	if upstream.Port != 443 {
		sb.WriteString(fmt.Sprintf("%s:%d {\n", upstream.Address, upstream.Port))
	} else {
		sb.WriteString(fmt.Sprintf("%s {\n", upstream.Address))
	}

	// TLS configuration
	switch upstream.TLSSetting {
	case "internal":
		sb.WriteString("    tls internal\n")
	case "off":
		sb.WriteString("    # TLS disabled\n")
	default:
		// "auto" — Caddy auto-manages
	}

	// Security headers
	sb.WriteString("    header {\n")
	sb.WriteString("        -Server\n")
	sb.WriteString("        X-Content-Type-Options nosniff\n")
	sb.WriteString("        X-Frame-Options SAMEORIGIN\n")
	sb.WriteString("        X-Powered-By \"Launch\"\n")
	sb.WriteString("    }\n\n")

	// Reverse proxy block
	sb.WriteString("    reverse_proxy {\n")

	// Backend addresses (plain HTTP on dedicated port)
	var addrs []string
	for _, backend := range backends {
		if backend.IsDown {
			continue
		}

		port := backend.Port
		if port == 0 {
			port = 8080
		}

		ip := ""
		if backend.Server != nil && backend.Server.PublicIPv4 != nil {
			ip = *backend.Server.PublicIPv4
		}
		if ip == "" {
			continue
		}

		addrs = append(addrs, fmt.Sprintf("%s:%d", ip, port))
	}

	if len(addrs) > 0 {
		sb.WriteString(fmt.Sprintf("        to %s\n\n", strings.Join(addrs, " ")))
	} else {
		sb.WriteString("        # No active backends\n\n")
	}

	// Load balancing policy
	sb.WriteString(fmt.Sprintf("        lb_policy %s\n\n", upstream.LBPolicy))

	// Health checks
	sb.WriteString(fmt.Sprintf("        health_uri %s\n", upstream.HealthCheckPath))
	sb.WriteString(fmt.Sprintf("        health_interval %s\n", upstream.HealthCheckInterval))
	sb.WriteString(fmt.Sprintf("        health_timeout %s\n", upstream.HealthCheckTimeout))
	sb.WriteString("        health_status 200\n\n")

	// Forwarding headers
	sb.WriteString("        header_up Host {upstream_hostport}\n")
	sb.WriteString("        header_up X-Real-IP {remote_host}\n")
	sb.WriteString("        header_up X-Forwarded-For {remote_host}\n")
	sb.WriteString("        header_up X-Forwarded-Proto {scheme}\n")

	sb.WriteString("    }\n\n")

	// Logging
	logPath := fmt.Sprintf("/var/log/caddy/%s.log", strings.ReplaceAll(upstream.Address, ".", "_"))
	sb.WriteString("    log {\n")
	sb.WriteString(fmt.Sprintf("        output file %s {\n", logPath))
	sb.WriteString("            roll_size 100mb\n")
	sb.WriteString("            roll_keep 30\n")
	sb.WriteString("            roll_keep_for 720h\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")

	sb.WriteString("}\n")

	return sb.String()
}

// UpstreamCaddyfilePath returns the path for an upstream's Caddyfile
func UpstreamCaddyfilePath(upstreamID string) string {
	return fmt.Sprintf("/etc/caddy/upstreams/%s.caddy", upstreamID)
}

// GenerateUpstreamsImports generates the content for /etc/caddy/Upstreams.caddy
// which imports all individual upstream Caddyfiles
func GenerateUpstreamsImports(upstreamIDs []string) string {
	var sb strings.Builder

	sb.WriteString("# Load balancer upstream configurations\n")
	sb.WriteString("# Managed by Launch - do not edit manually\n\n")

	for _, id := range upstreamIDs {
		sb.WriteString(fmt.Sprintf("import /etc/caddy/upstreams/%s.caddy\n", id))
	}

	return sb.String()
}
