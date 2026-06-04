package tasks

import (
	"strings"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
)

func TestRenderTraefikConfig_NoDomains(t *testing.T) {
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  80,
	})
	if !strings.Contains(out, "no domains configured") {
		t.Fatalf("expected placeholder marker, got:\n%s", out)
	}
	if !strings.Contains(out, "http: {}") {
		t.Fatalf("expected empty http stanza, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_HTTPOnly(t *testing.T) {
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  3000,
		Domains: []models.ApplicationDomain{
			{Host: "api.example.com", HTTPS: false},
		},
	})
	// Compose the expected literal — raw strings can't contain backticks,
	// so we sandwich them with a "`" literal.
	wantRule := "rule: \"Host(`api.example.com`)\""
	if !strings.Contains(out, wantRule) {
		t.Fatalf("expected %q, got:\n%s", wantRule, out)
	}
	// HTTP-only domain → no websecure router, no tls block.
	if strings.Contains(out, "websecure") {
		t.Fatalf("HTTP-only domain shouldn't emit websecure router, got:\n%s", out)
	}
	if strings.Contains(out, "tls:") {
		t.Fatalf("HTTP-only domain shouldn't emit tls block, got:\n%s", out)
	}
	if !strings.Contains(out, `url: "http://launch-acme-api:3000"`) {
		t.Fatalf("expected service to point to container:port, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_HTTPSWithRedirect(t *testing.T) {
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{Host: "api.example.com", HTTPS: true},
		},
	})
	// HTTPS domain → HTTP router redirects to HTTPS via the
	// middleware Traefik provisioning already set up.
	if !strings.Contains(out, "middlewares: [redirect-to-https]") {
		t.Fatalf("expected redirect-to-https middleware on http router, got:\n%s", out)
	}
	if !strings.Contains(out, "entryPoints: [websecure]") {
		t.Fatalf("expected websecure router for HTTPS domain, got:\n%s", out)
	}
	if !strings.Contains(out, "certresolver: letsencrypt") {
		t.Fatalf("expected letsencrypt cert resolver, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_PathPrefix(t *testing.T) {
	path := "/api/v1"
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{Host: "example.com", Path: &path, HTTPS: false},
		},
	})
	if !strings.Contains(out, "PathPrefix(`/api/v1`)") {
		t.Fatalf("expected PathPrefix rule, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_MultipleDomains(t *testing.T) {
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{Host: "api.example.com", HTTPS: true},
			{Host: "alt.example.com", HTTPS: false},
		},
	})
	// Each domain gets its own router pair indexed by position so multiple
	// domains don't collide on router names — a real bug when we had a
	// single hardcoded router name and only the last domain worked.
	if !strings.Contains(out, "acme-api-0-http") {
		t.Fatalf("expected router for first domain, got:\n%s", out)
	}
	if !strings.Contains(out, "acme-api-1-http") {
		t.Fatalf("expected router for second domain, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_ContainerPortOverridesInternalPort(t *testing.T) {
	cp := 3000
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "web",
		ContainerName: "launch-acme-web",
		InternalPort:  80, // deliberately "wrong" — must be overridden
		Domains: []models.ApplicationDomain{
			{Host: "app.example.com", HTTPS: true, ContainerPort: &cp},
		},
	})
	// Backend must target the domain's container_port, NOT internal_port.
	if !strings.Contains(out, `url: "http://launch-acme-web:3000"`) {
		t.Fatalf("expected backend on container_port 3000, got:\n%s", out)
	}
	if strings.Contains(out, `:80"`) {
		t.Fatalf("internal_port 80 must not be used when container_port is set, got:\n%s", out)
	}
	// Routers must reference the port-suffixed service.
	if !strings.Contains(out, "service: acme-web-3000") {
		t.Fatalf("expected router to reference port-suffixed service, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_FallsBackToInternalPort(t *testing.T) {
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  8080,
		Domains: []models.ApplicationDomain{
			{Host: "api.example.com", HTTPS: true}, // no container_port
		},
	})
	if !strings.Contains(out, `url: "http://launch-acme-api:8080"`) {
		t.Fatalf("expected fallback to internal_port 8080, got:\n%s", out)
	}
	if !strings.Contains(out, "service: acme-api-8080") {
		t.Fatalf("expected port-suffixed service for fallback, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_PerDomainDistinctPorts(t *testing.T) {
	front := 3000
	api := 8080
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "gj",
		AppSlug:       "stack",
		ContainerName: "launch-gj-stack",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{Host: "app.example.com", HTTPS: true, ContainerPort: &front},
			{Host: "api.example.com", HTTPS: true, ContainerPort: &api},
		},
	})
	// One service per distinct port, each pointing at the right port.
	if !strings.Contains(out, `url: "http://launch-gj-stack:3000"`) ||
		!strings.Contains(out, `url: "http://launch-gj-stack:8080"`) {
		t.Fatalf("expected a service per distinct port, got:\n%s", out)
	}
	if !strings.Contains(out, "service: gj-stack-3000") ||
		!strings.Contains(out, "service: gj-stack-8080") {
		t.Fatalf("expected routers to reference both port services, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_SharedPortEmitsOneService(t *testing.T) {
	p := 3000
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "web",
		ContainerName: "launch-acme-web",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{Host: "a.example.com", HTTPS: true, ContainerPort: &p},
			{Host: "b.example.com", HTTPS: true, ContainerPort: &p},
		},
	})
	// Two domains, same port → exactly one service block (one loadBalancer).
	if got := strings.Count(out, "loadBalancer:"); got != 1 {
		t.Fatalf("expected one service for a shared port, got %d. yaml:\n%s", got, out)
	}
}

