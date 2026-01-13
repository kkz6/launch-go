package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestUploadCron_GeneratesCorrectScript(t *testing.T) {
	config := tasks.UploadCronConfig{
		Path:     "/etc/cron.d/backup-job",
		Contents: "0 2 * * * launcher /home/launcher/backup.sh >> /home/launcher/.launch/cron-backup.log 2>&1",
		LogPath:  "/home/launcher/.launch/cron-backup.log",
		User:     "launcher",
	}

	task := tasks.UploadCron(config)

	testutil.AssertTask(t, task).
		HasName("Upload Cron File").
		ScriptContains(config.Path).
		ScriptContains(config.Contents).
		ScriptContains(config.LogPath).
		ScriptContains("chmod 644").
		ScriptContains("mkdir -p").
		ScriptContains("chown launcher:launcher").
		ScriptMatches("tasks/cron_upload")
}

func TestUploadCron_DifferentUser(t *testing.T) {
	config := tasks.UploadCronConfig{
		Path:     "/etc/cron.d/custom-job",
		Contents: "*/5 * * * * deploy /opt/deploy.sh",
		LogPath:  "/home/deploy/.launch/cron.log",
		User:     "deploy",
	}

	task := tasks.UploadCron(config)

	testutil.AssertTask(t, task).
		HasName("Upload Cron File").
		ScriptContains("chown deploy:deploy")
}

func TestDeleteCron_GeneratesCorrectScript(t *testing.T) {
	config := tasks.DeleteCronConfig{
		Path: "/etc/cron.d/backup-job",
	}

	task := tasks.DeleteCron(config)

	testutil.AssertTask(t, task).
		HasName("Delete Cron File").
		ScriptContains(config.Path).
		ScriptContains("rm -f").
		ScriptContains("if [ -f").
		ScriptMatches("tasks/cron_delete")
}
