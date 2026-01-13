package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestUploadDaemon_GeneratesCorrectScript(t *testing.T) {
	config := tasks.UploadDaemonConfig{
		Path: "/etc/supervisor/conf.d/queue-worker.conf",
		Contents: `[program:queue-worker]
command=php /home/launcher/site/artisan queue:work
user=launcher
autostart=true
autorestart=true`,
		LogPath:      "/home/launcher/.launch/daemon-queue.log",
		ErrorLogPath: "/home/launcher/.launch/daemon-queue-error.log",
		User:         "launcher",
	}

	task := tasks.UploadDaemon(config)

	testutil.AssertTask(t, task).
		HasName("Upload Daemon Config").
		ScriptContains(config.Path).
		ScriptContains("[program:queue-worker]").
		ScriptContains(config.LogPath).
		ScriptContains(config.ErrorLogPath).
		ScriptContains("chmod 644").
		ScriptContains("chown launcher:launcher").
		ScriptMatches("tasks/daemon_upload")
}

func TestDeleteDaemon_GeneratesCorrectScript(t *testing.T) {
	config := tasks.DeleteDaemonConfig{
		Path:        "/etc/supervisor/conf.d/queue-worker.conf",
		ProgramName: "queue-worker",
	}

	task := tasks.DeleteDaemon(config)

	testutil.AssertTask(t, task).
		HasName("Delete Daemon").
		ScriptContains("supervisorctl stop").
		ScriptContains(config.ProgramName).
		ScriptContains(config.Path).
		ScriptContains("rm -f").
		ScriptContains("supervisorctl reread").
		ScriptContains("supervisorctl update").
		ScriptMatches("tasks/daemon_delete")
}

func TestRestartDaemon_GeneratesCorrectScript(t *testing.T) {
	config := tasks.RestartDaemonConfig{
		ProgramName: "queue-worker",
	}

	task := tasks.RestartDaemon(config)

	testutil.AssertTask(t, task).
		HasName("Restart Daemon").
		ScriptContains("supervisorctl restart").
		ScriptContains(config.ProgramName).
		ScriptMatches("tasks/daemon_restart")
}

func TestReloadSupervisor_GeneratesCorrectScript(t *testing.T) {
	task := tasks.ReloadSupervisor()

	testutil.AssertTask(t, task).
		HasName("Reload Supervisor").
		ScriptContains("supervisorctl reread").
		ScriptContains("supervisorctl update").
		ScriptMatches("tasks/daemon_reload_supervisor")
}
