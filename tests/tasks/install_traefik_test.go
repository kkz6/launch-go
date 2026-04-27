package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestInstallTraefik_Name(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareTraefik, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).HasName("Install Traefik")
}

func TestInstallTraefik_WritesStaticConfig(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareTraefik, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"/etc/launch/traefik/traefik.yml",
		"entryPoints:",
		"  web:",
		"  websecure:",
		"providers:",
		"  docker:",
		"network: launch-network",
	)
}

func TestInstallTraefik_RunsContainer(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareTraefik, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker rm -f launch-traefik",
		"docker run -d",
		"--name launch-traefik",
		"--network launch-network",
		"-p 80:80",
		"-p 443:443",
		"/var/run/docker.sock:/var/run/docker.sock",
		"traefik:v3.6",
	)
}

func TestInstallTraefik_AcmeJsonHasTightPerms(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareTraefik, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"/etc/launch/traefik/acme.json",
		"chmod 600 /etc/launch/traefik/acme.json",
	)
}

func TestInstallTraefik_NoEmailMeansNoACME(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareTraefik, tasks.SoftwareInstallConfig{
		Username: "launcher",
		// TraefikAdminEmail intentionally empty
	})
	testutil.AssertTask(t, task).
		ScriptNotContains("certificatesResolvers").
		ScriptNotContains("letsencrypt")
}

func TestInstallTraefik_WithEmailEnablesACME(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareTraefik, tasks.SoftwareInstallConfig{
		Username:          "launcher",
		TraefikAdminEmail: "ops@example.com",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"certificatesResolvers:",
		"letsencrypt:",
		"email: ops@example.com",
		"storage: /etc/launch/traefik/acme.json",
	)
}
