package notification

import (
	"testing"
)

func TestChannelType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       ChannelType
		expected string
	}{
		{"email", ChannelTypeEmail, "email"},
		{"slack", ChannelTypeSlack, "slack"},
		{"discord", ChannelTypeDiscord, "discord"},
		{"telegram", ChannelTypeTelegram, "telegram"},
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
		ct       ChannelType
		expected string
	}{
		{"email label", ChannelTypeEmail, "Email"},
		{"slack label", ChannelTypeSlack, "Slack"},
		{"discord label", ChannelTypeDiscord, "Discord"},
		{"telegram label", ChannelTypeTelegram, "Telegram"},
		{"unknown label", ChannelType("unknown"), "unknown"},
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
		ct       ChannelType
		expected bool
	}{
		{"email valid", ChannelTypeEmail, true},
		{"slack valid", ChannelTypeSlack, true},
		{"discord valid", ChannelTypeDiscord, true},
		{"telegram valid", ChannelTypeTelegram, true},
		{"invalid type", ChannelType("webhook"), false},
		{"empty type", ChannelType(""), false},
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
		expected  ChannelType
		expectErr bool
	}{
		{"scan string", "email", ChannelTypeEmail, false},
		{"scan bytes", []byte("slack"), ChannelTypeSlack, false},
		{"scan nil", nil, "", false},
		{"scan int", 123, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ct ChannelType
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
	ct := ChannelTypeEmail
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
		expected  ChannelType
		expectErr bool
	}{
		{"parse email", "email", ChannelTypeEmail, false},
		{"parse slack", "slack", ChannelTypeSlack, false},
		{"parse discord", "discord", ChannelTypeDiscord, false},
		{"parse telegram", "telegram", ChannelTypeTelegram, false},
		{"parse invalid", "webhook", "", true},
		{"parse empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := ParseChannelType(tt.input)

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
	types := AllChannelTypes()
	if len(types) != 4 {
		t.Errorf("AllChannelTypes() returned %d types, want 4", len(types))
	}

	expected := map[ChannelType]bool{
		ChannelTypeEmail:    true,
		ChannelTypeSlack:    true,
		ChannelTypeDiscord:  true,
		ChannelTypeTelegram: true,
	}

	for _, ct := range types {
		if !expected[ct] {
			t.Errorf("unexpected channel type: %v", ct)
		}
	}
}

func TestNotificationType_String(t *testing.T) {
	nt := NotificationTypeServerProvisioned
	if nt.String() != "server_provisioned" {
		t.Errorf("NotificationType.String() = %v, want server_provisioned", nt.String())
	}
}

func TestNotificationType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		nt       NotificationType
		expected bool
	}{
		{"server provisioned", NotificationTypeServerProvisioned, true},
		{"server provisioning failed", NotificationTypeServerProvisioningFailed, true},
		{"server connection lost", NotificationTypeServerConnectionLost, true},
		{"server threshold exceeded", NotificationTypeServerThresholdExceeded, true},
		{"deployment failed", NotificationTypeDeploymentFailed, true},
		{"site installation failed", NotificationTypeSiteInstallationFailed, true},
		{"job on server failed", NotificationTypeJobOnServerFailed, true},
		{"php installation failed", NotificationTypePhpInstallationFailed, true},
		{"php extension install failed", NotificationTypePhpExtensionInstallFailed, true},
		{"php extension uninstall failed", NotificationTypePhpExtensionUninstallFailed, true},
		{"vulnerability audit completed", NotificationTypeVulnerabilityAuditCompleted, true},
		{"failed to delete server", NotificationTypeFailedToDeleteServer, true},
		{"invalid type", NotificationType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nt.IsValid(); got != tt.expected {
				t.Errorf("NotificationType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}
