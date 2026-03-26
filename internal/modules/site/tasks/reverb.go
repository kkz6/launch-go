package tasks

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ConfigureReverbEnv creates a task to set Reverb .env variables on the server
func ConfigureReverbEnv(appDirectory, siteAddress string, port int) *taskrunner.BaseTask {
	appID := randomDigits(6)
	appKey := randomAlphanumeric(20)
	appSecret := randomAlphanumeric(40)

	script := fmt.Sprintf(`#!/bin/bash
set -e

ENV_FILE="%s/.env"

# Resolve symlinks so sed -i modifies the actual file (e.g. shared/.env)
if [ -L "$ENV_FILE" ]; then
    ENV_FILE="$(readlink -f "$ENV_FILE")"
fi

if [ ! -f "$ENV_FILE" ]; then
    echo ".env file not found at $ENV_FILE"
    exit 1
fi

# Helper function to set or add an env variable
set_env() {
    local key="$1" val="$2"
    if grep -q "^${key}=" "$ENV_FILE"; then
        sed -i "s|^${key}=.*|${key}=${val}|" "$ENV_FILE"
    else
        echo "${key}=${val}" >> "$ENV_FILE"
    fi
}

set_env "BROADCAST_CONNECTION" "reverb"
set_env "REVERB_APP_ID" "%s"
set_env "REVERB_APP_KEY" "%s"
set_env "REVERB_APP_SECRET" "%s"
set_env "REVERB_HOST" "%s"
set_env "REVERB_PORT" "%d"
set_env "REVERB_SCHEME" "https"

echo "Reverb .env variables configured successfully"
`, appDirectory, appID, appKey, appSecret, siteAddress, port)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Configure Reverb Environment"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// randomDigits generates a random string of n digits
func randomDigits(n int) string {
	const digits = "0123456789"
	result := make([]byte, n)

	// Ensure the first digit is non-zero
	first, _ := rand.Int(rand.Reader, big.NewInt(9))
	result[0] = digits[first.Int64()+1]

	for i := 1; i < n; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		result[i] = digits[idx.Int64()]
	}

	return string(result)
}

// randomAlphanumeric generates a random alphanumeric string of length n
func randomAlphanumeric(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)

	for i := 0; i < n; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[idx.Int64()]
	}

	return string(result)
}
