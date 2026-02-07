package services

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
)

// TraefikConfig represents the full Traefik dynamic config for a service
type TraefikConfig struct {
	HTTP TraefikHTTP `yaml:"http"`
}

// TraefikHTTP holds the HTTP routers and services configuration
type TraefikHTTP struct {
	Routers  map[string]TraefikRouter  `yaml:"routers"`
	Services map[string]TraefikService `yaml:"services"`
}

// TraefikRouter represents a Traefik router entry
type TraefikRouter struct {
	Rule        string      `yaml:"rule"`
	Service     string      `yaml:"service"`
	EntryPoints []string    `yaml:"entryPoints"`
	TLS         *TraefikTLS `yaml:"tls,omitempty"`
	Middlewares []string    `yaml:"middlewares,omitempty"`
}

// TraefikTLS holds TLS configuration for a router
type TraefikTLS struct {
	CertResolver string `yaml:"certResolver,omitempty"`
}

// TraefikService represents a Traefik service entry
type TraefikService struct {
	LoadBalancer TraefikLoadBalancer `yaml:"loadBalancer"`
}

// TraefikLoadBalancer holds load balancer config for a service
type TraefikLoadBalancer struct {
	Servers []TraefikServer `yaml:"servers"`
}

// TraefikServer represents a backend server in the load balancer
type TraefikServer struct {
	URL string `yaml:"url"`
}

// GenerateTraefikConfig generates the Traefik dynamic YAML config for a docker service
func GenerateTraefikConfig(svc *models.DockerService) (*TraefikConfig, error) {
	if len(svc.Domains) == 0 {
		return nil, nil
	}

	config := &TraefikConfig{
		HTTP: TraefikHTTP{
			Routers:  make(map[string]TraefikRouter),
			Services: make(map[string]TraefikService),
		},
	}

	serviceName := sanitizeName(svc.ContainerName)

	for _, domain := range svc.Domains {
		routerName := fmt.Sprintf("%s-%s", serviceName, sanitizeName(domain.Host))
		rule := buildRule(domain)

		router := TraefikRouter{
			Rule:        rule,
			Service:     serviceName,
			EntryPoints: []string{"websecure"},
		}

		if domain.HTTPS && domain.CertificateType == types.CertificateTypeLetsEncrypt {
			router.TLS = &TraefikTLS{CertResolver: "letsencrypt"}
		} else if domain.HTTPS {
			router.TLS = &TraefikTLS{}
		}

		config.HTTP.Routers[routerName] = router

		// Also add HTTP router for redirect if force SSL
		if domain.ForceSSL {
			httpRouterName := fmt.Sprintf("%s-http", routerName)
			config.HTTP.Routers[httpRouterName] = TraefikRouter{
				Rule:        rule,
				Service:     serviceName,
				EntryPoints: []string{"web"},
				Middlewares: []string{"redirect-to-https@file"},
			}
		}
	}

	// Service definition - point to container via Docker network
	port := 80
	if len(svc.Domains) > 0 {
		port = svc.Domains[0].ContainerPort
	}

	config.HTTP.Services[serviceName] = TraefikService{
		LoadBalancer: TraefikLoadBalancer{
			Servers: []TraefikServer{
				{URL: fmt.Sprintf("http://%s:%d", svc.ContainerName, port)},
			},
		},
	}

	return config, nil
}

func buildRule(domain models.DockerDomain) string {
	rule := fmt.Sprintf("Host(`%s`)", domain.Host)

	if domain.Path != "/" && domain.Path != "" {
		rule += fmt.Sprintf(" && PathPrefix(`%s`)", domain.Path)
	}

	return rule
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, ".", "-"), "_", "-")
}
