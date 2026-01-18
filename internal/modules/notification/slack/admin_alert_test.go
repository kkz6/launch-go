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

func TestNewAdminAlerter(t *testing.T) {
	alerter := NewAdminAlerter("https://hooks.slack.com/test")

	require.NotNil(t, alerter)
	assert.Equal(t, "https://hooks.slack.com/test", alerter.webhookURL)
	assert.NotNil(t, alerter.httpClient)
}

func TestAdminAlerter_IsConfigured(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		expected   bool
	}{
		{
			name:       "configured",
			webhookURL: "https://hooks.slack.com/test",
			expected:   true,
		},
		{
			name:       "not configured",
			webhookURL: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerter := NewAdminAlerter(tt.webhookURL)
			assert.Equal(t, tt.expected, alerter.IsConfigured())
		})
	}
}

func TestAdminAlerter_Send(t *testing.T) {
	t.Run("sends message to webhook", func(t *testing.T) {
		var receivedPayload BlockKitMessage
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			assert.NoError(t, err)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		alerter := NewAdminAlerter(server.URL)
		message := NewBlockKitMessage("Test message")
		message.AddHeader("Test Header")

		err := alerter.Send(context.Background(), message)

		assert.NoError(t, err)
		assert.Equal(t, "Test message", receivedPayload.Text)
		require.Len(t, receivedPayload.Blocks, 1)
		assert.Equal(t, "header", receivedPayload.Blocks[0].Type)
	})

	t.Run("skips when not configured", func(t *testing.T) {
		alerter := NewAdminAlerter("")
		message := NewBlockKitMessage("Test message")

		err := alerter.Send(context.Background(), message)

		assert.NoError(t, err)
	})

	t.Run("handles server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		alerter := NewAdminAlerter(server.URL)
		message := NewBlockKitMessage("Test message")

		// Note: Current implementation doesn't return error on non-2xx status
		err := alerter.Send(context.Background(), message)
		assert.NoError(t, err)
	})
}

func TestNewBlockKitMessage(t *testing.T) {
	message := NewBlockKitMessage("Fallback text")

	assert.Equal(t, "Fallback text", message.Text)
	assert.Empty(t, message.Blocks)
}

func TestBlockKitMessage_AddHeader(t *testing.T) {
	message := NewBlockKitMessage("Test")

	result := message.AddHeader("My Header")

	assert.Same(t, message, result)
	require.Len(t, message.Blocks, 1)
	assert.Equal(t, "header", message.Blocks[0].Type)
	assert.Equal(t, "plain_text", message.Blocks[0].Text.Type)
	assert.Equal(t, "My Header", message.Blocks[0].Text.Text)
}

func TestBlockKitMessage_AddSection(t *testing.T) {
	message := NewBlockKitMessage("Test")

	result := message.AddSection("Section content")

	assert.Same(t, message, result)
	require.Len(t, message.Blocks, 1)
	assert.Equal(t, "section", message.Blocks[0].Type)
	assert.Equal(t, "mrkdwn", message.Blocks[0].Text.Type)
	assert.Equal(t, "Section content", message.Blocks[0].Text.Text)
}

func TestBlockKitMessage_AddFieldsSection(t *testing.T) {
	message := NewBlockKitMessage("Test")

	result := message.AddFieldsSection("*Field 1:*\nValue 1", "*Field 2:*\nValue 2")

	assert.Same(t, message, result)
	require.Len(t, message.Blocks, 1)
	assert.Equal(t, "section", message.Blocks[0].Type)
	require.Len(t, message.Blocks[0].Fields, 2)
	assert.Equal(t, "mrkdwn", message.Blocks[0].Fields[0].Type)
	assert.Equal(t, "*Field 1:*\nValue 1", message.Blocks[0].Fields[0].Text)
}

func TestBlockKitMessage_AddDivider(t *testing.T) {
	message := NewBlockKitMessage("Test")

	result := message.AddDivider()

	assert.Same(t, message, result)
	require.Len(t, message.Blocks, 1)
	assert.Equal(t, "divider", message.Blocks[0].Type)
}

func TestBlockKitMessage_AddContext(t *testing.T) {
	message := NewBlockKitMessage("Test")

	result := message.AddContext("Context text")

	assert.Same(t, message, result)
	require.Len(t, message.Blocks, 1)
	assert.Equal(t, "context", message.Blocks[0].Type)
	require.Len(t, message.Blocks[0].Elements, 1)
	assert.Equal(t, "mrkdwn", message.Blocks[0].Elements[0].Type)
	assert.Equal(t, "Context text", message.Blocks[0].Elements[0].Text)
}

func TestBlockKitMessage_Chaining(t *testing.T) {
	message := NewBlockKitMessage("Test")

	message.
		AddHeader("Header").
		AddSection("Section").
		AddDivider().
		AddFieldsSection("Field 1", "Field 2").
		AddContext("Context")

	assert.Len(t, message.Blocks, 5)
	assert.Equal(t, "header", message.Blocks[0].Type)
	assert.Equal(t, "section", message.Blocks[1].Type)
	assert.Equal(t, "divider", message.Blocks[2].Type)
	assert.Equal(t, "section", message.Blocks[3].Type)
	assert.Equal(t, "context", message.Blocks[4].Type)
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short output",
			input:    "short",
			maxLen:   100,
			expected: "short",
		},
		{
			name:     "exact length",
			input:    "12345",
			maxLen:   5,
			expected: "12345",
		},
		{
			name:     "truncated",
			input:    "1234567890",
			maxLen:   5,
			expected: "12345\n... (truncated)",
		},
		{
			name:     "default max length",
			input:    "short",
			maxLen:   0, // Should use default 2900
			expected: "short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateOutput(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBlockKitMessage_JSON(t *testing.T) {
	message := NewBlockKitMessage("Fallback")
	message.AddHeader("Test Header")
	message.AddSection("Test section with *bold* text")

	data, err := json.Marshal(message)

	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "Fallback", parsed["text"])
	blocks := parsed["blocks"].([]interface{})
	assert.Len(t, blocks, 2)
}
