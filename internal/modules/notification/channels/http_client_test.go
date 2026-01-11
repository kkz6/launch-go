package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHTTPClient_Post(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewDefaultHTTPClient()
	ctx := context.Background()

	body := map[string]string{"message": "test"}
	respBody, statusCode, err := client.Post(ctx, server.URL, body)

	if err != nil {
		t.Errorf("Post() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if string(respBody) != `{"status":"ok"}` {
		t.Errorf("respBody = %s, want {\"status\":\"ok\"}", string(respBody))
	}
}

func TestDefaultHTTPClient_Post_NilBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDefaultHTTPClient()
	ctx := context.Background()

	_, statusCode, err := client.Post(ctx, server.URL, nil)

	if err != nil {
		t.Errorf("Post() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
}

func TestDefaultHTTPClient_Post_InvalidURL(t *testing.T) {
	client := NewDefaultHTTPClient()
	ctx := context.Background()

	_, _, err := client.Post(ctx, "://invalid-url", nil)

	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestDefaultHTTPClient_Post_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	client := NewDefaultHTTPClient()
	ctx := context.Background()

	_, statusCode, err := client.Post(ctx, server.URL, nil)

	if err != nil {
		t.Errorf("Post() error = %v", err)
	}
	if statusCode != http.StatusInternalServerError {
		t.Errorf("statusCode = %d, want 500", statusCode)
	}
}

func TestDefaultHTTPClient_Post_MarshalError(t *testing.T) {
	client := NewDefaultHTTPClient()
	ctx := context.Background()

	// Create an unmarshalable value (channel cannot be marshaled)
	ch := make(chan int)
	_, _, err := client.Post(ctx, "http://example.com", ch)

	if err == nil {
		t.Error("expected error for unmarshalable body")
	}
}

func TestDefaultHTTPClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"test"}`))
	}))
	defer server.Close()

	client := NewDefaultHTTPClient()
	ctx := context.Background()

	respBody, statusCode, err := client.Get(ctx, server.URL)

	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if string(respBody) != `{"data":"test"}` {
		t.Errorf("respBody = %s, want {\"data\":\"test\"}", string(respBody))
	}
}

func TestDefaultHTTPClient_Get_InvalidURL(t *testing.T) {
	client := NewDefaultHTTPClient()
	ctx := context.Background()

	_, _, err := client.Get(ctx, "://invalid-url")

	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestMockHTTPClient_Post(t *testing.T) {
	called := false
	mockClient := &MockHTTPClient{
		PostFunc: func(ctx context.Context, url string, body interface{}) ([]byte, int, error) {
			called = true
			return []byte("mock response"), 200, nil
		},
	}

	ctx := context.Background()
	respBody, statusCode, err := mockClient.Post(ctx, "http://example.com", nil)

	if !called {
		t.Error("PostFunc should have been called")
	}
	if err != nil {
		t.Errorf("Post() error = %v", err)
	}
	if statusCode != 200 {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if string(respBody) != "mock response" {
		t.Errorf("respBody = %s, want 'mock response'", string(respBody))
	}
}

func TestMockHTTPClient_Post_NoFunc(t *testing.T) {
	mockClient := &MockHTTPClient{}

	ctx := context.Background()
	respBody, statusCode, err := mockClient.Post(ctx, "http://example.com", nil)

	if err != nil {
		t.Errorf("Post() error = %v", err)
	}
	if statusCode != 200 {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if respBody != nil {
		t.Errorf("respBody = %v, want nil", respBody)
	}
}

func TestMockHTTPClient_Get(t *testing.T) {
	called := false
	mockClient := &MockHTTPClient{
		GetFunc: func(ctx context.Context, url string) ([]byte, int, error) {
			called = true
			return []byte("mock get response"), 200, nil
		},
	}

	ctx := context.Background()
	respBody, statusCode, err := mockClient.Get(ctx, "http://example.com")

	if !called {
		t.Error("GetFunc should have been called")
	}
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if statusCode != 200 {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if string(respBody) != "mock get response" {
		t.Errorf("respBody = %s, want 'mock get response'", string(respBody))
	}
}

func TestMockHTTPClient_Get_NoFunc(t *testing.T) {
	mockClient := &MockHTTPClient{}

	ctx := context.Background()
	respBody, statusCode, err := mockClient.Get(ctx, "http://example.com")

	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if statusCode != 200 {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if respBody != nil {
		t.Errorf("respBody = %v, want nil", respBody)
	}
}

func TestDefaultHTTPClient_Post_JSONBody(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDefaultHTTPClient()
	ctx := context.Background()

	body := map[string]string{"key": "value", "message": "hello"}
	_, _, err := client.Post(ctx, server.URL, body)

	if err != nil {
		t.Errorf("Post() error = %v", err)
	}

	if receivedBody["key"] != "value" {
		t.Errorf("receivedBody[\"key\"] = %s, want 'value'", receivedBody["key"])
	}
	if receivedBody["message"] != "hello" {
		t.Errorf("receivedBody[\"message\"] = %s, want 'hello'", receivedBody["message"])
	}
}

func TestNewDefaultHTTPClient(t *testing.T) {
	client := NewDefaultHTTPClient()

	if client == nil {
		t.Error("NewDefaultHTTPClient() returned nil")
	}
	if client.client == nil {
		t.Error("client.client should not be nil")
	}
	if client.client.Timeout == 0 {
		t.Error("client should have a timeout set")
	}
}
