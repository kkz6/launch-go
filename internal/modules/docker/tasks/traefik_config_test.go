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

func TestTraefikConfigPath(t *testing.T) {
	got := TraefikConfigPath("acme", "api")
	want := "/etc/launch/traefik/dynamic/acme-api.yml"
	if got != want {
		t.Fatalf("TraefikConfigPath = %q, want %q", got, want)
	}
}
