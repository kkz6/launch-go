package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestInstallDocker_Name(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).HasName("Install Docker")
}

func TestInstallDocker_InstallsEngineAndCompose(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker-ce",
		"docker-ce-cli",
		"containerd.io",
		"docker-compose-plugin",
		"docker-buildx-plugin",
	)
}

func TestInstallDocker_AddsAptRepo(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"download.docker.com/linux/ubuntu/gpg",
		"/etc/apt/keyrings/docker.gpg",
		"/etc/apt/sources.list.d/docker.list",
	)
}

func TestInstallDocker_RejectsSnap(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContains("not supported")
}

func TestInstallDocker_EnablesAndStartsService(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContains("systemctl enable --now docker")
}

func TestInstallDocker_AddsUserToDockerGroup(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContains("usermod -aG docker launcher")
}

func TestInstallDocker_CreatesLaunchDirectoryTree(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"/etc/launch",
		"traefik/dynamic",
		"apps",
		"compose",
		"chown -R launcher:launcher /etc/launch",
	)
}

func TestInstallDocker_CreatesNetwork(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	testutil.AssertTask(t, task).ScriptContains("docker network create --driver bridge launch-network")
}

func TestInstallDocker_NetworkCreateIsIdempotent(t *testing.T) {
	task := tasks.InstallSoftware(types.SoftwareDocker, tasks.SoftwareInstallConfig{
		Username: "launcher",
	})
	// Idempotency: only create the network if `docker network inspect` fails.
	testutil.AssertTask(t, task).ScriptContains("docker network inspect launch-network")
}
