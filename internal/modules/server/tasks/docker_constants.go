package tasks

// Constants for Docker-based server provisioning.
// Shared by Go callers and shell-script template data so the on-disk layout,
// network name, and pinned versions are defined in exactly one place.
const (
	// DockerNetworkName is the bridge network created on each docker
	// server. Traefik and every customer workload attach to it so
	// docker's built-in DNS lets Traefik reach containers by name.
	// Legacy (pre-v2) servers have this as an overlay network from the
	// swarm install — the host-inspect code is driver-agnostic.
	DockerNetworkName = "launch-network"

	// DockerRootDir is the on-server root for launch-managed config and state.
	DockerRootDir = "/etc/launch"

	// DockerVersion is the docker-ce version stream installed via apt.
	// Empty means "latest from the docker apt repo"; pin if reproducibility matters.
	DockerVersion = ""

	// TraefikVersion is the Traefik image tag used for the reverse-proxy
	// container on a docker server.
	TraefikVersion = "v3.1"

	// TraefikContainerName is the container name for the Traefik
	// instance running on a docker server. Pre-v2 servers running the
	// legacy swarm install use the same name as a swarm service name,
	// which is why the host-inspect helper still understands the
	// `.replica.taskid` suffix that swarm appends to spawned
	// containers.
	TraefikContainerName = "launch-traefik"

	// TraefikServiceName is the legacy alias kept so call sites that
	// haven't been updated still compile. New code should use
	// TraefikContainerName.
	//
	// Deprecated: use TraefikContainerName.
	TraefikServiceName = TraefikContainerName
)
