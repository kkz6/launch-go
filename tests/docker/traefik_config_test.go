package docker_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestGenerateTraefikConfig_SingleDomain(t *testing.T) {
	svc := &models.DockerService{
		ContainerName: "my-app-abc12345",
		Domains: []models.DockerDomain{
			{
				Host:            "example.com",
				Path:            "/",
				ContainerPort:   8080,
				HTTPS:           true,
				CertificateType: types.CertificateTypeLetsEncrypt,
				ForceSSL:        true,
			},
		},
	}

	config, err := services.GenerateTraefikConfig(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("expected config to be non-nil")
	}

	// Should have HTTPS router + HTTP redirect router
	if len(config.HTTP.Routers) != 2 {
		t.Errorf("expected 2 routers (HTTPS + HTTP redirect), got %d", len(config.HTTP.Routers))
	}

	// Check HTTPS router
	routerName := "my-app-abc12345-example-com"
	router, ok := config.HTTP.Routers[routerName]
	if !ok {
		t.Fatalf("expected router %q to exist", routerName)
	}

	if router.Rule != "Host(`example.com`)" {
		t.Errorf("expected rule 'Host(`example.com`)', got %q", router.Rule)
	}

	if router.TLS == nil {
		t.Fatal("expected TLS to be configured")
	}

	if router.TLS.CertResolver != "letsencrypt" {
		t.Errorf("expected cert resolver 'letsencrypt', got %q", router.TLS.CertResolver)
	}

	if len(router.EntryPoints) != 1 || router.EntryPoints[0] != "websecure" {
		t.Errorf("expected entrypoint 'websecure', got %v", router.EntryPoints)
	}

	// Check HTTP redirect router
	httpRouterName := routerName + "-http"
	httpRouter, ok := config.HTTP.Routers[httpRouterName]
	if !ok {
		t.Fatalf("expected HTTP redirect router %q to exist", httpRouterName)
	}

	if len(httpRouter.EntryPoints) != 1 || httpRouter.EntryPoints[0] != "web" {
		t.Errorf("expected entrypoint 'web', got %v", httpRouter.EntryPoints)
	}

	if len(httpRouter.Middlewares) != 1 || httpRouter.Middlewares[0] != "redirect-to-https@file" {
		t.Errorf("expected redirect middleware, got %v", httpRouter.Middlewares)
	}

	// Check service backend
	serviceName := "my-app-abc12345"
	svcConfig, ok := config.HTTP.Services[serviceName]
	if !ok {
		t.Fatalf("expected service %q to exist", serviceName)
	}

	if len(svcConfig.LoadBalancer.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(svcConfig.LoadBalancer.Servers))
	}

	expectedURL := "http://my-app-abc12345:8080"
	if svcConfig.LoadBalancer.Servers[0].URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, svcConfig.LoadBalancer.Servers[0].URL)
	}
}

func TestGenerateTraefikConfig_WithPathPrefix(t *testing.T) {
	svc := &models.DockerService{
		ContainerName: "api-abc12345",
		Domains: []models.DockerDomain{
			{
				Host:            "example.com",
				Path:            "/api/v1",
				ContainerPort:   3000,
				HTTPS:           true,
				CertificateType: types.CertificateTypeLetsEncrypt,
				ForceSSL:        false,
			},
		},
	}

	config, err := services.GenerateTraefikConfig(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No ForceSSL = no HTTP redirect router
	if len(config.HTTP.Routers) != 1 {
		t.Errorf("expected 1 router (no redirect), got %d", len(config.HTTP.Routers))
	}

	routerName := "api-abc12345-example-com"
	router := config.HTTP.Routers[routerName]

	expectedRule := "Host(`example.com`) && PathPrefix(`/api/v1`)"
	if router.Rule != expectedRule {
		t.Errorf("expected rule %q, got %q", expectedRule, router.Rule)
	}
}

func TestGenerateTraefikConfig_NoDomains(t *testing.T) {
	svc := &models.DockerService{
		ContainerName: "worker-abc12345",
		Domains:       nil,
	}

	config, err := services.GenerateTraefikConfig(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config != nil {
		t.Error("expected nil config for service with no domains")
	}
}

func TestGenerateTraefikConfig_MultipleDomains(t *testing.T) {
	svc := &models.DockerService{
		ContainerName: "web-abc12345",
		Domains: []models.DockerDomain{
			{
				Host:            "example.com",
				Path:            "/",
				ContainerPort:   80,
				HTTPS:           true,
				CertificateType: types.CertificateTypeLetsEncrypt,
				ForceSSL:        true,
			},
			{
				Host:            "www.example.com",
				Path:            "/",
				ContainerPort:   80,
				HTTPS:           true,
				CertificateType: types.CertificateTypeLetsEncrypt,
				ForceSSL:        true,
			},
		},
	}

	config, err := services.GenerateTraefikConfig(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 domains x 2 routers each (HTTPS + HTTP redirect) = 4 routers
	if len(config.HTTP.Routers) != 4 {
		t.Errorf("expected 4 routers, got %d", len(config.HTTP.Routers))
	}

	// Should still have a single service definition
	if len(config.HTTP.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(config.HTTP.Services))
	}
}

func TestGenerateTraefikConfig_CustomCertificate(t *testing.T) {
	svc := &models.DockerService{
		ContainerName: "app-abc12345",
		Domains: []models.DockerDomain{
			{
				Host:            "internal.example.com",
				Path:            "/",
				ContainerPort:   8080,
				HTTPS:           true,
				CertificateType: types.CertificateTypeCustom,
				ForceSSL:        false,
			},
		},
	}

	config, err := services.GenerateTraefikConfig(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	routerName := "app-abc12345-internal-example-com"
	router := config.HTTP.Routers[routerName]

	if router.TLS == nil {
		t.Fatal("expected TLS to be configured for custom cert")
	}

	if router.TLS.CertResolver != "" {
		t.Errorf("expected empty cert resolver for custom cert, got %q", router.TLS.CertResolver)
	}
}

func TestGenerateTraefikConfig_NoHTTPS(t *testing.T) {
	svc := &models.DockerService{
		ContainerName: "dev-abc12345",
		Domains: []models.DockerDomain{
			{
				Host:            "dev.local",
				Path:            "/",
				ContainerPort:   3000,
				HTTPS:           false,
				CertificateType: types.CertificateTypeNone,
				ForceSSL:        false,
			},
		},
	}

	config, err := services.GenerateTraefikConfig(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	routerName := "dev-abc12345-dev-local"
	router := config.HTTP.Routers[routerName]

	if router.TLS != nil {
		t.Error("expected no TLS for non-HTTPS domain")
	}
}
