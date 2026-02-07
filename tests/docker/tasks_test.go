package docker_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestDeployDockerService_BasicConfig(t *testing.T) {
	config := tasks.DeployConfig{
		ContainerName: "my-app-abc12345",
		Image:         "nginx:latest",
		RestartPolicy: "unless-stopped",
	}

	task := tasks.DeployDockerService(config, "svc-1", "deploy-1", "team-1", "server-1")

	testutil.AssertTask(t, task).
		HasName("Deploy Docker Service").
		ScriptContainsAll(
			"my-app-abc12345",
			"nginx:latest",
			"docker pull nginx:latest",
			"docker run -d",
			"--restart unless-stopped",
			"--network launch-network",
		)
}

func TestDeployDockerService_WithEnvVarsAndPorts(t *testing.T) {
	config := tasks.DeployConfig{
		ContainerName: "redis-abc12345",
		Image:         "redis:7-alpine",
		RestartPolicy: "always",
		EnvVars: []tasks.EnvVar{
			{Key: "REDIS_PASSWORD", Value: "secret123"},
			{Key: "REDIS_PORT", Value: "6379"},
		},
		Ports: []tasks.Port{
			{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"},
		},
	}

	task := tasks.DeployDockerService(config, "svc-2", "deploy-2", "team-1", "server-1")

	testutil.AssertTask(t, task).
		HasName("Deploy Docker Service").
		ScriptContainsAll(
			`-e "REDIS_PASSWORD=secret123"`,
			`-e "REDIS_PORT=6379"`,
			"-p 6379:6379/tcp",
		)
}

func TestDeployDockerService_WithVolumes(t *testing.T) {
	config := tasks.DeployConfig{
		ContainerName: "postgres-abc12345",
		Image:         "postgres:16",
		RestartPolicy: "unless-stopped",
		Volumes: []tasks.Volume{
			{Source: "pgdata", Target: "/var/lib/postgresql/data"},
			{Source: "/host/config", Target: "/etc/postgresql", ReadOnly: true},
		},
	}

	task := tasks.DeployDockerService(config, "svc-3", "deploy-3", "team-1", "server-1")

	testutil.AssertTask(t, task).
		ScriptContainsAll(
			"-v pgdata:/var/lib/postgresql/data",
			"-v /host/config:/etc/postgresql:ro",
		)
}

func TestDeployDockerService_WithRegistry(t *testing.T) {
	config := tasks.DeployConfig{
		ContainerName:    "my-app-abc12345",
		Image:            "registry.example.com/my-app:v1.0",
		RestartPolicy:    "always",
		RegistryURL:      "registry.example.com",
		RegistryUsername: "deploy",
		RegistryPassword: "token123",
	}

	task := tasks.DeployDockerService(config, "svc-4", "deploy-4", "team-1", "server-1")

	testutil.AssertTask(t, task).
		ScriptContainsAll(
			"docker login registry.example.com",
			`-u "deploy"`,
			"registry.example.com/my-app:v1.0",
		)
}

func TestDeployDockerService_WithResourceLimits(t *testing.T) {
	config := tasks.DeployConfig{
		ContainerName: "worker-abc12345",
		Image:         "my-worker:latest",
		RestartPolicy: "on-failure",
		CPULimit:      "0.5",
		MemoryLimit:   "512",
		Command:       "worker --concurrency=4",
	}

	task := tasks.DeployDockerService(config, "svc-5", "deploy-5", "team-1", "server-1")

	testutil.AssertTask(t, task).
		ScriptContainsAll(
			"--cpus=0.5",
			"--memory=512m",
			"worker --concurrency=4",
		)
}

func TestDeployDockerService_CallbackData(t *testing.T) {
	config := tasks.DeployConfig{
		ContainerName: "test-abc12345",
		Image:         "test:latest",
		RestartPolicy: "unless-stopped",
	}

	task := tasks.DeployDockerService(config, "svc-99", "deploy-99", "team-42", "server-7")

	if task.TypeName() != tasks.DeployDockerServiceTaskType {
		t.Errorf("Expected type %q, got %q", tasks.DeployDockerServiceTaskType, task.TypeName())
	}

	payload, err := task.MarshalPayload()
	if err != nil {
		t.Fatalf("MarshalPayload failed: %v", err)
	}

	if len(payload) == 0 {
		t.Fatal("Expected non-empty payload")
	}
}

func TestStopDockerService(t *testing.T) {
	config := tasks.ManageConfig{ContainerName: "my-app-abc12345"}
	task := tasks.StopDockerService(config)

	testutil.AssertTask(t, task).
		HasName("Stop Docker Service").
		ScriptContainsAll(
			"docker stop my-app-abc12345",
			"Container stopped",
		)
}

func TestStartDockerService(t *testing.T) {
	config := tasks.ManageConfig{ContainerName: "my-app-abc12345"}
	task := tasks.StartDockerService(config)

	testutil.AssertTask(t, task).
		HasName("Start Docker Service").
		ScriptContainsAll(
			"docker start my-app-abc12345",
			"Container started",
		)
}

func TestRestartDockerService(t *testing.T) {
	config := tasks.ManageConfig{ContainerName: "my-app-abc12345"}
	task := tasks.RestartDockerService(config)

	testutil.AssertTask(t, task).
		HasName("Restart Docker Service").
		ScriptContainsAll(
			"docker restart my-app-abc12345",
			"Container restarted",
		)
}

func TestUpdateTraefik(t *testing.T) {
	config := tasks.UpdateTraefikConfig{
		ContainerName: "my-app-abc12345",
		YAMLContent:   "http:\n  routers: {}\n",
	}
	task := tasks.UpdateTraefik(config)

	testutil.AssertTask(t, task).
		HasName("Update Traefik Config").
		ScriptContainsAll(
			"mkdir -p /etc/launch/traefik/dynamic",
			"/etc/launch/traefik/dynamic/my-app-abc12345.yml",
			"http:\n  routers: {}\n",
		)
}

func TestRemoveTraefikConfig(t *testing.T) {
	config := tasks.ManageConfig{ContainerName: "my-app-abc12345"}
	task := tasks.RemoveTraefikConfig(config)

	testutil.AssertTask(t, task).
		HasName("Remove Traefik Config").
		ScriptContainsAll(
			"rm -f /etc/launch/traefik/dynamic/my-app-abc12345.yml",
			"Traefik config removed",
		)
}
