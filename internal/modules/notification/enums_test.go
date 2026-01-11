package notification

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/notification/enums"
)

func TestChannelType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       enums.ChannelType
		expected string
	}{
		{"email", enums.ChannelTypeEmail, "email"},
		{"slack", enums.ChannelTypeSlack, "slack"},
		{"discord", enums.ChannelTypeDiscord, "discord"},
		{"telegram", enums.ChannelTypeTelegram, "telegram"},
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
		ct       enums.ChannelType
		expected string
	}{
		{"email label", enums.ChannelTypeEmail, "Email"},
		{"slack label", enums.ChannelTypeSlack, "Slack"},
		{"discord label", enums.ChannelTypeDiscord, "Discord"},
		{"telegram label", enums.ChannelTypeTelegram, "Telegram"},
		{"unknown label", enums.ChannelType("unknown"), "unknown"},
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
		ct       enums.ChannelType
		expected bool
	}{
		{"email valid", enums.ChannelTypeEmail, true},
		{"slack valid", enums.ChannelTypeSlack, true},
		{"discord valid", enums.ChannelTypeDiscord, true},
		{"telegram valid", enums.ChannelTypeTelegram, true},
		{"invalid type", enums.ChannelType("webhook"), false},
		{"empty type", enums.ChannelType(""), false},
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
		expected  enums.ChannelType
		expectErr bool
	}{
		{"scan string", "email", enums.ChannelTypeEmail, false},
		{"scan bytes", []byte("slack"), enums.ChannelTypeSlack, false},
		{"scan nil", nil, "", false},
		{"scan int", 123, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ct enums.ChannelType
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
	ct := enums.ChannelTypeEmail
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
		expected  enums.ChannelType
		expectErr bool
	}{
		{"parse email", "email", enums.ChannelTypeEmail, false},
		{"parse slack", "slack", enums.ChannelTypeSlack, false},
		{"parse discord", "discord", enums.ChannelTypeDiscord, false},
		{"parse telegram", "telegram", enums.ChannelTypeTelegram, false},
		{"parse invalid", "webhook", "", true},
		{"parse empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := enums.ParseChannelType(tt.input)

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
	types := enums.AllChannelTypes()
	if len(types) != 4 {
		t.Errorf("AllChannelTypes() returned %d types, want 4", len(types))
	}

	expected := map[enums.ChannelType]bool{
		enums.ChannelTypeEmail:    true,
		enums.ChannelTypeSlack:    true,
		enums.ChannelTypeDiscord:  true,
		enums.ChannelTypeTelegram: true,
	}

	for _, ct := range types {
		if !expected[ct] {
			t.Errorf("unexpected channel type: %v", ct)
		}
	}
}

func TestNotificationType_String(t *testing.T) {
	nt := enums.NotificationTypeServerProvisioned
	if nt.String() != "server_provisioned" {
		t.Errorf("NotificationType.String() = %v, want server_provisioned", nt.String())
	}
}

func TestNotificationType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		nt       enums.NotificationType
		expected bool
	}{
		{"server provisioned", enums.NotificationTypeServerProvisioned, true},
		{"server provisioning failed", enums.NotificationTypeServerProvisioningFailed, true},
		{"server connection lost", enums.NotificationTypeServerConnectionLost, true},
		{"server threshold exceeded", enums.NotificationTypeServerThresholdExceeded, true},
		{"deployment failed", enums.NotificationTypeDeploymentFailed, true},
		{"site installation failed", enums.NotificationTypeSiteInstallationFailed, true},
		{"job on server failed", enums.NotificationTypeJobOnServerFailed, true},
		{"php installation failed", enums.NotificationTypePhpInstallationFailed, true},
		{"php extension install failed", enums.NotificationTypePhpExtensionInstallFailed, true},
		{"php extension uninstall failed", enums.NotificationTypePhpExtensionUninstallFailed, true},
		{"vulnerability audit completed", enums.NotificationTypeVulnerabilityAuditCompleted, true},
		{"failed to delete server", enums.NotificationTypeFailedToDeleteServer, true},
		{"invalid type", enums.NotificationType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nt.IsValid(); got != tt.expected {
				t.Errorf("NotificationType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}
