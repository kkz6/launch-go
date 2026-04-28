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

// ----- Compose: Deploy -----

func TestDockerApp_ComposeDeploy_WritesProjectAndUps(t *testing.T) {
	task := dockerapptasks.ComposeDeploy(dockerapptasks.ComposeDeployOptions{
		AppName:     "myapp",
		Project:     "launch-app-myapp",
		ComposeYAML: "version: '3'\nservices:\n  web:\n    image: nginx",
		ComposeEnv:  "FOO=bar\n",
	})
	testutil.AssertTask(t, task).
		HasName("Deploy myapp (compose)").
		ScriptContainsAll(
			"/opt/launch/apps/myapp",
			"docker-compose.yml",
			".env",
			"docker compose -p \"launch-app-myapp\" pull",
			"docker compose -p \"launch-app-myapp\" up -d",
			"--remove-orphans",
		)
}

func TestDockerApp_ComposeDeploy_LoginsWhenRegistrySet(t *testing.T) {
	task := dockerapptasks.ComposeDeploy(dockerapptasks.ComposeDeployOptions{
		AppName:          "myapp",
		Project:          "launch-app-myapp",
		ComposeYAML:      "version: '3'",
		RegistryURL:      "ghcr.io",
		RegistryUsername: "acme",
		RegistryPassword: "s3cret",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker login \"ghcr.io\"",
		"--username \"acme\"",
		"--password-stdin",
	)
}

// ----- Compose: Uninstall -----

func TestDockerApp_ComposeUninstall_DownsAndRemovesDir(t *testing.T) {
	task := dockerapptasks.ComposeUninstall(dockerapptasks.ComposeUninstallOptions{
		AppName: "myapp",
		Project: "launch-app-myapp",
	})
	testutil.AssertTask(t, task).
		HasName("Uninstall myapp (compose)").
		ScriptContainsAll(
			"docker compose -p \"launch-app-myapp\" down",
			"--remove-orphans",
			"rm -rf",
		)
}

func TestDockerApp_ComposeUninstall_RemoveDataDropsVolumes(t *testing.T) {
	task := dockerapptasks.ComposeUninstall(dockerapptasks.ComposeUninstallOptions{
		AppName:    "myapp",
		Project:    "launch-app-myapp",
		RemoveData: true,
	})
	testutil.AssertTask(t, task).ScriptContains("down -v")
}

// ----- Compose: Lifecycle -----

func TestDockerApp_ComposeLifecycle_Start(t *testing.T) {
	task := dockerapptasks.ComposeLifecycle(dockerapptasks.ComposeLifecycleOptions{
		AppName: "myapp",
		Project: "launch-app-myapp",
		Action:  dockerapptasks.LifecycleStart,
	})
	testutil.AssertTask(t, task).
		HasName("Start myapp (compose)").
		ScriptContains("docker compose -p \"launch-app-myapp\" start")
}

// ----- Compose: Logs -----

func TestDockerApp_ComposeLogs_TailsCompose(t *testing.T) {
	task := dockerapptasks.ComposeLogs(dockerapptasks.ComposeLogsOptions{
		AppName: "myapp",
		Project: "launch-app-myapp",
		Tail:    200,
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker compose -p \"launch-app-myapp\" logs --tail \"200\"")
}

// ----- Git: Deploy -----

func TestDockerApp_GitDeploy_ClonesAndBuilds(t *testing.T) {
	task := dockerapptasks.GitDeploy(dockerapptasks.GitDeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "launch-app-myapp:latest",
		RestartPolicy: "unless-stopped",
		RepoURL:       "https://github.com/acme/web.git",
		Branch:        "main",
	})
	testutil.AssertTask(t, task).
		HasName("Deploy myapp (git)").
		ScriptContainsAll(
			"git clone --depth 1 --branch \"main\"",
			"https://github.com/acme/web.git",
			"docker build",
			"-t \"launch-app-myapp:latest\"",
			"-f \"Dockerfile\"",
			"docker run -d",
			"--network launch-network",
		)
}

func TestDockerApp_GitDeploy_InjectsTokenForPrivateRepo(t *testing.T) {
	task := dockerapptasks.GitDeploy(dockerapptasks.GitDeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "launch-app-myapp:latest",
		RestartPolicy: "unless-stopped",
		RepoURL:       "https://github.com/acme/private.git",
		Branch:        "main",
		GitToken:      "ghp_secret",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"x-access-token:ghp_secret@",
		"git remote set-url origin 'https://github.com/acme/private.git'",
	)
}

func TestDockerApp_GitDeploy_DefaultsDockerfileAndContext(t *testing.T) {
	task := dockerapptasks.GitDeploy(dockerapptasks.GitDeployOptions{
		AppName:       "myapp",
		Container:     "launch-app-myapp",
		ImageRef:      "launch-app-myapp:latest",
		RestartPolicy: "unless-stopped",
		RepoURL:       "https://github.com/acme/web.git",
		Branch:        "main",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"-f \"Dockerfile\"",
		"\".\"",
	)
}
