package jobs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

func strPtr(s string) *string { return &s }

func newLaravelSite() *models.Site {
	phpVer := sitetypes.PhpVersion("php83")
	site := &models.Site{
		Address:                "example.com",
		Type:                   sitetypes.SiteTypeLaravel,
		TLSSetting:             sitetypes.TLSSettingAuto,
		User:                   "launcher",
		Path:                   "/home/launcher/example.com",
		WebFolder:              "public",
		PhpVersion:             &phpVer,
		ZeroDowntimeDeployment: true,
	}
	site.ID = "01HTEST000000000000000001"

	return site
}

func newStaticSite() *models.Site {
	site := &models.Site{
		Address:                "static.example.com",
		Type:                   sitetypes.SiteTypeStatic,
		TLSSetting:             sitetypes.TLSSettingAuto,
		User:                   "launcher",
		Path:                   "/home/launcher/static.example.com",
		WebFolder:              "",
		ZeroDowntimeDeployment: true,
	}
	site.ID = "01HTEST000000000000000002"

	return site
}

func newWordPressSite() *models.Site {
	phpVer := sitetypes.PhpVersion("php83")
	site := &models.Site{
		Address:                "wp.example.com",
		Type:                   sitetypes.SiteTypeWordpress,
		TLSSetting:             sitetypes.TLSSettingAuto,
		User:                   "launcher",
		Path:                   "/home/launcher/wp.example.com",
		WebFolder:              "/",
		PhpVersion:             &phpVer,
		ZeroDowntimeDeployment: false,
	}
	site.ID = "01HTEST000000000000000003"

	return site
}

func newRedirect(id, from, to string, mode int) models.Redirect {
	r := models.Redirect{
		From: from,
		To:   to,
		Mode: mode,
	}
	r.ID = id

	return r
}

// TestGenerateCaddyfile_LoadBalancedSite verifies that generateCaddyfile routes to
// the load balanced Caddyfile generator when the site has a LoadBalancedUpstreamID
// and a loadBalancerIP is provided.
func TestGenerateCaddyfile_LoadBalancedSite(t *testing.T) {
	site := newLaravelSite()
	site.LoadBalancedUpstreamID = strPtr("01HLBUPSTREAM00000000001")
	loadBalancerIP := "10.0.0.50"

	result := generateCaddyfile(site, nil, loadBalancerIP)

	if !strings.Contains(result, "http://example.com:8080") {
		t.Errorf("expected load balanced Caddyfile with http:// and :8080, got:\n%s", result)
	}

	if !strings.Contains(result, "@notlb not remote_ip 10.0.0.50") {
		t.Errorf("expected IP restriction for load balancer, got:\n%s", result)
	}

	// Should NOT contain TLS snippet (load balanced sites use HTTP only)
	if strings.Contains(result, "import tls-") {
		t.Errorf("load balanced Caddyfile should not contain TLS import, got:\n%s", result)
	}
}

// TestGenerateCaddyfile_StandardSite verifies that generateCaddyfile routes to
// the standard Caddyfile generator when the site has no LoadBalancedUpstreamID.
func TestGenerateCaddyfile_StandardSite(t *testing.T) {
	site := newLaravelSite()

	result := generateCaddyfile(site, nil, "")

	// Standard site should have the address with port (443 for TLS auto)
	if !strings.Contains(result, "example.com:443") {
		t.Errorf("expected standard Caddyfile with address:443, got:\n%s", result)
	}

	// Standard site should have TLS snippet
	if !strings.Contains(result, "import tls-01HTEST000000000000000001") {
		t.Errorf("expected TLS import in standard Caddyfile, got:\n%s", result)
	}

	// Should NOT have the load balancer HTTP-only prefix
	if strings.Contains(result, "http://example.com:8080") {
		t.Errorf("standard Caddyfile should not have http://:8080, got:\n%s", result)
	}
}

