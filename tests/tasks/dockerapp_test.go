package tasks_test

import (
	"testing"

	dockerapptasks "github.com/kkz6/launch-go/internal/modules/dockerapp/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

// ----- Deploy -----

func TestDockerApp_Deploy_Name(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "ghcr.io/acme/web:1.0.0",
		RestartPolicy: "unless-stopped",
	})
	testutil.AssertTask(t, task).HasName("Deploy myapp")
}

func TestDockerApp_Deploy_PullsAndRunsOnLaunchNetwork(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "ghcr.io/acme/web:1.0.0",
		RestartPolicy: "unless-stopped",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker pull",
		"ghcr.io/acme/web:1.0.0",
		"docker rm -f",
		"launch-app-myapp",
		"docker run -d",
		"--network launch-network",
		"--restart",
		"unless-stopped",
	)
}

func TestDockerApp_Deploy_NoRegistryLoginWhenPublic(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "nginx:latest",
		RestartPolicy: "unless-stopped",
	})
	testutil.AssertTask(t, task).ScriptNotContains("docker login")
}

func TestDockerApp_Deploy_RegistryLoginWhenSet(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:          "myapp",
		Container:        "launch-app-myapp",
		ImageRef:         "ghcr.io/acme/web:1.0.0",
		RestartPolicy:    "unless-stopped",
		RegistryURL:      "ghcr.io",
		RegistryUsername: "acme-bot",
		RegistryPassword: "ghp_super_secret",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker login",
		"ghcr.io",
		"--username",
		"acme-bot",
		"--password-stdin",
	)
}

func TestDockerApp_Deploy_EnvVarsRendered(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "nginx:latest",
		RestartPolicy: "unless-stopped",
		EnvVars: []dockerapptasks.EnvVar{
			{Key: "DATABASE_URL", Value: "postgres://x"},
			{Key: "API_KEY", Value: "abc123"},
		},
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		`-e "DATABASE_URL=postgres://x"`,
		`-e "API_KEY=abc123"`,
	)
}

func TestDockerApp_Deploy_VolumesCreatedAndMounted(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "nginx:latest",
		RestartPolicy: "unless-stopped",
		Volumes: []dockerapptasks.Volume{
			{HostName: "launch-app-myapp-data", MountPath: "/var/data"},
		},
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker volume create",
		"launch-app-myapp-data",
		`-v "launch-app-myapp-data:/var/data"`,
	)
}

func TestDockerApp_Deploy_PortsMapped(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "nginx:latest",
		RestartPolicy: "unless-stopped",
		Ports: []dockerapptasks.Port{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
	})
	testutil.AssertTask(t, task).ScriptContains(`-p "8080:80/tcp"`)
}

func TestDockerApp_Deploy_TraefikLabelsAppended(t *testing.T) {
	task := dockerapptasks.Deploy(dockerapptasks.DeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "nginx:latest",
		RestartPolicy: "unless-stopped",
		Labels: []string{
			"traefik.enable=true",
			"traefik.http.routers.myapp.rule=Host(`api.example.com`)",
		},
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		`--label "traefik.enable=true"`,
		"traefik.http.routers.myapp.rule",
		"api.example.com",
	)
}

// ----- Uninstall -----

func TestDockerApp_Uninstall_Name(t *testing.T) {
	task := dockerapptasks.Uninstall(dockerapptasks.UninstallOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
	})
	testutil.AssertTask(t, task).HasName("Uninstall myapp")
}

func TestDockerApp_Uninstall_KeepsVolumesByDefault(t *testing.T) {
	task := dockerapptasks.Uninstall(dockerapptasks.UninstallOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
		Volumes: []dockerapptasks.Volume{
			{HostName: "launch-app-myapp-data", MountPath: "/var/data"},
		},
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker rm -f").
		ScriptNotContains("docker volume rm")
}

func TestDockerApp_Uninstall_RemovesVolumesWhenAsked(t *testing.T) {
	task := dockerapptasks.Uninstall(dockerapptasks.UninstallOptions{
		AppName:    "myapp",
		Container:  "launch-app-myapp",
		RemoveData: true,
		Volumes: []dockerapptasks.Volume{
			{HostName: "launch-app-myapp-data", MountPath: "/var/data"},
		},
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker rm -f",
		"docker volume rm",
		"launch-app-myapp-data",
	)
}

// ----- Lifecycle -----

func TestDockerApp_Lifecycle_Start(t *testing.T) {
	task := dockerapptasks.Lifecycle(dockerapptasks.LifecycleOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
		Action:    dockerapptasks.LifecycleStart,
	})
	testutil.AssertTask(t, task).
		HasName("Start myapp").
		ScriptContains("docker start")
}

func TestDockerApp_Lifecycle_Stop(t *testing.T) {
	task := dockerapptasks.Lifecycle(dockerapptasks.LifecycleOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
		Action:    dockerapptasks.LifecycleStop,
	})
	testutil.AssertTask(t, task).
		HasName("Stop myapp").
		ScriptContains("docker stop")
}

func TestDockerApp_Lifecycle_Restart(t *testing.T) {
	task := dockerapptasks.Lifecycle(dockerapptasks.LifecycleOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
		Action:    dockerapptasks.LifecycleRestart,
	})
	testutil.AssertTask(t, task).
		HasName("Restart myapp").
		ScriptContains("docker restart")
}

// ----- Logs -----

func TestDockerApp_Logs_TailsContainer(t *testing.T) {
	task := dockerapptasks.Logs(dockerapptasks.LogsOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
		Tail:      150,
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker logs --tail 150").
		ScriptContains("launch-app-myapp")
}

func TestDockerApp_Logs_DefaultsTailTo100(t *testing.T) {
	task := dockerapptasks.Logs(dockerapptasks.LogsOptions{
		AppName:   "myapp",
		Container: "launch-app-myapp",
	})
	testutil.AssertTask(t, task).ScriptContains("docker logs --tail 100")
}
