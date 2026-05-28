package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// ProvisionStep represents a step in the server provisioning process
type ProvisionStep string

const (
	// DetectOS is the first step in every provision flow. Sources
	// /etc/os-release + uname on the box, writes the result back via a
	// ::LAUNCH::detected_os marker. Downstream steps that need to
	// branch on distro (install_docker, install_php, …) consult the
	// detected fields rather than whatever the user picked in the
	// dashboard dropdown.
	ProvisionStepDetectOS                 ProvisionStep = "detect_os"
	ProvisionStepConfigureFirewall        ProvisionStep = "configure_firewall"
	ProvisionStepConfigureSwap            ProvisionStep = "configure_swap"
	ProvisionStepInstallEssentialPackages ProvisionStep = "install_essential_packages"
	ProvisionStepSetupDefaultUser         ProvisionStep = "setup_default_user"
	ProvisionStepSetupRoot                ProvisionStep = "setup_root"
	ProvisionStepSSHSecurity              ProvisionStep = "ssh_security"

	// Docker-stack steps
	ProvisionStepValidatePorts      ProvisionStep = "validate_ports"
	ProvisionStepInstallDocker      ProvisionStep = "install_docker"
	ProvisionStepSetupDockerNetwork ProvisionStep = "setup_docker_network"
	ProvisionStepSetupLaunchDirs    ProvisionStep = "setup_launch_dirs"
	ProvisionStepInstallTraefik     ProvisionStep = "install_traefik"

	// ProvisionStepSetupSwarmNetwork is the legacy step name from the
	// pre-v2 docker stack that initialised Docker Swarm and created an
	// overlay network. Kept here so old `server_provision_logs` rows
	// still parse via ParseProvisionStep — never re-emitted by new
	// provisions, never added to ForDockerServer().
	ProvisionStepSetupSwarmNetwork ProvisionStep = "setup_swarm_network"
)

var allProvisionSteps = []ProvisionStep{
	ProvisionStepDetectOS,
	ProvisionStepConfigureFirewall,
	ProvisionStepConfigureSwap,
	ProvisionStepInstallEssentialPackages,
	ProvisionStepSetupDefaultUser,
	ProvisionStepSetupRoot,
	ProvisionStepSSHSecurity,
	ProvisionStepValidatePorts,
	ProvisionStepInstallDocker,
	ProvisionStepSetupDockerNetwork,
	ProvisionStepSetupLaunchDirs,
	ProvisionStepInstallTraefik,
	// Legacy step — kept so historical logs still parse. Never
	// scheduled by ForDockerServer().
	ProvisionStepSetupSwarmNetwork,
}

var provisionStepLabels = map[ProvisionStep]string{
	ProvisionStepDetectOS:                 "Detect Operating System",
	ProvisionStepConfigureFirewall:        "Configure Firewall",
	ProvisionStepConfigureSwap:            "Configure Swap",
	ProvisionStepInstallEssentialPackages: "Install Essential Packages",
	ProvisionStepSetupDefaultUser:         "Setup Default User",
	ProvisionStepSetupRoot:                "Setup Root",
	ProvisionStepSSHSecurity:              "SSH Security",
	ProvisionStepValidatePorts:            "Validate Ports",
	ProvisionStepInstallDocker:            "Install Docker",
	ProvisionStepSetupDockerNetwork:       "Setup Launch Docker Network",
	ProvisionStepSetupLaunchDirs:          "Setup Launch Directories",
	ProvisionStepInstallTraefik:           "Install Traefik",
	// Legacy label — historical rows; new provisions never emit this.
	ProvisionStepSetupSwarmNetwork: "Setup Docker Swarm & Network (legacy)",
}

func (p ProvisionStep) String() string {
	return string(p)
}

// TemplateName returns the template path for this provision step
func (p ProvisionStep) TemplateName() string {
	templateNames := map[ProvisionStep]string{
		ProvisionStepDetectOS:                 "provision/detect_os.sh",
		ProvisionStepConfigureFirewall:        "provision/configure_firewall.sh",
		ProvisionStepConfigureSwap:            "provision/configure_swap.sh",
		ProvisionStepInstallEssentialPackages: "provision/install_essential_packages.sh",
		ProvisionStepSetupDefaultUser:         "provision/setup_default_user.sh",
		ProvisionStepSetupRoot:                "provision/setup_root.sh",
		ProvisionStepSSHSecurity:              "provision/ssh_security.sh",
		ProvisionStepValidatePorts:            "provision/validate_ports.sh",
		ProvisionStepInstallDocker:            "provision/install_docker.sh",
		ProvisionStepSetupDockerNetwork:       "provision/setup_docker_network.sh",
		ProvisionStepSetupLaunchDirs:          "provision/setup_launch_dirs.sh",
		ProvisionStepInstallTraefik:           "software/install_traefik.sh",
		// Legacy mapping retained so reruns of historical rows still
		// have a template to render (the file is also kept on disk).
		ProvisionStepSetupSwarmNetwork: "provision/setup_swarm_network.sh",
	}

	if name, ok := templateNames[p]; ok {
		return name
	}

	return "provision/" + string(p) + ".sh"
}

