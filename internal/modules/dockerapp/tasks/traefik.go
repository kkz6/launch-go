package tasks

import "fmt"

// DomainSpec describes one externally-routable domain for label
// generation. Sourced from the Domain model at deploy time.
type DomainSpec struct {
	Domain        string
	ContainerPort int
	TLS           bool
}

// TraefikLabels returns the docker labels needed for Traefik to route
// the given domains to the application container.
//
// We generate one router and one service per (app + first port). When
// multiple domains share the same container port, they piggy-back on
// the same Traefik service to avoid duplicate upstream definitions.
//
// Single-host launch-network deployment, no replicas.
func TraefikLabels(appName string, domains []DomainSpec) []string {
	if len(domains) == 0 {
		return nil
	}

	labels := []string{"traefik.enable=true"}

	// Group by container port so multiple domains pointing at the same
	// upstream port reuse a service definition.
	type group struct {
		port    int
		domains []DomainSpec
	}
	var groups []*group
	for _, d := range domains {
		var g *group
		for _, existing := range groups {
			if existing.port == d.ContainerPort {
				g = existing
				break
			}
		}
		if g == nil {
			g = &group{port: d.ContainerPort}
			groups = append(groups, g)
		}
		g.domains = append(g.domains, d)
	}

	for i, g := range groups {
		serviceName := fmt.Sprintf("%s-%d", appName, g.port)

		// One router per domain so HTTPS / HTTP can be toggled
		// independently.
		for j, d := range g.domains {
			routerName := fmt.Sprintf("%s-%d-%d", appName, i, j)
			labels = append(labels,
				fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", routerName, d.Domain),
				fmt.Sprintf("traefik.http.routers.%s.service=%s", routerName, serviceName),
			)
			if d.TLS {
				labels = append(labels,
					fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", routerName),
					fmt.Sprintf("traefik.http.routers.%s.tls=true", routerName),
					fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=letsencrypt", routerName),
				)
			} else {
				labels = append(labels,
					fmt.Sprintf("traefik.http.routers.%s.entrypoints=web", routerName),
				)
			}
		}

		// One service per port group.
		labels = append(labels,
			fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d", serviceName, g.port),
		)
	}

	return labels
}