// TestGenerateLoadBalancedCaddyfile_BasicPHP verifies the load balanced Caddyfile
// for a PHP/Laravel site includes: HTTP prefix, port 8080, IP restriction,
// correct root directory, php_fastcgi block, and no TLS.
func TestGenerateLoadBalancedCaddyfile_BasicPHP(t *testing.T) {
	site := newLaravelSite()
	loadBalancerIP := "192.168.1.100"

	result := generateLoadBalancedCaddyfile(site, nil, loadBalancerIP)

	// HTTP-only on port 8080
	if !strings.Contains(result, "http://example.com:8080 {") {
		t.Errorf("expected http://example.com:8080, got:\n%s", result)
	}

	// IP restriction
	if !strings.Contains(result, "@notlb not remote_ip 192.168.1.100") {
		t.Errorf("expected IP restriction matcher, got:\n%s", result)
	}

	if !strings.Contains(result, `respond @notlb "Forbidden" 403`) {
		t.Errorf("expected 403 response for non-LB IPs, got:\n%s", result)
	}

	// Root directory
	expectedRoot := "root * /home/launcher/example.com/current/public"
	if !strings.Contains(result, expectedRoot) {
		t.Errorf("expected root directive %q, got:\n%s", expectedRoot, result)
	}

	// Encoding
	if !strings.Contains(result, "encode zstd gzip") {
		t.Errorf("expected encode directive, got:\n%s", result)
	}

	// Security headers
	if !strings.Contains(result, "X-Content-Type-Options nosniff") {
		t.Errorf("expected security headers, got:\n%s", result)
	}

	if !strings.Contains(result, `X-Powered-By "Launch"`) {
		t.Errorf("expected X-Powered-By Launch header, got:\n%s", result)
	}

	// PHP FastCGI block
	if !strings.Contains(result, "php_fastcgi unix//run/php/php8.3-fpm.sock") {
		t.Errorf("expected php_fastcgi block with php8.3 socket, got:\n%s", result)
	}

	if !strings.Contains(result, "resolve_root_symlink") {
		t.Errorf("expected resolve_root_symlink in php_fastcgi block, got:\n%s", result)
	}

	// No TLS configuration
	if strings.Contains(result, "tls ") || strings.Contains(result, "import tls-") {
		t.Errorf("load balanced Caddyfile should not have TLS configuration, got:\n%s", result)
	}

	// Log block
	if !strings.Contains(result, "output file /home/launcher/example.com/logs/caddy.log") {
		t.Errorf("expected log output file directive, got:\n%s", result)
	}

	// file_server
	if !strings.Contains(result, "file_server") {
		t.Errorf("expected file_server directive, got:\n%s", result)
	}
}

// TestGenerateLoadBalancedCaddyfile_StaticSite verifies that a static site
// behind a load balancer does not include a php_fastcgi block.
func TestGenerateLoadBalancedCaddyfile_StaticSite(t *testing.T) {
	site := newStaticSite()
	site.LoadBalancedUpstreamID = strPtr("01HLBUPSTREAM00000000002")
	loadBalancerIP := "10.0.0.1"

	result := generateLoadBalancedCaddyfile(site, nil, loadBalancerIP)

	// Should have HTTP on port 8080
	if !strings.Contains(result, "http://static.example.com:8080 {") {
		t.Errorf("expected http://static.example.com:8080, got:\n%s", result)
	}

	// Should NOT have php_fastcgi
	if strings.Contains(result, "php_fastcgi") {
		t.Errorf("static site should not have php_fastcgi block, got:\n%s", result)
	}

	// Should still have file_server
	if !strings.Contains(result, "file_server") {
		t.Errorf("expected file_server directive, got:\n%s", result)
	}

	// Root directory for static site with empty WebFolder
	expectedRoot := "root * /home/launcher/static.example.com/current"
	if !strings.Contains(result, expectedRoot) {
		t.Errorf("expected root directive %q, got:\n%s", expectedRoot, result)
	}
}

