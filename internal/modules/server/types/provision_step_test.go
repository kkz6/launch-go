package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForFreshServer_Order(t *testing.T) {
	steps := ForFreshServer()
	require := []ProvisionStep{
		ProvisionStepConfigureSwap,
		ProvisionStepConfigureFirewall,
		ProvisionStepAptUpdateUpgrade,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupUnattendedUpgrades,
		ProvisionStepSetupRoot,
		ProvisionStepSSHSecurity,
		ProvisionStepSetupDefaultUser,
	}
	assert.Equal(t, require, steps)
}

func TestForDockerServer_Order(t *testing.T) {
	steps := ForDockerServer()

	// Must contain all 8 base hardening steps in the same order as ForFreshServer,
	// then the 5 docker-specific steps appended.
	expected := []ProvisionStep{
		ProvisionStepConfigureSwap,
		ProvisionStepConfigureFirewall,
		ProvisionStepAptUpdateUpgrade,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupUnattendedUpgrades,
		ProvisionStepSetupRoot,
		ProvisionStepSSHSecurity,
		ProvisionStepSetupDefaultUser,
		ProvisionStepValidatePorts,
		ProvisionStepInstallDocker,
		ProvisionStepSetupSwarmNetwork,
		ProvisionStepSetupLaunchDirs,
		ProvisionStepInstallTraefik,
	}
	assert.Equal(t, expected, steps, "docker stack must run base hardening then docker-specific steps")
	assert.Len(t, steps, 13)
}

func TestForDockerServer_BaseStepsMatchFreshServer(t *testing.T) {
	fresh := ForFreshServer()
	docker := ForDockerServer()

	// The first N steps of the docker stack must be the fresh-server steps verbatim
	// so any future change to fresh-server hardening propagates to docker servers.
	require := len(fresh)
	assert.GreaterOrEqual(t, len(docker), require, "docker stack must include all fresh-server steps")
	assert.Equal(t, fresh, docker[:require], "docker stack must start with the fresh-server step sequence")
}

func TestProvisionStep_Docker_TemplateNames(t *testing.T) {
	cases := map[ProvisionStep]string{
		ProvisionStepValidatePorts:     "provision/validate_ports.sh",
		ProvisionStepInstallDocker:     "provision/install_docker.sh",
		ProvisionStepSetupSwarmNetwork: "provision/setup_swarm_network.sh",
		ProvisionStepSetupLaunchDirs:   "provision/setup_launch_dirs.sh",
		ProvisionStepInstallTraefik:    "software/install_traefik.sh",
	}
	for step, want := range cases {
		t.Run(string(step), func(t *testing.T) {
			assert.Equal(t, want, step.TemplateName())
		})
	}
}

func TestProvisionStep_Docker_RequiresData(t *testing.T) {
	for _, step := range []ProvisionStep{
		ProvisionStepInstallDocker,
		ProvisionStepSetupSwarmNetwork,
		ProvisionStepSetupLaunchDirs,
		ProvisionStepInstallTraefik,
	} {
		t.Run(string(step), func(t *testing.T) {
			assert.True(t, step.RequiresData(), "%s should require template data", step)
		})
	}
	assert.False(t, ProvisionStepValidatePorts.RequiresData(),
		"validate_ports is static and should not require data")
}

func TestProvisionStep_Docker_IsValid(t *testing.T) {
	for _, step := range []ProvisionStep{
		ProvisionStepValidatePorts,
		ProvisionStepInstallDocker,
		ProvisionStepSetupSwarmNetwork,
		ProvisionStepSetupLaunchDirs,
		ProvisionStepInstallTraefik,
	} {
		t.Run(string(step), func(t *testing.T) {
			assert.True(t, step.IsValid())
			assert.NotEmpty(t, step.Description())
			assert.NotEmpty(t, step.Label())
		})
	}
}
