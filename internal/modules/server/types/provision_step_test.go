package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForFreshServer_Order(t *testing.T) {
	steps := ForFreshServer()
	require := []ProvisionStep{
		ProvisionStepDetectOS,
		ProvisionStepConfigureSwap,
		ProvisionStepConfigureFirewall,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupRoot,
		ProvisionStepSSHSecurity,
		ProvisionStepSetupDefaultUser,
	}
	assert.Equal(t, require, steps)
}

func TestForDockerServer_Order(t *testing.T) {
	steps := ForDockerServer()

	// Must contain all 7 base hardening steps in the same order as ForFreshServer
	// (detect_os + 6 base hardening), then the 5 docker-specific steps appended.
	// Swarm step is intentionally excluded from the fresh-provision flow — see
	// ForDockerServer() comment.
	expected := []ProvisionStep{
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
	assert.Equal(t, expected, steps, "docker stack must run base hardening then docker-specific steps")
	assert.Len(t, steps, 12)

	// Swarm step is reachable via the enum (for parsing historical rows)
	// but must NEVER appear in the live provision sequence.
	for _, step := range steps {
		assert.NotEqual(t, ProvisionStepSetupSwarmNetwork, step,
			"legacy swarm step must not be scheduled by v2 provisions")
	}
}

// TestForFreshServer_OmitsRemovedSteps pins which historical steps must
// not come back. Re-adding requires reconsidering the speed/UX tradeoff
// that drove their removal.
func TestForFreshServer_OmitsRemovedSteps(t *testing.T) {
	removed := []string{"apt_update_upgrade", "setup_unattended_upgrades"}
	for _, list := range [][]ProvisionStep{ForFreshServer(), ForDockerServer()} {
		for _, step := range list {
			for _, r := range removed {
				assert.NotEqual(t, r, string(step),
					"%s was deliberately removed from provisioning", r)
			}
		}
	}
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
		ProvisionStepValidatePorts:      "provision/validate_ports.sh",
		ProvisionStepInstallDocker:      "provision/install_docker.sh",
		ProvisionStepSetupDockerNetwork: "provision/setup_docker_network.sh",
		ProvisionStepSetupLaunchDirs:    "provision/setup_launch_dirs.sh",
		ProvisionStepInstallTraefik:     "software/install_traefik.sh",
		// Legacy step retained so old rows still parse. Template file
		// is still on disk for the same reason.
		ProvisionStepSetupSwarmNetwork: "provision/setup_swarm_network.sh",
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
		ProvisionStepSetupDockerNetwork,
		ProvisionStepSetupSwarmNetwork, // legacy; still expected to claim it needs data
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
		ProvisionStepSetupDockerNetwork,
		ProvisionStepSetupSwarmNetwork, // legacy; remains valid so historical rows parse
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
