package status

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetSystemdServiceName_Docker is the regression guard for the
// "Docker shows Unknown" bug: docker is a real systemd unit, so the
// status probe must map it to "docker". Before the fix this fell
// through to "" and the WS handler reported StateUnknown.
func TestGetSystemdServiceName_Docker(t *testing.T) {
	assert.Equal(t, "docker", GetSystemdServiceName("docker"))
}

// TestGetContainerName guards the Traefik "Unknown" bug: Traefik runs as
// a Docker container, so it must resolve to a container name (probed via
// docker inspect) rather than a systemd unit.
func TestGetContainerName(t *testing.T) {
	assert.Equal(t, "launch-traefik", GetContainerName("traefik"))
	// Non-container services must return "" so the handler uses systemctl.
	assert.Equal(t, "", GetContainerName("docker"))
	assert.Equal(t, "", GetContainerName("mysql"))
	assert.Equal(t, "", GetContainerName("php82"))
}

func TestParseContainerState(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		isActive bool
	}{
		{"running", StateRunning, true},
		{"RUNNING", StateRunning, true},
		{" running ", StateRunning, true},
		{"restarting", StateFailed, false},
		{"exited", StateStopped, false},
		{"created", StateStopped, false},
		{"paused", StateStopped, false},
		{"dead", StateStopped, false},
		{"", StateUnknown, false},
		{"garbage", StateUnknown, false},
	}
	for _, c := range cases {
		gotStatus, gotActive := ParseContainerState(c.in)
		assert.Equalf(t, c.want, gotStatus, "state %q", c.in)
		assert.Equalf(t, c.isActive, gotActive, "state %q active", c.in)
	}
}

func TestAgentVersionCommand(t *testing.T) {
	assert.Equal(t, "launch-agent --version 2>/dev/null", AgentVersionCommand("launch_agent"))
	assert.Equal(t, "", AgentVersionCommand("docker"))
	assert.Equal(t, "", AgentVersionCommand("mysql"))
}

func TestParseAgentVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"launch-agent version 1.4.2", "1.4.2"},
		{"launch-agent version v1.4.2", "1.4.2"},             // leading v stripped
		{"launch-agent version 1.4.2\nextra noise", "1.4.2"}, // first line only
		{"  launch-agent version 0.9.0  ", "0.9.0"},
		{"", ""},
		{"command not found", ""}, // no digit → rejected
		{"version", ""},           // no digit → rejected
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, ParseAgentVersion(c.in), "input %q", c.in)
	}
}

func TestServiceStatusJSONSerialization(t *testing.T) {
	t.Run("full service status", func(t *testing.T) {
		service := ServiceStatus{
			ID:       "service-123",
			Software: "nginx",
			Name:     "nginx.service",
			Status:   "running",
			IsActive: true,
			Memory:   "256 MB",
			Uptime:   "5d 12h 30m",
			PID:      1234,
		}

		data, err := json.Marshal(service)
		assert.NoError(t, err)

		var decoded ServiceStatus
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, service, decoded)
	})

	t.Run("omits empty optional fields", func(t *testing.T) {
		service := ServiceStatus{
			ID:       "service-456",
			Software: "php-fpm",
			Name:     "php8.2-fpm.service",
			Status:   "stopped",
			IsActive: false,
		}

		data, err := json.Marshal(service)
		assert.NoError(t, err)

		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "memory")
		assert.NotContains(t, jsonStr, "uptime")
		// PID with omitempty and zero value will be omitted
		assert.NotContains(t, jsonStr, "pid")
	})

	t.Run("deserialize from JSON", func(t *testing.T) {
		jsonData := `{
			"id": "svc-789",
			"software": "mysql",
			"name": "mysql.service",
			"status": "active",
			"is_active": true,
			"memory": "512 MB",
			"uptime": "3h 45m",
			"pid": 5678
		}`

		var service ServiceStatus
		err := json.Unmarshal([]byte(jsonData), &service)
		assert.NoError(t, err)

		assert.Equal(t, "svc-789", service.ID)
		assert.Equal(t, "mysql", service.Software)
		assert.Equal(t, "mysql.service", service.Name)
		assert.Equal(t, "active", service.Status)
		assert.True(t, service.IsActive)
		assert.Equal(t, "512 MB", service.Memory)
		assert.Equal(t, "3h 45m", service.Uptime)
		assert.Equal(t, 5678, service.PID)
	})
}