func TestTraefikConfigPath(t *testing.T) {
	got := TraefikConfigPath("acme", "api")
	want := "/etc/launch/traefik/dynamic/acme-api.yml"
	if got != want {
		t.Fatalf("TraefikConfigPath = %q, want %q", got, want)
	}
}

func TestRenderTraefikConfig_StoredCertificate(t *testing.T) {
	certID := "01abcdefghijklmnopqrstuvwx"
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{
				Host:                "api.example.com",
				HTTPS:               true,
				CertificateProvider: "stored",
				StoredCertificateID: &certID,
			},
		},
	})
	// Stored-cert routers must NOT emit certresolver: letsencrypt;
	// that path is reserved for the auto-TLS branch.
	if strings.Contains(out, "certresolver: letsencrypt") {
		t.Fatalf("stored-cert router must not reference letsencrypt resolver, got:\n%s", out)
	}
	// Cert files are referenced by their in-container path
	// (/etc/traefik/...), NOT the host path (/etc/launch/traefik/...) —
	// the install_traefik script mounts the host dir at /etc/traefik
	// inside the container.
	if !strings.Contains(out, "certFile: /etc/traefik/certs/"+certID+"/cert.pem") {
		t.Fatalf("expected certFile reference at /etc/traefik/certs/<id>/cert.pem, got:\n%s", out)
	}
	if !strings.Contains(out, "keyFile: /etc/traefik/certs/"+certID+"/key.pem") {
		t.Fatalf("expected keyFile reference at /etc/traefik/certs/<id>/key.pem, got:\n%s", out)
	}
}

func TestRenderTraefikConfig_StoredCertificatesDeduped(t *testing.T) {
	certID := "01abcdefghijklmnopqrstuvwx"
	out := RenderTraefikConfig(TraefikConfigArgs{
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		InternalPort:  80,
		Domains: []models.ApplicationDomain{
			{Host: "a.example.com", HTTPS: true, CertificateProvider: "stored", StoredCertificateID: &certID},
			{Host: "b.example.com", HTTPS: true, CertificateProvider: "stored", StoredCertificateID: &certID},
		},
	})
	// Two domains, same cert id → one tls.certificates entry only.
	if got := strings.Count(out, "certFile: /etc/traefik/certs/"+certID+"/cert.pem"); got != 1 {
		t.Fatalf("expected one cert-file entry for shared cert, got %d. yaml:\n%s", got, out)
	}
}

func TestWriteStoredCertificatesTask_NoOpForEmpty(t *testing.T) {
	task := WriteStoredCertificatesTask(nil)
	script := task.Script()
	if !strings.Contains(script, "site_certs::skipped") &&
		!strings.Contains(script, "stored_certs::skipped") {
		t.Fatalf("empty materials should produce a no-op script, got:\n%s", script)
	}
}

func TestWriteStoredCertificatesTask_WritesPerCertDir(t *testing.T) {
	cid := "01abcdefghijklmnopqrstuvwx"
	task := WriteStoredCertificatesTask([]StoredCertMaterial{{
		CertificateID: cid,
		CertPEM:       "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
		KeyPEM:        "-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----",
	}})
	script := task.Script()
	wantDir := StoredCertHostDir(cid)
	if !strings.Contains(script, wantDir+"\"") && !strings.Contains(script, "\""+wantDir) {
		t.Fatalf("script must reference per-cert dir %s, got:\n%s", wantDir, script)
	}
	if !strings.Contains(script, "chmod 0600") {
		t.Fatalf("cert files must be chmod 0600 for safety, got:\n%s", script)
	}
}

func TestStoredCertHostDir_NamespacedUnderEtcLaunch(t *testing.T) {
	got := StoredCertHostDir("01abc")
	want := "/etc/launch/traefik/certs/01abc"
	if got != want {
		t.Fatalf("StoredCertHostDir = %q, want %q", got, want)
	}
}
