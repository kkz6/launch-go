package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPhpInstallationFailedAdminAlert(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpInstallationFailedAdminAlert(alerter, server, "8.2")

	require.NotNil(t, alert)
	assert.Equal(t, server, alert.Server)
	assert.Equal(t, "8.2", alert.phpVersion)
}

func TestPhpInstallationFailedAdminAlert_WithUser(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}
	user := &UserInfo{Name: "John Doe", Email: "john@example.com"}

	alert := NewPhpInstallationFailedAdminAlert(alerter, server, "8.2")
	result := alert.WithUser(user)

	assert.Same(t, alert, result)
	assert.Equal(t, user, alert.User)
}

func TestPhpInstallationFailedAdminAlert_WithOutput(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpInstallationFailedAdminAlert(alerter, server, "8.2")
	result := alert.WithOutput("apt-get failed")

	assert.Same(t, alert, result)
	assert.Equal(t, "apt-get failed", alert.Output)
}

func TestPhpInstallationFailedAdminAlert_WithErrorMessage(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpInstallationFailedAdminAlert(alerter, server, "8.2")
	result := alert.WithErrorMessage("package not found")

	assert.Same(t, alert, result)
	assert.Equal(t, "package not found", alert.ErrorMessage)
}

func TestPhpInstallationFailedAdminAlert_Send(t *testing.T) {
	var receivedPayload BlockKitMessage
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	alerter := NewAdminAlerter(testServer.URL)
	server := ServerInfo{
		ID:       "server-123",
		Name:     "my-server",
		TeamID:   "team-456",
		TeamName: "My Team",
	}

	alert := NewPhpInstallationFailedAdminAlert(alerter, server, "8.2").
		WithOutput("apt-get failed").
		WithErrorMessage("package not found")

	err := alert.Send(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, receivedPayload.Text, "PHP 8.2")
	assert.Contains(t, receivedPayload.Text, "my-server")

	// Verify header contains PHP version
	hasHeader := false
	for _, block := range receivedPayload.Blocks {
		if block.Type == "header" && block.Text != nil {
			if strings.Contains(block.Text.Text, "PHP") {
				hasHeader = true
			}
		}
	}
	assert.True(t, hasHeader)
}

func TestNewPhpExtensionInstallFailedAdminAlert(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpExtensionInstallFailedAdminAlert(alerter, server, "redis", "8.2")

	require.NotNil(t, alert)
	assert.Equal(t, server, alert.Server)
	assert.Equal(t, "redis", alert.extensionName)
	assert.Equal(t, "8.2", alert.phpVersion)
}

func TestPhpExtensionInstallFailedAdminAlert_WithUser(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}
	user := &UserInfo{Name: "John Doe", Email: "john@example.com"}

	alert := NewPhpExtensionInstallFailedAdminAlert(alerter, server, "redis", "8.2")
	result := alert.WithUser(user)

	assert.Same(t, alert, result)
	assert.Equal(t, user, alert.User)
}

func TestPhpExtensionInstallFailedAdminAlert_Send(t *testing.T) {
	var receivedPayload BlockKitMessage
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	alerter := NewAdminAlerter(testServer.URL)
	server := ServerInfo{
		ID:       "server-123",
		Name:     "my-server",
		TeamID:   "team-456",
		TeamName: "My Team",
	}

	alert := NewPhpExtensionInstallFailedAdminAlert(alerter, server, "redis", "8.2").
		WithOutput("pecl failed").
		WithErrorMessage("compilation error")

	err := alert.Send(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, receivedPayload.Text, "redis")
	assert.Contains(t, receivedPayload.Text, "my-server")

	// PHP version is in the blocks, not the fallback text
	hasPhpVersion := false
	for _, block := range receivedPayload.Blocks {
		for _, field := range block.Fields {
			if strings.Contains(field.Text, "8.2") {
				hasPhpVersion = true
			}
		}
	}
	assert.True(t, hasPhpVersion, "PHP version should be in the blocks")
}

func TestNewPhpExtensionUninstallFailedAdminAlert(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpExtensionUninstallFailedAdminAlert(alerter, server, "redis", "8.2")

	require.NotNil(t, alert)
	assert.Equal(t, server, alert.Server)
	assert.Equal(t, "redis", alert.extensionName)
	assert.Equal(t, "8.2", alert.phpVersion)
}

func TestPhpExtensionUninstallFailedAdminAlert_WithUser(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")
	server := ServerInfo{ID: "server-123", Name: "my-server"}
	user := &UserInfo{Name: "John Doe", Email: "john@example.com"}

	alert := NewPhpExtensionUninstallFailedAdminAlert(alerter, server, "redis", "8.2")
	result := alert.WithUser(user)

	assert.Same(t, alert, result)
	assert.Equal(t, user, alert.User)
}

func TestPhpExtensionUninstallFailedAdminAlert_Send(t *testing.T) {
	var receivedPayload BlockKitMessage
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	alerter := NewAdminAlerter(testServer.URL)
	server := ServerInfo{
		ID:       "server-123",
		Name:     "my-server",
		TeamID:   "team-456",
		TeamName: "My Team",
	}

	alert := NewPhpExtensionUninstallFailedAdminAlert(alerter, server, "redis", "8.2").
		WithOutput("apt-get remove failed").
		WithErrorMessage("package in use")

	err := alert.Send(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, receivedPayload.Text, "redis")
}

func TestPhpExtensionInstallFailedAdminAlert_SkipsWhenNotConfigured(t *testing.T) {
	alerter := NewAdminAlerter("")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpExtensionInstallFailedAdminAlert(alerter, server, "redis", "8.2")
	err := alert.Send(context.Background())

	assert.NoError(t, err)
}

func TestPhpExtensionUninstallFailedAdminAlert_SkipsWhenNotConfigured(t *testing.T) {
	alerter := NewAdminAlerter("")
	server := ServerInfo{ID: "server-123", Name: "my-server"}

	alert := NewPhpExtensionUninstallFailedAdminAlert(alerter, server, "redis", "8.2")
	err := alert.Send(context.Background())

	assert.NoError(t, err)
}
