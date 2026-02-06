package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestNewServerThresholdExceededNotification(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")

	require.NotNil(t, notif)
	assert.Equal(t, "my-server", notif.ServerName)
	assert.Equal(t, "192.168.1.100", notif.ServerIP)
	assert.Empty(t, notif.Thresholds)
	assert.Equal(t, notificationtypes.NotificationTypeServerThresholdExceeded, notif.Type())
	assert.Contains(t, notif.RawText(), "my-server")
	assert.Contains(t, notif.RawText(), "threshold")
}

func TestServerThresholdExceededNotification_AddThreshold(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")

	result := notif.AddThreshold(MetricTypeCPU, 80.0, 95.5)

	assert.Same(t, notif, result)
	require.Len(t, notif.Thresholds, 1)
	assert.Equal(t, MetricTypeCPU, notif.Thresholds[0].Metric)
	assert.Equal(t, 80.0, notif.Thresholds[0].Threshold)
	assert.Equal(t, 95.5, notif.Thresholds[0].Current)
}

func TestServerThresholdExceededNotification_AddMultipleThresholds(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")

	notif.AddThreshold(MetricTypeCPU, 80.0, 95.5).
		AddThreshold(MetricTypeMemory, 90.0, 92.0).
		AddThreshold(MetricTypeDisk, 85.0, 88.0)

	require.Len(t, notif.Thresholds, 3)
	assert.Equal(t, MetricTypeCPU, notif.Thresholds[0].Metric)
	assert.Equal(t, MetricTypeMemory, notif.Thresholds[1].Metric)
	assert.Equal(t, MetricTypeDisk, notif.Thresholds[2].Metric)
}

func TestServerThresholdExceededNotification_MetricLabel(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")

	tests := []struct {
		metric   MetricType
		expected string
	}{
		{MetricTypeCPU, "CPU Usage"},
		{MetricTypeMemory, "Memory Usage"},
		{MetricTypeDisk, "Disk Usage"},
		{MetricTypeLoad, "Load Average"},
		{MetricType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.metric), func(t *testing.T) {
			assert.Equal(t, tt.expected, notif.metricLabel(tt.metric))
		})
	}
}

func TestServerThresholdExceededNotification_ToEmail(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")
	notif.AddThreshold(MetricTypeCPU, 80.0, 95.5)
	notif.AddThreshold(MetricTypeMemory, 90.0, 92.0)

	email := notif.ToEmail()

	require.NotNil(t, email)
	assert.Contains(t, email.Subject, "Server Threshold Exceeded")
	assert.Contains(t, email.Subject, "my-server")
	assert.Contains(t, email.Body, "my-server")
	assert.Contains(t, email.Body, "192.168.1.100")
	assert.Contains(t, email.Body, "CPU Usage")
	assert.Contains(t, email.Body, "Memory Usage")
	assert.Contains(t, email.Body, "95.5%")
	assert.Contains(t, email.Body, "80.0%")
	assert.True(t, email.IsHTML)
}

func TestServerThresholdExceededNotification_ToSlack(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")
	notif.AddThreshold(MetricTypeCPU, 80.0, 95.5)

	message := notif.ToSlack()

	assert.Contains(t, message, "⚠️")
	assert.Contains(t, message, "Server Threshold Exceeded")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "192.168.1.100")
	assert.Contains(t, message, "CPU Usage")
	assert.Contains(t, message, "95.5%")
}

func TestServerThresholdExceededNotification_ToDiscord(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")
	notif.AddThreshold(MetricTypeCPU, 80.0, 95.5)

	message := notif.ToDiscord()

	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "CPU Usage")
}

func TestServerThresholdExceededNotification_ToTelegram(t *testing.T) {
	notif := NewServerThresholdExceededNotification("my-server", "192.168.1.100")
	notif.AddThreshold(MetricTypeCPU, 80.0, 95.5)

	message := notif.ToTelegram()

	assert.Contains(t, message, "<b>")
	assert.Contains(t, message, "my-server")
	assert.Contains(t, message, "CPU Usage")
	assert.Contains(t, message, "<code>")
}

func TestMetricTypes(t *testing.T) {
	assert.Equal(t, MetricType("cpu"), MetricTypeCPU)
	assert.Equal(t, MetricType("memory"), MetricTypeMemory)
	assert.Equal(t, MetricType("disk"), MetricTypeDisk)
	assert.Equal(t, MetricType("load"), MetricTypeLoad)
}
