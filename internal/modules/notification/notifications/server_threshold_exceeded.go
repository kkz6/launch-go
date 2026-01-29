package notifications

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

// MetricType represents the type of server metric
type MetricType string

const (
	MetricTypeCPU    MetricType = "cpu"
	MetricTypeMemory MetricType = "memory"
	MetricTypeDisk   MetricType = "disk"
	MetricTypeLoad   MetricType = "load"
)

// ThresholdExceeded represents a single threshold that was exceeded
type ThresholdExceeded struct {
	Metric    MetricType
	Threshold float64
	Current   float64
}

// ServerThresholdExceededNotification is sent when server metrics exceed thresholds
type ServerThresholdExceededNotification struct {
	*models.BaseNotification
	ServerName string
	ServerIP   string
	Thresholds []ThresholdExceeded
}

// NewServerThresholdExceededNotification creates a new server threshold exceeded notification
func NewServerThresholdExceededNotification(serverName, serverIP string) *ServerThresholdExceededNotification {
	rawText := fmt.Sprintf("Server '%s' has exceeded one or more resource thresholds.", serverName)
	return &ServerThresholdExceededNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeServerThresholdExceeded, rawText),
		ServerName:       serverName,
		ServerIP:         serverIP,
		Thresholds:       []ThresholdExceeded{},
	}
}

// AddThreshold adds a threshold that was exceeded
func (n *ServerThresholdExceededNotification) AddThreshold(metric MetricType, threshold, current float64) *ServerThresholdExceededNotification {
	n.Thresholds = append(n.Thresholds, ThresholdExceeded{
		Metric:    metric,
		Threshold: threshold,
		Current:   current,
	})
	return n
}

func (n *ServerThresholdExceededNotification) metricLabel(metric MetricType) string {
	switch metric {
	case MetricTypeCPU:
		return "CPU Usage"
	case MetricTypeMemory:
		return "Memory Usage"
	case MetricTypeDisk:
		return "Disk Usage"
	case MetricTypeLoad:
		return "Load Average"
	default:
		return string(metric)
	}
}

// ToEmail returns the email message content
func (n *ServerThresholdExceededNotification) ToEmail() *channels.EmailMessage {
	body := fmt.Sprintf("Server '%s' (%s) has exceeded one or more resource thresholds.\n\n", n.ServerName, n.ServerIP)

	body += "Exceeded Thresholds:\n"
	for _, t := range n.Thresholds {
		body += fmt.Sprintf("• %s: %.1f%% (threshold: %.1f%%)\n", n.metricLabel(t.Metric), t.Current, t.Threshold)
	}

	body += "\nPlease check your server's resource usage."

	return &channels.EmailMessage{
		Subject: fmt.Sprintf("Server Threshold Exceeded - %s", n.ServerName),
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *ServerThresholdExceededNotification) ToSlack() string {
	message := fmt.Sprintf("*⚠️ Server Threshold Exceeded*\n\nServer '%s' (%s) has exceeded resource thresholds.\n\n*Exceeded Thresholds:*\n", n.ServerName, n.ServerIP)

	for _, t := range n.Thresholds {
		message += fmt.Sprintf("• %s: `%.1f%%` (threshold: %.1f%%)\n", n.metricLabel(t.Metric), t.Current, t.Threshold)
	}

	return message
}

// ToDiscord returns the Discord message content
func (n *ServerThresholdExceededNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the Telegram message content
func (n *ServerThresholdExceededNotification) ToTelegram() string {
	message := fmt.Sprintf("<b>⚠️ Server Threshold Exceeded</b>\n\nServer '%s' (%s) has exceeded resource thresholds.\n\n<b>Exceeded Thresholds:</b>\n", n.ServerName, n.ServerIP)

	for _, t := range n.Thresholds {
		message += fmt.Sprintf("• %s: <code>%.1f%%</code> (threshold: %.1f%%)\n", n.metricLabel(t.Metric), t.Current, t.Threshold)
	}

	return message
}
