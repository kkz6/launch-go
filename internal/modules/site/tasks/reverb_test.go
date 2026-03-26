package tasks

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigureReverbEnv_TaskProperties(t *testing.T) {
	task := ConfigureReverbEnv("/home/launcher/example.com/current", "example.com", 6001)

	assert.Equal(t, "Configure Reverb Environment", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())
}

func TestConfigureReverbEnv_ScriptContainsEnvFile(t *testing.T) {
	task := ConfigureReverbEnv("/home/launcher/example.com/current", "example.com", 6001)
	script := task.Script()

	assert.Contains(t, script, `ENV_FILE="/home/launcher/example.com/current/.env"`)
	assert.Contains(t, script, `readlink -f "$ENV_FILE"`)
	assert.Contains(t, script, `if [ ! -f "$ENV_FILE" ]`)
	assert.Contains(t, script, "exit 1")
}

func TestConfigureReverbEnv_ScriptSetsEnvVariables(t *testing.T) {
	task := ConfigureReverbEnv("/home/launcher/site.com/current", "site.com", 6005)
	script := task.Script()

	assert.Contains(t, script, `set_env "BROADCAST_CONNECTION" "reverb"`)
	assert.Contains(t, script, `set_env "REVERB_HOST" "site.com"`)
	assert.Contains(t, script, `set_env "REVERB_PORT" "6005"`)
	assert.Contains(t, script, `set_env "REVERB_SCHEME" "https"`)
	assert.Contains(t, script, `set_env "REVERB_APP_ID"`)
	assert.Contains(t, script, `set_env "REVERB_APP_KEY"`)
	assert.Contains(t, script, `set_env "REVERB_APP_SECRET"`)
}

func TestConfigureReverbEnv_ScriptHasSetEnvHelper(t *testing.T) {
	task := ConfigureReverbEnv("/app/current", "app.com", 6001)
	script := task.Script()

	assert.Contains(t, script, "set_env() {")
	assert.Contains(t, script, `grep -q "^${key}=" "$ENV_FILE"`)
	assert.Contains(t, script, "sed -i")
	assert.Contains(t, script, `echo "${key}=${val}" >> "$ENV_FILE"`)
}

func TestConfigureReverbEnv_ScriptStartsWithShebang(t *testing.T) {
	task := ConfigureReverbEnv("/app/current", "app.com", 6001)
	script := task.Script()

	assert.True(t, strings.HasPrefix(script, "#!/bin/bash\nset -e"))
}

func TestConfigureReverbEnv_DifferentPorts(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port_6001", 6001},
		{"port_6042", 6042},
		{"port_6500", 6500},
		{"port_6999", 6999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ConfigureReverbEnv("/app/current", "app.com", tt.port)
			script := task.Script()

			expected := fmt.Sprintf(`set_env "REVERB_PORT" "%d"`, tt.port)
			assert.Contains(t, script, expected)
		})
	}
}

func TestConfigureReverbEnv_DifferentAppDirectories(t *testing.T) {
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
			task := ConfigureReverbEnv(tt.dir, "site.com", 6001)
			script := task.Script()

			assert.Contains(t, script, `ENV_FILE="`+tt.dir+`/.env"`)
		})
	}
}

func TestConfigureReverbEnv_DifferentAddresses(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"bare_domain", "example.com"},
		{"subdomain", "app.example.com"},
		{"www", "www.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ConfigureReverbEnv("/app/current", tt.address, 6001)
			script := task.Script()

			assert.Contains(t, script, `set_env "REVERB_HOST" "`+tt.address+`"`)
		})
	}
}

func TestConfigureReverbEnv_GeneratesRandomValues(t *testing.T) {
	task1 := ConfigureReverbEnv("/app/current", "app.com", 6001)
	task2 := ConfigureReverbEnv("/app/current", "app.com", 6001)

	// Two calls should generate different random values
	// (extremely unlikely to collide with 20+ char random strings)
	assert.NotEqual(t, task1.Script(), task2.Script())
}

func TestConfigureReverbEnv_SuccessMessage(t *testing.T) {
	task := ConfigureReverbEnv("/app/current", "app.com", 6001)
	script := task.Script()

	assert.Contains(t, script, "Reverb .env variables configured successfully")
}

func TestRandomDigits(t *testing.T) {
	result := randomDigits(6)

	assert.Len(t, result, 6)

	// First digit should not be zero
	assert.NotEqual(t, byte('0'), result[0])

	// All characters should be digits
	for _, c := range result {
		assert.True(t, c >= '0' && c <= '9', "expected digit, got %c", c)
	}
}

func TestRandomDigits_DifferentCalls(t *testing.T) {
	r1 := randomDigits(6)
	r2 := randomDigits(6)

	// Extremely unlikely to collide
	assert.NotEqual(t, r1, r2)
}

func TestRandomAlphanumeric(t *testing.T) {
	result := randomAlphanumeric(20)

	assert.Len(t, result, 20)

	// All characters should be alphanumeric
	for _, c := range result {
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		assert.True(t, isLower || isUpper || isDigit, "expected alphanumeric, got %c", c)
	}
}

func TestRandomAlphanumeric_DifferentLengths(t *testing.T) {
	tests := []int{1, 10, 20, 40}
	for _, length := range tests {
		result := randomAlphanumeric(length)
		assert.Len(t, result, length)
	}
}

func TestRandomAlphanumeric_DifferentCalls(t *testing.T) {
	r1 := randomAlphanumeric(40)
	r2 := randomAlphanumeric(40)

	assert.NotEqual(t, r1, r2)
}
