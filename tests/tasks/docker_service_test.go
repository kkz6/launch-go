package tasks_test

import (
	"testing"

	dstasks "github.com/kkz6/launch-go/internal/modules/dockerservice/tasks"
	dstypes "github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	"github.com/kkz6/launch-go/tests/testutil"
)

// ----- Install: Postgres -----

func TestDockerService_InstallPostgres_Name(t *testing.T) {
	task := dstasks.Install(dstasks.InstallOptions{
		Kind:         dstypes.KindPostgres,
		Image:        "postgres:16",
		Container:    "launch-postgres",
		Volume:       "launch-postgres-data",
		Username:     "launch",
		Password:     "secret123",
		DatabaseName: "appdb",
	})
	testutil.AssertTask(t, task).HasName("Install PostgreSQL")
}

func TestDockerService_InstallPostgres_RunsContainerOnLaunchNetwork(t *testing.T) {
	task := dstasks.Install(dstasks.InstallOptions{
		Kind:         dstypes.KindPostgres,
		Image:        "postgres:16",
		Container:    "launch-postgres",
		Volume:       "launch-postgres-data",
		Username:     "launch",
		Password:     "secret123",
		DatabaseName: "appdb",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker pull postgres:16",
		"docker volume create launch-postgres-data",
		"docker rm -f launch-postgres",
		"docker run -d",
		"--name launch-postgres",
		"--network launch-network",
		"--restart unless-stopped",
		"POSTGRES_USER=launch",
		"POSTGRES_PASSWORD=secret123",
		"POSTGRES_DB=appdb",
		"launch-postgres-data:/var/lib/postgresql/data",
	)
}

// ----- Install: MySQL -----

func TestDockerService_InstallMySQL_Name(t *testing.T) {
	task := dstasks.Install(dstasks.InstallOptions{
		Kind:         dstypes.KindMySQL,
		Image:        "mysql:8.0",
		Container:    "launch-mysql",
		Volume:       "launch-mysql-data",
		Username:     "appuser",
		Password:     "secret",
		DatabaseName: "appdb",
	})
	testutil.AssertTask(t, task).HasName("Install MySQL")
}

func TestDockerService_InstallMySQL_RunsContainer(t *testing.T) {
	task := dstasks.Install(dstasks.InstallOptions{
		Kind:         dstypes.KindMySQL,
		Image:        "mysql:8.0",
		Container:    "launch-mysql",
		Volume:       "launch-mysql-data",
		Username:     "appuser",
		Password:     "secret",
		DatabaseName: "appdb",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker pull mysql:8.0",
		"docker volume create launch-mysql-data",
		"docker rm -f launch-mysql",
		"--name launch-mysql",
		"--network launch-network",
		"MYSQL_ROOT_PASSWORD=secret",
		"MYSQL_DATABASE=appdb",
		"MYSQL_USER=appuser",
		"MYSQL_PASSWORD=secret",
		"launch-mysql-data:/var/lib/mysql",
	)
}

// ----- Install: Redis -----

func TestDockerService_InstallRedis_Name(t *testing.T) {
	task := dstasks.Install(dstasks.InstallOptions{
		Kind:      dstypes.KindRedis,
		Image:     "redis:7-alpine",
		Container: "launch-redis",
		Volume:    "launch-redis-data",
		Password:  "redispass",
	})
	testutil.AssertTask(t, task).HasName("Install Redis")
}

func TestDockerService_InstallRedis_RunsContainerWithAuth(t *testing.T) {
	task := dstasks.Install(dstasks.InstallOptions{
		Kind:      dstypes.KindRedis,
		Image:     "redis:7-alpine",
		Container: "launch-redis",
		Volume:    "launch-redis-data",
		Password:  "redispass",
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker pull redis:7-alpine",
		"--name launch-redis",
		"--network launch-network",
		"launch-redis-data:/data",
		"redis-server --requirepass redispass --appendonly yes",
	)
}

// ----- Uninstall -----

func TestDockerService_Uninstall_Name(t *testing.T) {
	task := dstasks.Uninstall(dstasks.UninstallOptions{
		Kind:      dstypes.KindPostgres,
		Container: "launch-postgres",
		Volume:    "launch-postgres-data",
	})
	testutil.AssertTask(t, task).HasName("Uninstall PostgreSQL")
}

func TestDockerService_Uninstall_KeepsVolumeByDefault(t *testing.T) {
	task := dstasks.Uninstall(dstasks.UninstallOptions{
		Kind:      dstypes.KindPostgres,
		Container: "launch-postgres",
		Volume:    "launch-postgres-data",
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker rm -f launch-postgres").
		ScriptNotContains("docker volume rm")
}

func TestDockerService_Uninstall_RemovesVolumeWhenAsked(t *testing.T) {
	task := dstasks.Uninstall(dstasks.UninstallOptions{
		Kind:       dstypes.KindPostgres,
		Container:  "launch-postgres",
		Volume:     "launch-postgres-data",
		RemoveData: true,
	})
	testutil.AssertTask(t, task).ScriptContainsAll(
		"docker rm -f launch-postgres",
		"docker volume rm launch-postgres-data",
	)
}

// ----- Lifecycle -----

func TestDockerService_Lifecycle_Start(t *testing.T) {
	task := dstasks.Lifecycle(dstasks.LifecycleOptions{
		Kind:      dstypes.KindRedis,
		Container: "launch-redis",
		Action:    dstasks.LifecycleStart,
	})
	testutil.AssertTask(t, task).
		HasName("Start Redis").
		ScriptContains("docker start launch-redis")
}

func TestDockerService_Lifecycle_Stop(t *testing.T) {
	task := dstasks.Lifecycle(dstasks.LifecycleOptions{
		Kind:      dstypes.KindMySQL,
		Container: "launch-mysql",
		Action:    dstasks.LifecycleStop,
	})
	testutil.AssertTask(t, task).
		HasName("Stop MySQL").
		ScriptContains("docker stop launch-mysql")
}

func TestDockerService_Lifecycle_Restart(t *testing.T) {
	task := dstasks.Lifecycle(dstasks.LifecycleOptions{
		Kind:      dstypes.KindPostgres,
		Container: "launch-postgres",
		Action:    dstasks.LifecycleRestart,
	})
	testutil.AssertTask(t, task).
		HasName("Restart PostgreSQL").
		ScriptContains("docker restart launch-postgres")
}

// ----- Logs -----

func TestDockerService_Logs_Name(t *testing.T) {
	task := dstasks.Logs(dstasks.LogsOptions{
		Kind:      dstypes.KindPostgres,
		Container: "launch-postgres",
		Tail:      200,
	})
	testutil.AssertTask(t, task).HasName("Tail PostgreSQL logs")
}

func TestDockerService_Logs_TailsContainer(t *testing.T) {
	task := dstasks.Logs(dstasks.LogsOptions{
		Kind:      dstypes.KindPostgres,
		Container: "launch-postgres",
		Tail:      200,
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker logs --tail 200").
		ScriptContains("launch-postgres")
}

func TestDockerService_Logs_DefaultsTailTo100(t *testing.T) {
	task := dstasks.Logs(dstasks.LogsOptions{
		Kind:      dstypes.KindRedis,
		Container: "launch-redis",
	})
	testutil.AssertTask(t, task).ScriptContains("docker logs --tail 100")
}
