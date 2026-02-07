package formatter

import (
	"strings"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func strPtr(s string) *string { return &s }

func TestGenerateLBCaddyfile_BasicRoundRobin(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Name:                "test-upstream",
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.2")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	// Header comments
	if !strings.Contains(result, "# Upstream: app.example.com") {
		t.Error("expected upstream address in header comment")
	}
	if !strings.Contains(result, "# Policy: round_robin") {
		t.Error("expected policy in header comment")
	}
	if !strings.Contains(result, "# Backends: 2") {
		t.Error("expected backend count in header comment")
	}

	// Domain block without port for 443
	if !strings.Contains(result, "app.example.com {") {
		t.Error("expected domain block without port for 443")
	}
	if strings.Contains(result, "app.example.com:443") {
		t.Error("should not include :443 in domain block")
	}

	// No TLS directive for auto
	if strings.Contains(result, "tls internal") {
		t.Error("should not have tls internal for auto setting")
	}
	if strings.Contains(result, "# TLS disabled") {
		t.Error("should not have TLS disabled for auto setting")
	}

	// Backend addresses
	if !strings.Contains(result, "10.0.0.1:8080") {
		t.Error("expected first backend IP and port")
	}
	if !strings.Contains(result, "10.0.0.2:8080") {
		t.Error("expected second backend IP and port")
	}

	// LB policy
	if !strings.Contains(result, "lb_policy round_robin") {
		t.Error("expected round_robin lb_policy")
	}

	// Health check settings
	if !strings.Contains(result, "health_uri /health") {
		t.Error("expected health_uri")
	}
	if !strings.Contains(result, "health_interval 30s") {
		t.Error("expected health_interval")
	}
	if !strings.Contains(result, "health_timeout 10s") {
		t.Error("expected health_timeout")
	}

	// Forwarding headers
	if !strings.Contains(result, "header_up X-Real-IP") {
		t.Error("expected X-Real-IP forwarding header")
	}
	if !strings.Contains(result, "header_up X-Forwarded-For") {
		t.Error("expected X-Forwarded-For forwarding header")
	}

	// Log block with dots replaced by underscores
	if !strings.Contains(result, "/var/log/caddy/app_example_com.log") {
		t.Error("expected log path with dots replaced by underscores")
	}

	// Security headers
	if !strings.Contains(result, "X-Content-Type-Options nosniff") {
		t.Error("expected X-Content-Type-Options header")
	}
	if !strings.Contains(result, "X-Frame-Options SAMEORIGIN") {
		t.Error("expected X-Frame-Options header")
	}
}

func TestGenerateLBCaddyfile_CustomPort(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                8443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "app.example.com:8443 {") {
		t.Error("expected address:8443 in domain block for non-443 port")
	}
}

func TestGenerateLBCaddyfile_TLSInternal(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "internal.example.com",
		Port:                443,
		TLSSetting:          "internal",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "tls internal") {
		t.Error("expected 'tls internal' directive for internal TLS setting")
	}
}

func TestGenerateLBCaddyfile_TLSOff(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "plain.example.com",
		Port:                443,
		TLSSetting:          "off",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "# TLS disabled") {
		t.Error("expected '# TLS disabled' comment for off TLS setting")
	}
	if strings.Contains(result, "tls internal") {
		t.Error("should not have 'tls internal' when TLS is off")
	}
}

func TestGenerateLBCaddyfile_SkipsDownBackends(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
		{
			Port:   8080,
			IsDown: true,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.2")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "10.0.0.1:8080") {
		t.Error("expected active backend IP to be present")
	}
	if strings.Contains(result, "10.0.0.2") {
		t.Error("down backend IP should not appear in output")
	}
}

func TestGenerateLBCaddyfile_SkipsBackendsWithoutIP(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
		{
			Port:   8080,
			IsDown: false,
			Server: nil, // no server loaded
		},
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: nil}, // server without IP
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "10.0.0.1:8080") {
		t.Error("expected backend with valid IP to be present")
	}

	// Should only have a single "to" line with one address
	if !strings.Contains(result, "to 10.0.0.1:8080") {
		t.Error("expected only the single valid backend in the 'to' directive")
	}
}

func TestGenerateLBCaddyfile_NoActiveBackends(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: true,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
		{
			Port:   8080,
			IsDown: true,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.2")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	// Should respond 503 instead of generating an invalid reverse_proxy block
	if !strings.Contains(result, `respond "Service Unavailable" 503`) {
		t.Error("expected 'respond 503' when all backends are down")
	}
	if strings.Contains(result, "reverse_proxy") {
		t.Error("should not have reverse_proxy block when no backends are active")
	}
	if strings.Contains(result, "to 10.0.0.1") || strings.Contains(result, "to 10.0.0.2") {
		t.Error("should not have backend addresses when no backends are active")
	}
}

func TestGenerateLBCaddyfile_DefaultPort8080(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyRoundRobin,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   0, // should default to 8080
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "10.0.0.1:8080") {
		t.Error("expected port 0 to default to 8080")
	}
}

func TestGenerateLBCaddyfile_LeastConnPolicy(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyLeastConn,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "lb_policy least_conn") {
		t.Error("expected lb_policy least_conn")
	}
}

func TestGenerateLBCaddyfile_IPHashPolicy(t *testing.T) {
	upstream := &models.LoadBalancerUpstream{
		Address:             "app.example.com",
		Port:                443,
		TLSSetting:          "auto",
		LBPolicy:            types.LBPolicyIPHash,
		HealthCheckPath:     "/health",
		HealthCheckInterval: "30s",
		HealthCheckTimeout:  "10s",
	}

	backends := []models.LoadBalancerBackend{
		{
			Port:   8080,
			IsDown: false,
			Server: &models.Server{PublicIPv4: strPtr("10.0.0.1")},
		},
	}

	result := GenerateLBCaddyfile(upstream, backends)

	if !strings.Contains(result, "lb_policy ip_hash") {
		t.Error("expected lb_policy ip_hash")
	}
}

func TestUpstreamCaddyfilePath(t *testing.T) {
	result := UpstreamCaddyfilePath("abc123")

	expected := "/etc/caddy/upstreams/abc123.caddy"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGenerateUpstreamsImports_MultipleUpstreams(t *testing.T) {
	ids := []string{"upstream-1", "upstream-2", "upstream-3"}

	result := GenerateUpstreamsImports(ids)

	if !strings.Contains(result, "# Load balancer upstream configurations") {
		t.Error("expected header comment")
	}
	if !strings.Contains(result, "# Managed by Launch - do not edit manually") {
		t.Error("expected managed-by comment")
	}
	if !strings.Contains(result, "import /etc/caddy/upstreams/upstream-1.caddy") {
		t.Error("expected import for upstream-1")
	}
	if !strings.Contains(result, "import /etc/caddy/upstreams/upstream-2.caddy") {
		t.Error("expected import for upstream-2")
	}
	if !strings.Contains(result, "import /etc/caddy/upstreams/upstream-3.caddy") {
		t.Error("expected import for upstream-3")
	}
}

func TestGenerateUpstreamsImports_Empty(t *testing.T) {
	result := GenerateUpstreamsImports([]string{})

	if !strings.Contains(result, "# Load balancer upstream configurations") {
		t.Error("expected header comment even with empty slice")
	}
	if !strings.Contains(result, "# Managed by Launch - do not edit manually") {
		t.Error("expected managed-by comment even with empty slice")
	}
	if strings.Contains(result, "import ") {
		t.Error("should not have any import lines for empty slice")
	}
}