// TestGenerateLoadBalancedCaddyfile_WordPress verifies that a WordPress site
// behind a load balancer includes the WordPress-specific @disallowed rules.
func TestGenerateLoadBalancedCaddyfile_WordPress(t *testing.T) {
	site := newWordPressSite()
	site.LoadBalancedUpstreamID = strPtr("01HLBUPSTREAM00000000003")
	loadBalancerIP := "172.16.0.5"

	result := generateLoadBalancedCaddyfile(site, nil, loadBalancerIP)

	// WordPress @disallowed matcher
	if !strings.Contains(result, "@disallowed {") {
		t.Errorf("expected @disallowed block for WordPress, got:\n%s", result)
	}

	if !strings.Contains(result, "path /xmlrpc.php") {
		t.Errorf("expected /xmlrpc.php in @disallowed, got:\n%s", result)
	}

	if !strings.Contains(result, "path *.sql") {
		t.Errorf("expected *.sql in @disallowed, got:\n%s", result)
	}

	if !strings.Contains(result, "path /wp-content/uploads/*.php") {
		t.Errorf("expected /wp-content/uploads/*.php in @disallowed, got:\n%s", result)
	}

	if !strings.Contains(result, "rewrite @disallowed '/index.php'") {
		t.Errorf("expected rewrite @disallowed directive, got:\n%s", result)
	}

	// Should also have php_fastcgi
	if !strings.Contains(result, "php_fastcgi") {
		t.Errorf("WordPress site should have php_fastcgi block, got:\n%s", result)
	}
}

// TestGenerateLoadBalancedCaddyfile_WithRedirects verifies that custom redirects
// are correctly included in the load balanced Caddyfile.
func TestGenerateLoadBalancedCaddyfile_WithRedirects(t *testing.T) {
	site := newLaravelSite()
	site.LoadBalancedUpstreamID = strPtr("01HLBUPSTREAM00000000004")
	loadBalancerIP := "10.0.0.10"

	redirects := []models.Redirect{
		newRedirect("01HREDIR0000000000000001", "/old-page", "/new-page", 301),
		newRedirect("01HREDIR0000000000000002", "/temp-path", "/other-path", 302),
	}

	result := generateLoadBalancedCaddyfile(site, redirects, loadBalancerIP)

	// Custom redirects comment
	if !strings.Contains(result, "# Custom redirects") {
		t.Errorf("expected custom redirects comment, got:\n%s", result)
	}

	// Permanent redirect (301)
	if !strings.Contains(result, "redir /old-page /new-page permanent") {
		t.Errorf("expected permanent redirect directive, got:\n%s", result)
	}

	// Temporary redirect (302) - no "permanent" suffix
	if !strings.Contains(result, "redir /temp-path /other-path") {
		t.Errorf("expected temporary redirect directive, got:\n%s", result)
	}

	// Ensure the temporary redirect does NOT have "permanent" appended
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "redir /temp-path") && strings.Contains(trimmed, "permanent") {
			t.Errorf("temporary redirect should not have 'permanent' keyword, got: %s", trimmed)
		}
	}
}

// TestGenerateStandardCaddyfile_NonWwwSite verifies that a standard site with
// address "example.com" generates a www-to-non-www redirect block.
func TestGenerateStandardCaddyfile_NonWwwSite(t *testing.T) {
	site := newLaravelSite()

	result := generateStandardCaddyfile(site, nil)

	// Should redirect www.example.com to example.com
	if !strings.Contains(result, "www.example.com:443 {") {
		t.Errorf("expected www redirect block, got:\n%s", result)
	}

	if !strings.Contains(result, "redir {scheme}://example.com{uri}") {
		t.Errorf("expected redirect from www to non-www, got:\n%s", result)
	}

	// Main block should use example.com
	if !strings.Contains(result, "example.com:443 {") {
		t.Errorf("expected main server block with example.com:443, got:\n%s", result)
	}

	// TLS snippet
	if !strings.Contains(result, "(tls-01HTEST000000000000000001)") {
		t.Errorf("expected TLS snippet definition, got:\n%s", result)
	}

	if !strings.Contains(result, "import tls-01HTEST000000000000000001") {
		t.Errorf("expected TLS snippet import, got:\n%s", result)
	}
}

// TestGenerateStandardCaddyfile_WwwSite verifies that a standard site with
// address "www.example.com" generates a non-www-to-www redirect block.
func TestGenerateStandardCaddyfile_WwwSite(t *testing.T) {
	site := newLaravelSite()
	site.Address = "www.example.com"

	result := generateStandardCaddyfile(site, nil)

	// Should redirect example.com to www.example.com
	if !strings.Contains(result, "example.com:443 {") {
		t.Errorf("expected non-www redirect block, got:\n%s", result)
	}

	if !strings.Contains(result, "redir {scheme}://www.{host}{uri}") {
		t.Errorf("expected redirect from non-www to www, got:\n%s", result)
	}

	// Main block should use www.example.com
	if !strings.Contains(result, "www.example.com:443 {") {
		t.Errorf("expected main server block with www.example.com:443, got:\n%s", result)
	}
}

