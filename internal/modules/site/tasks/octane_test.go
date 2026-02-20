package tasks

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDetectOctaneServer_TaskProperties(t *testing.T) {
	task := DetectOctaneServer("/home/launcher/example.com/current", "/usr/bin/php8.3")

	assert.Equal(t, "Detect Octane Server", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "cd /home/launcher/example.com/current")
	assert.Contains(t, script, "/usr/bin/php8.3 artisan tinker")
	assert.Contains(t, script, "config('octane.server')")
	assert.Contains(t, script, `|| echo "frankenphp"`)
}

func TestDetectOctaneServer_DifferentPhpVersions(t *testing.T) {
	tests := []struct {
		name      string
		phpBinary string
	}{
		{"php80", "/usr/bin/php8.0"},
		{"php81", "/usr/bin/php8.1"},
		{"php82", "/usr/bin/php8.2"},
		{"php83", "/usr/bin/php8.3"},
		{"php84", "/usr/bin/php8.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := DetectOctaneServer("/app/current", tt.phpBinary)
			assert.Contains(t, task.Script(), tt.phpBinary+" artisan tinker")
		})
	}
}

func TestDetectOctaneServer_DifferentAppDirectories(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"zero_downtime", "/home/launcher/site.com/current"},
		{"repository", "/home/launcher/site.com/repository"},
		{"custom_user", "/home/myuser/app.example.com/current"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := DetectOctaneServer(tt.dir, "php")
			assert.Contains(t, task.Script(), "cd "+tt.dir)
		})
	}
}

func TestInstallFrankenPhpBinary_TaskProperties(t *testing.T) {
	task := InstallFrankenPhpBinary()

	assert.Equal(t, "Install FrankenPHP Binary", task.Name())
	assert.Equal(t, 300*time.Second, task.Timeout())

	script := task.Script()
	assert.True(t, strings.HasPrefix(script, "#!/bin/bash\nset -e"))
	assert.Contains(t, script, "dunglas/frankenphp")
	assert.Contains(t, script, "/usr/local/bin/frankenphp")
	assert.Contains(t, script, "chmod +x")
	assert.Contains(t, script, "amd64")
	assert.Contains(t, script, "arm64")
}

func TestInstallFrankenPhpBinary_ArchitectureDetection(t *testing.T) {
	task := InstallFrankenPhpBinary()
	script := task.Script()

	assert.Contains(t, script, `x86_64|amd64) ARCH="amd64"`)
	assert.Contains(t, script, `aarch64|arm64) ARCH="arm64"`)
	assert.Contains(t, script, `"Unsupported architecture: $ARCH"`)
}

func TestInstallFrankenPhpBinary_FailsOnMissingURL(t *testing.T) {
	task := InstallFrankenPhpBinary()
	script := task.Script()

	assert.Contains(t, script, `if [ -z "$DOWNLOAD_URL" ]`)
	assert.Contains(t, script, "Failed to find FrankenPHP download URL")
	assert.Contains(t, script, "exit 1")
}

func TestInstallRoadRunnerBinary_TaskProperties(t *testing.T) {
	task := InstallRoadRunnerBinary()

	assert.Equal(t, "Install RoadRunner Binary", task.Name())
	assert.Equal(t, 300*time.Second, task.Timeout())

	script := task.Script()
	assert.True(t, strings.HasPrefix(script, "#!/bin/bash\nset -e"))
	assert.Contains(t, script, "roadrunner-server/roadrunner")
	assert.Contains(t, script, "/usr/local/bin/rr")
	assert.Contains(t, script, "chmod +x")
	assert.Contains(t, script, "tar -xz")
}

func TestInstallRoadRunnerBinary_ArchitectureDetection(t *testing.T) {
	task := InstallRoadRunnerBinary()
	script := task.Script()

	assert.Contains(t, script, `x86_64|amd64) ARCH="amd64"`)
	assert.Contains(t, script, `aarch64|arm64) ARCH="arm64"`)
}

func TestInstallRoadRunnerBinary_CleansUpTmpDir(t *testing.T) {
	task := InstallRoadRunnerBinary()
	script := task.Script()

	assert.Contains(t, script, "TMP_DIR=$(mktemp -d)")
	assert.Contains(t, script, "rm -rf \"$TMP_DIR\"")
}

func TestInstallRoadRunnerBinary_FailsOnMissingURL(t *testing.T) {
	task := InstallRoadRunnerBinary()
	script := task.Script()

	assert.Contains(t, script, `if [ -z "$DOWNLOAD_URL" ]`)
	assert.Contains(t, script, "Failed to find RoadRunner download URL")
	assert.Contains(t, script, "exit 1")
}
