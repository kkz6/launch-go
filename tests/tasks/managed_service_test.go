package tasks_test

import (
	"testing"

	mstasks "github.com/kkz6/launch-go/internal/modules/managedservice/tasks"
	mstypes "github.com/kkz6/launch-go/internal/modules/managedservice/types"
	"github.com/kkz6/launch-go/tests/testutil"
)

// ----- Install: Postgres -----

func TestManagedService_InstallPostgres_Name(t *testing.T) {
	task := mstasks.Install(mstasks.InstallOptions{
		Kind:         mstypes.KindPostgres,
		Image:        "postgres:16",
		Container:    "launch-postgres",
		Volume:       "launch-postgres-data",
		Username:     "launch",
		Password:     "secret123",
		DatabaseName: "appdb",
	})
	testutil.AssertTask(t, task).HasName("Install PostgreSQL")
}

func TestManagedService_InstallPostgres_RunsContainerOnLaunchNetwork(t *testing.T) {
	task := mstasks.Install(mstasks.InstallOptions{
		Kind:         mstypes.KindPostgres,
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

func TestManagedService_InstallMySQL_Name(t *testing.T) {
	task := mstasks.Install(mstasks.InstallOptions{
		Kind:         mstypes.KindMySQL,
		Image:        "mysql:8.0",
		Container:    "launch-mysql",
		Volume:       "launch-mysql-data",
		Username:     "appuser",
		Password:     "secret",
		DatabaseName: "appdb",
	})
	testutil.AssertTask(t, task).HasName("Install MySQL")
}

func TestManagedService_InstallMySQL_RunsContainer(t *testing.T) {
	task := mstasks.Install(mstasks.InstallOptions{
		Kind:         mstypes.KindMySQL,
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

func TestManagedService_InstallRedis_Name(t *testing.T) {
	task := mstasks.Install(mstasks.InstallOptions{
		Kind:      mstypes.KindRedis,
		Image:     "redis:7-alpine",
		Container: "launch-redis",
		Volume:    "launch-redis-data",
		Password:  "redispass",
	})
	testutil.AssertTask(t, task).HasName("Install Redis")
}

func TestManagedService_InstallRedis_RunsContainerWithAuth(t *testing.T) {
	task := mstasks.Install(mstasks.InstallOptions{
		Kind:      mstypes.KindRedis,
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

func TestManagedService_Uninstall_Name(t *testing.T) {
	task := mstasks.Uninstall(mstasks.UninstallOptions{
		Kind:      mstypes.KindPostgres,
		Container: "launch-postgres",
		Volume:    "launch-postgres-data",
	})
	testutil.AssertTask(t, task).HasName("Uninstall PostgreSQL")
}

func TestManagedService_Uninstall_KeepsVolumeByDefault(t *testing.T) {
	task := mstasks.Uninstall(mstasks.UninstallOptions{
		Kind:      mstypes.KindPostgres,
		Container: "launch-postgres",
		Volume:    "launch-postgres-data",
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker rm -f launch-postgres").
		ScriptNotContains("docker volume rm")
}

func TestManagedService_Uninstall_RemovesVolumeWhenAsked(t *testing.T) {
	task := mstasks.Uninstall(mstasks.UninstallOptions{
		Kind:       mstypes.KindPostgres,
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

func TestManagedService_Lifecycle_Start(t *testing.T) {
	task := mstasks.Lifecycle(mstasks.LifecycleOptions{
		Kind:      mstypes.KindRedis,
		Container: "launch-redis",
		Action:    mstasks.LifecycleStart,
	})
	testutil.AssertTask(t, task).
		HasName("Start Redis").
		ScriptContains("docker start launch-redis")
}

func TestManagedService_Lifecycle_Stop(t *testing.T) {
	task := mstasks.Lifecycle(mstasks.LifecycleOptions{
		Kind:      mstypes.KindMySQL,
		Container: "launch-mysql",
		Action:    mstasks.LifecycleStop,
	})
	testutil.AssertTask(t, task).
		HasName("Stop MySQL").
		ScriptContains("docker stop launch-mysql")
}

func TestManagedService_Lifecycle_Restart(t *testing.T) {
	task := mstasks.Lifecycle(mstasks.LifecycleOptions{
		Kind:      mstypes.KindPostgres,
		Container: "launch-postgres",
		Action:    mstasks.LifecycleRestart,
	})
	testutil.AssertTask(t, task).
		HasName("Restart PostgreSQL").
		ScriptContains("docker restart launch-postgres")
}

// ----- Logs -----

func TestManagedService_Logs_Name(t *testing.T) {
	task := mstasks.Logs(mstasks.LogsOptions{
		Kind:      mstypes.KindPostgres,
		Container: "launch-postgres",
		Tail:      200,
	})
	testutil.AssertTask(t, task).HasName("Tail PostgreSQL logs")
}

func TestManagedService_Logs_TailsContainer(t *testing.T) {
	task := mstasks.Logs(mstasks.LogsOptions{
		Kind:      mstypes.KindPostgres,
		Container: "launch-postgres",
		Tail:      200,
	})
	testutil.AssertTask(t, task).
		ScriptContains("docker logs --tail 200").
		ScriptContains("launch-postgres")
}

func TestManagedService_Logs_DefaultsTailTo100(t *testing.T) {
	task := mstasks.Logs(mstasks.LogsOptions{
		Kind:      mstypes.KindRedis,
		Container: "launch-redis",
	})
	testutil.AssertTask(t, task).ScriptContains("docker logs --tail 100")
}
