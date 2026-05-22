package tasks

// Constants for Docker-based server provisioning.
// Shared by Go callers and shell-script template data so the on-disk layout,
// network name, and pinned versions are defined in exactly one place.
const (
	// DockerNetworkName is the overlay network created on each docker server.
	DockerNetworkName = "launch-network"

	// DockerRootDir is the on-server root for launch-managed config and state.
	DockerRootDir = "/etc/launch"

	// DockerVersion is the docker-ce version stream installed via apt.
	// Empty means "latest from the docker apt repo"; pin if reproducibility matters.
	DockerVersion = ""

	// TraefikVersion is the Traefik image tag deployed as a Swarm service.
	TraefikVersion = "v3.1"

	// TraefikServiceName is the Swarm service name for the Traefik instance.
	TraefikServiceName = "launch-traefik"
)
