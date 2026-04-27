package tasks

import (
	"strings"
	"testing"
)

func TestTraefikLabels_NoDomainsReturnsNil(t *testing.T) {
	if labels := TraefikLabels("myapp", nil); labels != nil {
		t.Errorf("expected nil, got %v", labels)
	}
}

func TestTraefikLabels_SingleHTTPSDomain(t *testing.T) {
	labels := TraefikLabels("myapp", []DomainSpec{
		{Domain: "api.example.com", ContainerPort: 8080, TLS: true},
	})
	joined := strings.Join(labels, "\n")
	for _, want := range []string{
		"traefik.enable=true",
		"Host(`api.example.com`)",
		"loadbalancer.server.port=8080",
		"tls.certresolver=letsencrypt",
		"entrypoints=websecure",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in labels:\n%s", want, joined)
		}
	}
}

func TestTraefikLabels_HTTPOnlyDomain(t *testing.T) {
	labels := TraefikLabels("myapp", []DomainSpec{
		{Domain: "old.example.com", ContainerPort: 80, TLS: false},
	})
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, "entrypoints=web") {
		t.Errorf("expected entrypoints=web, got:\n%s", joined)
	}
	if strings.Contains(joined, "letsencrypt") {
		t.Errorf("HTTP-only domain must not pull in letsencrypt:\n%s", joined)
	}
}

func TestTraefikLabels_MultipleDomainsSamePortShareService(t *testing.T) {
	labels := TraefikLabels("myapp", []DomainSpec{
		{Domain: "a.example.com", ContainerPort: 8080, TLS: true},
		{Domain: "b.example.com", ContainerPort: 8080, TLS: true},
	})
	joined := strings.Join(labels, "\n")
	count := strings.Count(joined, "loadbalancer.server.port=8080")
	if count != 1 {
		t.Errorf("expected one shared service, got %d:\n%s", count, joined)
	}
	for _, want := range []string{"a.example.com", "b.example.com"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q:\n%s", want, joined)
		}
	}
}

func TestTraefikLabels_DifferentPortsGetSeparateServices(t *testing.T) {
	labels := TraefikLabels("myapp", []DomainSpec{
		{Domain: "api.example.com", ContainerPort: 8080, TLS: true},
		{Domain: "ws.example.com", ContainerPort: 9090, TLS: true},
	})
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, "loadbalancer.server.port=8080") ||
		!strings.Contains(joined, "loadbalancer.server.port=9090") {
		t.Errorf("expected services for both ports:\n%s", joined)
	}
}