// TestGenerateStandardCaddyfile_OctaneReverseProxy verifies that a site with Octane enabled
// generates a reverse_proxy directive instead of php_fastcgi.
func TestGenerateStandardCaddyfile_OctaneReverseProxy(t *testing.T) {
	site := newLaravelSite()
	port := 8000
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", OctanePort: &port, OctaneServer: strPtr("frankenphp")},
	}

	result := generateStandardCaddyfile(site, nil)

	if !strings.Contains(result, "reverse_proxy localhost:8000") {
		t.Errorf("expected reverse_proxy localhost:8000, got:\n%s", result)
	}

	if strings.Contains(result, "php_fastcgi") {
		t.Errorf("Octane site should NOT have php_fastcgi block, got:\n%s", result)
	}
}

// TestGenerateStandardCaddyfile_OctaneDifferentPorts verifies different Octane ports.
func TestGenerateStandardCaddyfile_OctaneDifferentPorts(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port_8000", 8000},
		{"port_8001", 8001},
		{"port_8080", 8080},
		{"port_8999", 8999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := newLaravelSite()
			port := tt.port
			site.EnabledFeatures = models.EnabledFeaturesSlice{
				{Name: "octane", OctanePort: &port, OctaneServer: strPtr("swoole")},
			}

			result := generateStandardCaddyfile(site, nil)

			expected := fmt.Sprintf("reverse_proxy localhost:%d", tt.port)
			if !strings.Contains(result, expected) {
				t.Errorf("expected %s, got:\n%s", expected, result)
			}
		})
	}
}

// TestGenerateStandardCaddyfile_NoOctane_HasPhpFastcgi verifies that a site without Octane
// still uses php_fastcgi.
func TestGenerateStandardCaddyfile_NoOctane_HasPhpFastcgi(t *testing.T) {
	site := newLaravelSite()

	result := generateStandardCaddyfile(site, nil)

	if !strings.Contains(result, "php_fastcgi") {
		t.Errorf("non-Octane site should have php_fastcgi block, got:\n%s", result)
	}

	if strings.Contains(result, "reverse_proxy localhost:") {
		t.Errorf("non-Octane site should NOT have reverse_proxy, got:\n%s", result)
	}
}

// TestGenerateStandardCaddyfile_OctaneStillHasOtherDirectives verifies that Octane sites
// still include root, encode, headers, file_server, log.
func TestGenerateStandardCaddyfile_OctaneStillHasOtherDirectives(t *testing.T) {
	site := newLaravelSite()
	port := 8000
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", OctanePort: &port, OctaneServer: strPtr("frankenphp")},
	}

	result := generateStandardCaddyfile(site, nil)

	if !strings.Contains(result, "root * ") {
		t.Errorf("expected root directive, got:\n%s", result)
	}
	if !strings.Contains(result, "encode zstd gzip") {
		t.Errorf("expected encode directive, got:\n%s", result)
	}
	if !strings.Contains(result, "X-Powered-By") {
		t.Errorf("expected security headers, got:\n%s", result)
	}
	if !strings.Contains(result, "file_server") {
		t.Errorf("expected file_server, got:\n%s", result)
	}
	if !strings.Contains(result, "log {") {
		t.Errorf("expected log block, got:\n%s", result)
	}
}

// TestGenerateStandardCaddyfile_OctaneWithRedirects verifies Octane and custom redirects work together.
func TestGenerateStandardCaddyfile_OctaneWithRedirects(t *testing.T) {
	site := newLaravelSite()
	port := 8000
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", OctanePort: &port, OctaneServer: strPtr("frankenphp")},
	}

	redirects := []models.Redirect{
		newRedirect("01HREDIR0000000000000010", "/old", "/new", 301),
	}

	result := generateStandardCaddyfile(site, redirects)

	if !strings.Contains(result, "reverse_proxy localhost:8000") {
		t.Errorf("expected reverse_proxy, got:\n%s", result)
	}
	if !strings.Contains(result, "redir /old /new permanent") {
		t.Errorf("expected redirect, got:\n%s", result)
	}
}

