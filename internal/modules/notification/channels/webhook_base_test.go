package channels

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestNewWebhookChannel(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	webhookURL := "https://example.com/webhook"

	channel := NewWebhookChannel(mockHTTP, webhookURL)

	if channel.GetWebhookURL() != webhookURL {
		t.Errorf("GetWebhookURL() = %s, want %s", channel.GetWebhookURL(), webhookURL)
	}

	if channel.GetHTTPClient() != mockHTTP {
		t.Error("GetHTTPClient() did not return the expected client")
	}
}

func TestWebhookChannel_ValidateConfig(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		httpClient HTTPClient
		expectErr  bool
	}{
		{
			name:       "valid config",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{},
			expectErr:  false,
		},
		{
			name:       "empty webhook URL",
			webhookURL: "",
			httpClient: &MockHTTPClient{},
			expectErr:  true,
		},
		{
			name:       "nil HTTP client",
			webhookURL: "https://example.com/webhook",
			httpClient: nil,
			expectErr:  true,
		},
		{
			name:       "both empty URL and nil client",
			webhookURL: "",
			httpClient: nil,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := NewWebhookChannel(tt.httpClient, tt.webhookURL)

			err := channel.ValidateConfig()

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				if !errors.Is(err, ErrInvalidConfiguration) {
					t.Errorf("expected ErrInvalidConfiguration, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWebhookChannel_Post(t *testing.T) {
	tests := []struct {
		name       string
		webhookURL string
		httpClient HTTPClient
		statusCode int
		httpError  error
		expectErr  bool
		errType    error
	}{
		{
			name:       "successful post",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, http.StatusOK, nil
				},
			},
			expectErr: false,
		},
		{
			name:       "successful post with 201",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, http.StatusCreated, nil
				},
			},
			expectErr: false,
		},
		{
			name:       "successful post with 204",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, http.StatusNoContent, nil
				},
			},
			expectErr: false,
		},
		{
			name:       "empty webhook URL",
			webhookURL: "",
			httpClient: &MockHTTPClient{},
			expectErr:  true,
			errType:    ErrInvalidConfiguration,
		},
		{
			name:       "nil HTTP client",
			webhookURL: "https://example.com/webhook",
			httpClient: nil,
			expectErr:  true,
			errType:    ErrInvalidConfiguration,
		},
		{
			name:       "HTTP error",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, 0, errors.New("network error")
				},
			},
			expectErr: true,
			errType:   ErrSendFailed,
		},
		{
			name:       "server error 500",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, http.StatusInternalServerError, nil
				},
			},
			expectErr: true,
			errType:   ErrSendFailed,
		},
		{
			name:       "client error 400",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, http.StatusBadRequest, nil
				},
			},
			expectErr: true,
			errType:   ErrSendFailed,
		},
		{
			name:       "redirect 302",
			webhookURL: "https://example.com/webhook",
			httpClient: &MockHTTPClient{
				PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
					return nil, http.StatusFound, nil
				},
			},
			expectErr: true,
			errType:   ErrSendFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := NewWebhookChannel(tt.httpClient, tt.webhookURL)

			payload := map[string]string{"message": "test"}
			err := channel.Post(context.Background(), payload)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWebhookChannel_Post_PassesCorrectURL(t *testing.T) {
	expectedURL := "https://example.com/webhook"
	var receivedURL string

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedURL = url
			return nil, http.StatusOK, nil
		},
	}

	channel := NewWebhookChannel(mockHTTP, expectedURL)

	_ = channel.Post(context.Background(), nil)

	if receivedURL != expectedURL {
		t.Errorf("received URL = %s, want %s", receivedURL, expectedURL)
	}
}

func TestWebhookChannel_Post_PassesCorrectPayload(t *testing.T) {
	var receivedPayload interface{}

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedPayload = body
			return nil, http.StatusOK, nil
		},
	}

	channel := NewWebhookChannel(mockHTTP, "https://example.com/webhook")

	expectedPayload := map[string]string{"key": "value"}
	_ = channel.Post(context.Background(), expectedPayload)

	payload, ok := receivedPayload.(map[string]string)
	if !ok {
		t.Error("expected map[string]string type")
		return
	}

	if payload["key"] != "value" {
		t.Errorf("payload[key] = %s, want 'value'", payload["key"])
	}
}

func TestWebhookChannel_GetWebhookURL(t *testing.T) {
	expectedURL := "https://example.com/webhook"
	channel := NewWebhookChannel(nil, expectedURL)

	if channel.GetWebhookURL() != expectedURL {
		t.Errorf("GetWebhookURL() = %s, want %s", channel.GetWebhookURL(), expectedURL)
	}
}

func TestWebhookChannel_SetWebhookURL(t *testing.T) {
	channel := NewWebhookChannel(nil, "https://old-url.com")

	newURL := "https://new-url.com/webhook"
	channel.SetWebhookURL(newURL)

	if channel.GetWebhookURL() != newURL {
		t.Errorf("GetWebhookURL() after SetWebhookURL = %s, want %s", channel.GetWebhookURL(), newURL)
	}
}

func TestWebhookChannel_GetHTTPClient(t *testing.T) {
	mockHTTP := &MockHTTPClient{}
	channel := NewWebhookChannel(mockHTTP, "https://example.com")

	if channel.GetHTTPClient() != mockHTTP {
		t.Error("GetHTTPClient() did not return the expected client")
	}
}

func TestWebhookChannel_SetHTTPClient(t *testing.T) {
	oldClient := &MockHTTPClient{}
	channel := NewWebhookChannel(oldClient, "https://example.com")

	newClient := &MockHTTPClient{}
	channel.SetHTTPClient(newClient)

	if channel.GetHTTPClient() != newClient {
		t.Error("GetHTTPClient() after SetHTTPClient did not return the new client")
	}
}

func TestWebhookChannel_Post_RespectsContext(t *testing.T) {
	var receivedCtx context.Context

	mockHTTP := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			receivedCtx = ctx
			return nil, http.StatusOK, nil
		},
	}

	channel := NewWebhookChannel(mockHTTP, "https://example.com/webhook")

	type ctxKey string
	const testKey ctxKey = "test-key"
	ctx := context.WithValue(context.Background(), testKey, "test-value")
	_ = channel.Post(ctx, nil)

	if receivedCtx.Value(testKey) != "test-value" {
		t.Error("context was not passed correctly to HTTP client")
	}
}
