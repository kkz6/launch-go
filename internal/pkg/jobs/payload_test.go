package jobs

import (
	"encoding/json"
	"testing"
)

func TestBasePayload_HasUser(t *testing.T) {
	userID := "user-123"

	tests := []struct {
		name    string
		payload BasePayload
		want    bool
	}{
		{
			name:    "with user ID",
			payload: BasePayload{UserID: &userID},
			want:    true,
		},
		{
			name:    "without user ID",
			payload: BasePayload{},
			want:    false,
		},
		{
			name:    "with empty user ID",
			payload: BasePayload{UserID: func() *string { s := ""; return &s }()},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.payload.HasUser(); got != tt.want {
				t.Errorf("HasUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBasePayload_GetUserID(t *testing.T) {
	userID := "user-123"

	tests := []struct {
		name    string
		payload BasePayload
		want    string
	}{
		{
			name:    "with user ID",
			payload: BasePayload{UserID: &userID},
			want:    "user-123",
		},
		{
			name:    "without user ID",
			payload: BasePayload{},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.payload.GetUserID(); got != tt.want {
				t.Errorf("GetUserID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBasePayload_WithUser(t *testing.T) {
	p := BasePayload{}
	p = p.WithUser("user-123")

	if !p.HasUser() {
		t.Error("Expected HasUser() to be true after WithUser()")
	}
	if p.GetUserID() != "user-123" {
		t.Errorf("Expected user ID 'user-123', got '%s'", p.GetUserID())
	}
}

func TestBasePayload_WithTeam(t *testing.T) {
	p := BasePayload{}
	p = p.WithTeam("team-456")

	if !p.HasTeam() {
		t.Error("Expected HasTeam() to be true after WithTeam()")
	}
	if p.TeamID != "team-456" {
		t.Errorf("Expected team ID 'team-456', got '%s'", p.TeamID)
	}
}

func TestNewBasePayload(t *testing.T) {
	t.Run("without user ID", func(t *testing.T) {
		p := NewBasePayload()
		if p.HasUser() {
			t.Error("Expected HasUser() to be false")
		}
	})

	t.Run("with user ID", func(t *testing.T) {
		p := NewBasePayload("user-123")
		if !p.HasUser() {
			t.Error("Expected HasUser() to be true")
		}
		if p.GetUserID() != "user-123" {
			t.Errorf("Expected user ID 'user-123', got '%s'", p.GetUserID())
		}
	})

	t.Run("with empty user ID", func(t *testing.T) {
		p := NewBasePayload("")
		if p.HasUser() {
			t.Error("Expected HasUser() to be false for empty string")
		}
	})
}

func TestServerPayload(t *testing.T) {
	p := NewServerPayload("server-789", "user-123")

	if p.ServerID != "server-789" {
		t.Errorf("Expected server ID 'server-789', got '%s'", p.ServerID)
	}
	if !p.HasUser() {
		t.Error("Expected HasUser() to be true")
	}
}

func TestSitePayload(t *testing.T) {
	p := NewSitePayload("site-001").WithServer("server-789")

	if p.SiteID != "site-001" {
		t.Errorf("Expected site ID 'site-001', got '%s'", p.SiteID)
	}
	if p.ServerID != "server-789" {
		t.Errorf("Expected server ID 'server-789', got '%s'", p.ServerID)
	}
}

func TestPayloadBuilder(t *testing.T) {
	t.Run("build with all fields", func(t *testing.T) {
		payload := NewPayloadBuilder().
			WithUser("user-123").
			WithTeam("team-456").
			WithServer("server-789").
			WithSite("site-001").
			WithField("custom_field", "custom_value").
			Build()

		if payload["user_id"] != "user-123" {
			t.Errorf("Expected user_id 'user-123', got '%v'", payload["user_id"])
		}
		if payload["team_id"] != "team-456" {
			t.Errorf("Expected team_id 'team-456', got '%v'", payload["team_id"])
		}
		if payload["server_id"] != "server-789" {
			t.Errorf("Expected server_id 'server-789', got '%v'", payload["server_id"])
		}
		if payload["site_id"] != "site-001" {
			t.Errorf("Expected site_id 'site-001', got '%v'", payload["site_id"])
		}
		if payload["custom_field"] != "custom_value" {
			t.Errorf("Expected custom_field 'custom_value', got '%v'", payload["custom_field"])
		}
	})

	t.Run("build JSON", func(t *testing.T) {
		builder := NewPayloadBuilder().
			WithServer("server-123").
			WithField("cron_id", "cron-456")

		data, err := builder.BuildJSON()
		if err != nil {
			t.Fatalf("BuildJSON() error = %v", err)
		}

		var result map[string]string
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		if result["server_id"] != "server-123" {
			t.Errorf("Expected server_id 'server-123', got '%s'", result["server_id"])
		}
		if result["cron_id"] != "cron-456" {
			t.Errorf("Expected cron_id 'cron-456', got '%s'", result["cron_id"])
		}
	})

	t.Run("to task", func(t *testing.T) {
		task, err := NewPayloadBuilder().
			WithServer("server-123").
			ToTask("test:job")

		if err != nil {
			t.Fatalf("ToTask() error = %v", err)
		}

		if task.Type() != "test:job" {
			t.Errorf("Expected task type 'test:job', got '%s'", task.Type())
		}
	})

	t.Run("empty user/team not included", func(t *testing.T) {
		payload := NewPayloadBuilder().
			WithUser("").
			WithTeam("").
			Build()

		if _, ok := payload["user_id"]; ok {
			t.Error("Expected user_id to not be included for empty string")
		}
		if _, ok := payload["team_id"]; ok {
			t.Error("Expected team_id to not be included for empty string")
		}
	})
}

func TestBasePayload_JSONSerialization(t *testing.T) {
	userID := "user-123"
	p := BasePayload{
		UserID: &userID,
		TeamID: "team-456",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded BasePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.GetUserID() != "user-123" {
		t.Errorf("Expected user ID 'user-123', got '%s'", decoded.GetUserID())
	}
	if decoded.TeamID != "team-456" {
		t.Errorf("Expected team ID 'team-456', got '%s'", decoded.TeamID)
	}
}

func TestServerPayload_JSONSerialization(t *testing.T) {
	p := NewServerPayload("server-789", "user-123")
	p.TeamID = "team-456"

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ServerPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.ServerID != "server-789" {
		t.Errorf("Expected server ID 'server-789', got '%s'", decoded.ServerID)
	}
	if decoded.GetUserID() != "user-123" {
		t.Errorf("Expected user ID 'user-123', got '%s'", decoded.GetUserID())
	}
	if decoded.TeamID != "team-456" {
		t.Errorf("Expected team ID 'team-456', got '%s'", decoded.TeamID)
	}
}
