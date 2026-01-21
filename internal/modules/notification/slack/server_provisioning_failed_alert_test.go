package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerProvisioningFailedAdminAlert(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{
		ID:   "server-123",
		Name: "my-server",
	}

	alert := NewServerProvisioningFailedAdminAlert(alerter, server)

	require.NotNil(t, alert)
	assert.Equal(t, server, alert.Server)
	assert.Nil(t, alert.User)
	assert.Empty(t, alert.Output)
	assert.Empty(t, alert.ErrorMessage)
}

func TestServerProvisioningFailedAdminAlert_WithUser(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}
	user := &UserInfo{Name: "John Doe", Email: "john@example.com"}

	alert := NewServerProvisioningFailedAdminAlert(alerter, server)
	result := alert.WithUser(user)

	assert.Same(t, alert, result)
	assert.Equal(t, user, alert.User)
}

func TestServerProvisioningFailedAdminAlert_WithOutput(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewServerProvisioningFailedAdminAlert(alerter, server)
	result := alert.WithOutput("task output")

	assert.Same(t, alert, result)
	assert.Equal(t, "task output", alert.Output)
}

func TestServerProvisioningFailedAdminAlert_WithErrorMessage(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewServerProvisioningFailedAdminAlert(alerter, server)
	result := alert.WithErrorMessage("connection timeout")

	assert.Same(t, alert, result)
	assert.Equal(t, "connection timeout", alert.ErrorMessage)
}

func TestServerProvisioningFailedAdminAlert_WithOutputRetrievalError(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewServerProvisioningFailedAdminAlert(alerter, server)
	result := alert.WithOutputRetrievalError("logs not available")

	assert.Same(t, alert, result)
	assert.Equal(t, "logs not available", alert.OutputRetrievalError)
}

func TestServerProvisioningFailedAdminAlert_Send(t *testing.T) {
	t.Run("sends alert successfully", func(t *testing.T) {
		var receivedPayload BlockKitMessage
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			assert.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		alerter := NewAdminAlerter(testServer.URL)
		server := ServerInfo{
			ID:         "server-123",
			Name:       "my-server",
			TeamID:     "team-456",
			TeamName:   "My Team",
			Provider:   "digitalocean",
			PublicIPv4: "192.168.1.100",
		}
		user := &UserInfo{Name: "John Doe", Email: "john@example.com"}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server).
			WithUser(user).
			WithOutput("apt-get failed").
			WithErrorMessage("connection timeout")

		err := alert.Send(context.Background())

		assert.NoError(t, err)
		assert.Contains(t, receivedPayload.Text, "my-server")

		// Verify blocks contain expected content
		hasHeader := false
		hasServerName := false
		hasOutput := false
		for _, block := range receivedPayload.Blocks {
			if block.Type == "header" && block.Text != nil {
				hasHeader = true
				assert.Contains(t, block.Text.Text, "Server Provisioning Failed")
			}
			if block.Type == "section" {
				for _, field := range block.Fields {
					if field.Text != "" && containsString(field.Text, "my-server") {
						hasServerName = true
					}
				}
				if block.Text != nil && containsString(block.Text.Text, "apt-get failed") {
					hasOutput = true
				}
			}
		}
		assert.True(t, hasHeader)
		assert.True(t, hasServerName)
		assert.True(t, hasOutput)
	})

	t.Run("skips when not configured", func(t *testing.T) {
		alerter := NewAdminAlerter("")
		server := ServerInfo{ID: "server-123", Name: "my-server"}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server)
		err := alert.Send(context.Background())

		assert.NoError(t, err)
	})
}

func TestServerProvisioningFailedAdminAlert_BuildMessage(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")

	t.Run("includes all server details", func(t *testing.T) {
		server := ServerInfo{
			ID:              "server-123",
			Name:            "my-server",
			TeamID:          "team-456",
			TeamName:        "My Team",
			Provider:        "digitalocean",
			PublicIPv4:      "192.168.1.100",
			ServerType:      "s-1vcpu-1gb",
			OperatingSystem: "Ubuntu 22.04",
			MemoryInMB:      1024,
			CPUCores:        1,
		}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server)
		message := alert.buildMessage()

		// Check that blocks are created
		assert.NotEmpty(t, message.Blocks)
		assert.Equal(t, "header", message.Blocks[0].Type)
	})

	t.Run("handles missing optional fields", func(t *testing.T) {
		server := ServerInfo{
			ID:   "server-123",
			Name: "my-server",
		}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server)
		message := alert.buildMessage()

		// Should not panic with missing fields
		assert.NotEmpty(t, message.Blocks)
	})

	t.Run("includes user info when provided", func(t *testing.T) {
		server := ServerInfo{ID: "server-123", Name: "my-server"}
		user := &UserInfo{Name: "John Doe", Email: "john@example.com"}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server).WithUser(user)
		message := alert.buildMessage()

		// Find user info in blocks
		found := false
		for _, block := range message.Blocks {
			for _, field := range block.Fields {
				if containsString(field.Text, "John Doe") {
					found = true
				}
			}
		}
		assert.True(t, found, "User name should be in message")
	})

	t.Run("includes output when provided", func(t *testing.T) {
		server := ServerInfo{ID: "server-123", Name: "my-server"}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server).WithOutput("apt-get failed")
		message := alert.buildMessage()

		found := false
		for _, block := range message.Blocks {
			if block.Text != nil && containsString(block.Text.Text, "apt-get failed") {
				found = true
			}
		}
		assert.True(t, found, "Output should be in message")
	})

	t.Run("includes error message when provided", func(t *testing.T) {
		server := ServerInfo{ID: "server-123", Name: "my-server"}

		alert := NewServerProvisioningFailedAdminAlert(alerter, server).WithErrorMessage("connection timeout")
		message := alert.buildMessage()

		found := false
		for _, block := range message.Blocks {
			if block.Text != nil && containsString(block.Text.Text, "connection timeout") {
				found = true
			}
		}
		assert.True(t, found, "Error message should be in message")
	})
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > 0 && containsSubstring(haystack, needle))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