// TestGenerateLoadBalancedCaddyfile_OctaneReverseProxy verifies that a load-balanced site
// with Octane enabled generates reverse_proxy instead of php_fastcgi.
func TestGenerateLoadBalancedCaddyfile_OctaneReverseProxy(t *testing.T) {
	site := newLaravelSite()
	site.LoadBalancedUpstreamID = strPtr("01HLBUPSTREAM00000000010")
	port := 8000
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", OctanePort: &port, OctaneServer: strPtr("swoole")},
	}
	loadBalancerIP := "10.0.0.50"

	result := generateLoadBalancedCaddyfile(site, nil, loadBalancerIP)

	if !strings.Contains(result, "\treverse_proxy localhost:8000") {
		t.Errorf("expected tab-indented reverse_proxy localhost:8000 in LB Caddyfile, got:\n%s", result)
	}

	if strings.Contains(result, "php_fastcgi") {
		t.Errorf("Octane LB site should NOT have php_fastcgi, got:\n%s", result)
	}

	// Should still have IP restriction
	if !strings.Contains(result, "@notlb not remote_ip 10.0.0.50") {
		t.Errorf("expected IP restriction, got:\n%s", result)
	}

	// Should still have http:// and port 8080
	if !strings.Contains(result, "http://example.com:8080") {
		t.Errorf("expected http://example.com:8080, got:\n%s", result)
	}
}

// TestGenerateLoadBalancedCaddyfile_NoOctane_HasPhpFastcgi verifies that LB site without
// Octane still uses php_fastcgi.
func TestGenerateLoadBalancedCaddyfile_NoOctane_HasPhpFastcgi(t *testing.T) {
	site := newLaravelSite()
	loadBalancerIP := "10.0.0.50"

	result := generateLoadBalancedCaddyfile(site, nil, loadBalancerIP)

	if !strings.Contains(result, "php_fastcgi") {
		t.Errorf("non-Octane LB site should have php_fastcgi, got:\n%s", result)
	}

	if strings.Contains(result, "reverse_proxy localhost:") {
		t.Errorf("non-Octane LB site should NOT have reverse_proxy, got:\n%s", result)
	}
}

// TestGenerateCaddyfile_OctaneRouting verifies the top-level generateCaddyfile function
// correctly routes to the right generator with Octane.
func TestGenerateCaddyfile_OctaneRouting(t *testing.T) {
	t.Run("standard_with_octane", func(t *testing.T) {
		site := newLaravelSite()
		port := 8000
		site.EnabledFeatures = models.EnabledFeaturesSlice{
			{Name: "octane", OctanePort: &port, OctaneServer: strPtr("frankenphp")},
		}

		result := generateCaddyfile(site, nil, "")

		if !strings.Contains(result, "reverse_proxy localhost:8000") {
			t.Errorf("expected reverse_proxy in standard Octane Caddyfile, got:\n%s", result)
		}
	})

	t.Run("load_balanced_with_octane", func(t *testing.T) {
		site := newLaravelSite()
		site.LoadBalancedUpstreamID = strPtr("01HLBUPSTREAM00000000020")
		port := 8001
		site.EnabledFeatures = models.EnabledFeaturesSlice{
			{Name: "octane", OctanePort: &port, OctaneServer: strPtr("roadrunner")},
		}

		result := generateCaddyfile(site, nil, "10.0.0.1")

		if !strings.Contains(result, "reverse_proxy localhost:8001") {
			t.Errorf("expected reverse_proxy in LB Octane Caddyfile, got:\n%s", result)
		}
	})
}

// TestGenerateStandardCaddyfile_OctaneNilPort verifies that if Octane feature exists
// but has nil port, we fall back to php_fastcgi.
func TestGenerateStandardCaddyfile_OctaneNilPort(t *testing.T) {
	site := newLaravelSite()
	site.EnabledFeatures = models.EnabledFeaturesSlice{
		{Name: "octane", OctaneServer: strPtr("frankenphp")},
	}

	result := generateStandardCaddyfile(site, nil)

	if strings.Contains(result, "reverse_proxy localhost:") {
		t.Errorf("nil octane port should fall back to php_fastcgi, got:\n%s", result)
	}

	if !strings.Contains(result, "php_fastcgi") {
		t.Errorf("expected php_fastcgi fallback for nil octane port, got:\n%s", result)
	}
}