// RequiresData returns true if this step requires template data
func (p ProvisionStep) RequiresData() bool {
	switch p {
	case ProvisionStepConfigureSwap, ProvisionStepConfigureFirewall,
		ProvisionStepSetupRoot, ProvisionStepSetupDefaultUser,
		ProvisionStepInstallDocker, ProvisionStepSetupDockerNetwork,
		ProvisionStepSetupSwarmNetwork,
		ProvisionStepSetupLaunchDirs, ProvisionStepInstallTraefik:
		return true
	default:
		return false
	}
}

// Description returns a human-readable description of this step
func (p ProvisionStep) Description() string {
	descriptions := map[ProvisionStep]string{
		ProvisionStepDetectOS:                 "Detect the operating system, version, architecture and kernel",
		ProvisionStepConfigureFirewall:        "Configure the firewall with the default rules (SSH, HTTP, HTTPS)",
		ProvisionStepConfigureSwap:            "Configure a swap file so the server can handle more memory-intensive tasks",
		ProvisionStepInstallEssentialPackages: "Install essential packages (curl, git, wget, etc.)",
		ProvisionStepSetupDefaultUser:         "Create a default user account",
		ProvisionStepSetupRoot:                "Configure the root user",
		ProvisionStepSSHSecurity:              "Enhance SSH security by disabling password authentication and root login",
		ProvisionStepValidatePorts:            "Verify ports 80 and 443 are available for Traefik",
		ProvisionStepInstallDocker:            "Install Docker CE, Compose plugin, and Buildx",
		ProvisionStepSetupDockerNetwork:       "Create the launch-network bridge so Traefik can route to containers by name",
		ProvisionStepSetupLaunchDirs:          "Create the /etc/launch directory tree for service state",
		ProvisionStepInstallTraefik:           "Run Traefik as a container with docker-label discovery + ACME",
		// Legacy description retained for historical rows.
		ProvisionStepSetupSwarmNetwork: "Initialize Docker Swarm and create the launch overlay network (legacy)",
	}
	if desc, ok := descriptions[p]; ok {
		return desc
	}
	return string(p)
}

// Label returns a short label for UI display
func (p ProvisionStep) Label() string {
	return enumtypes.Label(p, provisionStepLabels, string(p))
}

// IsValid returns true if this is a valid provision step
func (p ProvisionStep) IsValid() bool {
	return enumtypes.IsValid(p, allProvisionSteps...)
}

// ForFreshServer returns the provision steps in order for a fresh server.
//
// Steps that used to exist here but were removed:
//
//   - "apt update + upgrade" — added 5-15 minutes per provision and broke
//     build reproducibility. Scripts that need fresh package metadata
//     (install_essential_packages, install_docker) run their own
//     `apt-get update` inline now.
//   - "setup_unattended_upgrades" — the unattended-upgrades package ships
//     sensible security-only defaults at install time (see Ubuntu's
//     pre-written /etc/apt/apt.conf.d/50unattended-upgrades and
//     20auto-upgrades). Our custom config wasn't adding meaningful
//     hardening over the defaults. The timer is re-enabled by
//     restoreUnattendedUpgrades() at the end of provisioning.
func ForFreshServer() []ProvisionStep {
	return []ProvisionStep{
		ProvisionStepDetectOS,
		ProvisionStepConfigureSwap,
		ProvisionStepConfigureFirewall,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupRoot,
		ProvisionStepSSHSecurity,
		ProvisionStepSetupDefaultUser,
	}
}

// ForDockerServer returns the provision steps in order for a fresh docker server.
// Reuses the base hardening steps then installs the Docker stack (validate
// ports, install Docker, create the launch-network bridge, create launch
// directory tree, run Traefik as a container with docker-label discovery).
//
// Pre-v2 servers were provisioned with Docker Swarm + an overlay network
// instead — the legacy ProvisionStepSetupSwarmNetwork is intentionally
// excluded here. Old servers keep working (their Traefik service is still
// running) but new provisions never opt in to swarm.
func ForDockerServer() []ProvisionStep {
	return []ProvisionStep{
		ProvisionStepDetectOS,
		ProvisionStepConfigureSwap,
		ProvisionStepConfigureFirewall,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupRoot,
		ProvisionStepSSHSecurity,
		ProvisionStepSetupDefaultUser,
		ProvisionStepValidatePorts,
		ProvisionStepInstallDocker,
		ProvisionStepSetupDockerNetwork,
		ProvisionStepSetupLaunchDirs,
		ProvisionStepInstallTraefik,
	}
}

// AllProvisionSteps returns all provision steps
func AllProvisionSteps() []ProvisionStep {
	return allProvisionSteps
}

// ParseProvisionStep parses a string into a ProvisionStep
func ParseProvisionStep(s string) (ProvisionStep, error) {
	step := ProvisionStep(s)
	if !step.IsValid() {
		return "", fmt.Errorf("invalid provision step: %s", s)
	}
	return step, nil
}

func (p *ProvisionStep) Scan(value interface{}) error {
	return enumtypes.ScanString(p, value)
}

func (p ProvisionStep) Value() (driver.Value, error) {
	return enumtypes.ValueString(p)
}
