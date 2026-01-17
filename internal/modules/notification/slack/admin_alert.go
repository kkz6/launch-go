package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// AdminAlerter sends admin alerts to Slack
type AdminAlerter struct {
	webhookURL string
	httpClient *http.Client
}

// NewAdminAlerter creates a new admin alerter
func NewAdminAlerter(webhookURL string) *AdminAlerter {
	return &AdminAlerter{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsConfigured returns true if the admin webhook is configured
func (a *AdminAlerter) IsConfigured() bool {
	return a.webhookURL != ""
}

// Send sends a Slack BlockKit message
func (a *AdminAlerter) Send(ctx context.Context, message *BlockKitMessage) error {
	if !a.IsConfigured() {
		return nil // Silently skip if not configured
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// BlockKitMessage represents a Slack BlockKit message
type BlockKitMessage struct {
	Text   string  `json:"text"`
	Blocks []Block `json:"blocks"`
}

// NewBlockKitMessage creates a new BlockKit message
func NewBlockKitMessage(fallbackText string) *BlockKitMessage {
	return &BlockKitMessage{
		Text:   fallbackText,
		Blocks: []Block{},
	}
}

// Block represents a Slack block
type Block struct {
	Type     string       `json:"type"`
	Text     *TextObject  `json:"text,omitempty"`
	Fields   []TextObject `json:"fields,omitempty"`
	Elements []TextObject `json:"elements,omitempty"`
}

// TextObject represents a Slack text object
type TextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// AddHeader adds a header block
func (m *BlockKitMessage) AddHeader(text string) *BlockKitMessage {
	m.Blocks = append(m.Blocks, Block{
		Type: "header",
		Text: &TextObject{
			Type: "plain_text",
			Text: text,
		},
	})
	return m
}

// AddSection adds a section block with markdown text
func (m *BlockKitMessage) AddSection(text string) *BlockKitMessage {
	m.Blocks = append(m.Blocks, Block{
		Type: "section",
		Text: &TextObject{
			Type: "mrkdwn",
			Text: text,
		},
	})
	return m
}

// AddFieldsSection adds a section with fields
func (m *BlockKitMessage) AddFieldsSection(fields ...string) *BlockKitMessage {
	textObjects := make([]TextObject, len(fields))
	for i, field := range fields {
		textObjects[i] = TextObject{
			Type: "mrkdwn",
			Text: field,
		}
	}

	m.Blocks = append(m.Blocks, Block{
		Type:   "section",
		Fields: textObjects,
	})
	return m
}

// AddDivider adds a divider block
func (m *BlockKitMessage) AddDivider() *BlockKitMessage {
	m.Blocks = append(m.Blocks, Block{
		Type: "divider",
	})
	return m
}

// AddContext adds a context block
func (m *BlockKitMessage) AddContext(text string) *BlockKitMessage {
	m.Blocks = append(m.Blocks, Block{
		Type: "context",
		Elements: []TextObject{
			{
				Type: "mrkdwn",
				Text: text,
			},
		},
	})
	return m
}

// TruncateOutput truncates output to fit Slack's limits (max 2900 chars)
func TruncateOutput(output string, maxLen int) string {
	if maxLen == 0 {
		maxLen = 2900
	}

	if len(output) > maxLen {
		return output[:maxLen] + "\n... (truncated)"
	}
	return output
}
