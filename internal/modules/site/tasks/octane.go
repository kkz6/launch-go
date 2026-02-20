package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DetectOctaneServer creates a task to detect the configured Octane server type
func DetectOctaneServer(appDirectory, phpBinary string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`#!/bin/bash
cd %s && %s artisan tinker --execute="echo config('octane.server')" 2>/dev/null || echo "frankenphp"
`, appDirectory, phpBinary)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Detect Octane Server"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// InstallFrankenPhpBinary creates a task to install the FrankenPHP binary
func InstallFrankenPhpBinary() *taskrunner.BaseTask {
	script := `#!/bin/bash
set -e

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# Download latest FrankenPHP binary
DOWNLOAD_URL=$(curl -sL https://api.github.com/repos/dunglas/frankenphp/releases/latest \
    | grep "browser_download_url.*frankenphp-linux-${ARCH}\"" \
    | head -1 \
    | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Failed to find FrankenPHP download URL"
    exit 1
fi

curl -sL "$DOWNLOAD_URL" -o /usr/local/bin/frankenphp
chmod +x /usr/local/bin/frankenphp

echo "FrankenPHP installed successfully"
frankenphp version 2>/dev/null || true
`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install FrankenPHP Binary"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// InstallRoadRunnerBinary creates a task to install the RoadRunner binary
func InstallRoadRunnerBinary() *taskrunner.BaseTask {
	script := `#!/bin/bash
set -e

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# Download latest RoadRunner binary
DOWNLOAD_URL=$(curl -sL https://api.github.com/repos/roadrunner-server/roadrunner/releases/latest \
    | grep "browser_download_url.*roadrunner.*linux-${ARCH}.tar.gz\"" \
    | head -1 \
    | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Failed to find RoadRunner download URL"
    exit 1
fi

TMP_DIR=$(mktemp -d)
curl -sL "$DOWNLOAD_URL" | tar -xz -C "$TMP_DIR"
find "$TMP_DIR" -name "rr" -type f -exec mv {} /usr/local/bin/rr \;
chmod +x /usr/local/bin/rr
rm -rf "$TMP_DIR"

echo "RoadRunner installed successfully"
rr --version 2>/dev/null || true
`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install RoadRunner Binary"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}
