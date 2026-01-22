package notification

import (
	"testing"

	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
)

func TestChannelType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       notificationtypes.ChannelType
		expected string
	}{
		{"email", notificationtypes.ChannelTypeEmail, "email"},
		{"slack", notificationtypes.ChannelTypeSlack, "slack"},
		{"discord", notificationtypes.ChannelTypeDiscord, "discord"},
		{"telegram", notificationtypes.ChannelTypeTelegram, "telegram"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.expected {
				t.Errorf("ChannelType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChannelType_Label(t *testing.T) {
	tests := []struct {
		name     string
		ct       notificationtypes.ChannelType
		expected string
	}{
		{"email label", notificationtypes.ChannelTypeEmail, "Email"},
		{"slack label", notificationtypes.ChannelTypeSlack, "Slack"},
		{"discord label", notificationtypes.ChannelTypeDiscord, "Discord"},
		{"telegram label", notificationtypes.ChannelTypeTelegram, "Telegram"},
		{"unknown label", notificationtypes.ChannelType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ct.Label(); got != tt.expected {
				t.Errorf("ChannelType.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChannelType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		ct       notificationtypes.ChannelType
		expected bool
	}{
		{"email valid", notificationtypes.ChannelTypeEmail, true},
		{"slack valid", notificationtypes.ChannelTypeSlack, true},
		{"discord valid", notificationtypes.ChannelTypeDiscord, true},
		{"telegram valid", notificationtypes.ChannelTypeTelegram, true},
		{"invalid type", notificationtypes.ChannelType("webhook"), false},
		{"empty type", notificationtypes.ChannelType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ct.IsValid(); got != tt.expected {
				t.Errorf("ChannelType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChannelType_Scan(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		expected  notificationtypes.ChannelType
		expectErr bool
	}{
		{"scan string", "email", notificationtypes.ChannelTypeEmail, false},
		{"scan bytes", []byte("slack"), notificationtypes.ChannelTypeSlack, false},
		{"scan nil", nil, "", false},
		{"scan int", 123, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ct notificationtypes.ChannelType
			err := ct.Scan(tt.value)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ct != tt.expected {
				t.Errorf("ChannelType.Scan() = %v, want %v", ct, tt.expected)
			}
		})
	}
}

func TestChannelType_Value(t *testing.T) {
	ct := notificationtypes.ChannelTypeEmail
	val, err := ct.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != "email" {
		t.Errorf("ChannelType.Value() = %v, want email", val)
	}
}

func TestParseChannelType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  notificationtypes.ChannelType
		expectErr bool
	}{
		{"parse email", "email", notificationtypes.ChannelTypeEmail, false},
		{"parse slack", "slack", notificationtypes.ChannelTypeSlack, false},
		{"parse discord", "discord", notificationtypes.ChannelTypeDiscord, false},
		{"parse telegram", "telegram", notificationtypes.ChannelTypeTelegram, false},
		{"parse invalid", "webhook", "", true},
		{"parse empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := notificationtypes.ParseChannelType(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ct != tt.expected {
				t.Errorf("ParseChannelType() = %v, want %v", ct, tt.expected)
			}
		})
	}
}

func TestAllChannelTypes(t *testing.T) {
	types := notificationtypes.AllChannelTypes()
	if len(types) != 4 {
		t.Errorf("AllChannelTypes() returned %d types, want 4", len(types))
	}

	expected := map[notificationtypes.ChannelType]bool{
		notificationtypes.ChannelTypeEmail:    true,
		notificationtypes.ChannelTypeSlack:    true,
		notificationtypes.ChannelTypeDiscord:  true,
		notificationtypes.ChannelTypeTelegram: true,
	}

	for _, ct := range types {
		if !expected[ct] {
			t.Errorf("unexpected channel type: %v", ct)
		}
	}
}

func TestNotificationType_String(t *testing.T) {
	nt := notificationtypes.NotificationTypeServerProvisioned
	if nt.String() != "server_provisioned" {
		t.Errorf("NotificationType.String() = %v, want server_provisioned", nt.String())
	}
}

func TestNotificationType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		nt       notificationtypes.NotificationType
		expected bool
	}{
		{"server provisioned", notificationtypes.NotificationTypeServerProvisioned, true},
		{"server provisioning failed", notificationtypes.NotificationTypeServerProvisioningFailed, true},
		{"server connection lost", notificationtypes.NotificationTypeServerConnectionLost, true},
		{"server threshold exceeded", notificationtypes.NotificationTypeServerThresholdExceeded, true},
		{"deployment failed", notificationtypes.NotificationTypeDeploymentFailed, true},
		{"site installation failed", notificationtypes.NotificationTypeSiteInstallationFailed, true},
		{"job on server failed", notificationtypes.NotificationTypeJobOnServerFailed, true},
		{"php installation failed", notificationtypes.NotificationTypePhpInstallationFailed, true},
		{"php extension install failed", notificationtypes.NotificationTypePhpExtensionInstallFailed, true},
		{"php extension uninstall failed", notificationtypes.NotificationTypePhpExtensionUninstallFailed, true},
		{"vulnerability audit completed", notificationtypes.NotificationTypeVulnerabilityAuditCompleted, true},
		{"failed to delete server", notificationtypes.NotificationTypeFailedToDeleteServer, true},
		{"invalid type", notificationtypes.NotificationType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nt.IsValid(); got != tt.expected {
				t.Errorf("NotificationType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}
