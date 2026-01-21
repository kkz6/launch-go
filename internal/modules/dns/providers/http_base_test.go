package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/httpclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPBaseProvider(t *testing.T) {
	config := HTTPBaseConfig{
		BaseURL:      "https://api.example.com",
		ProviderName: "TestProvider",
		Token:        "test-token",
		AuthScheme:   "Bearer",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
	}

	provider := NewHTTPBaseProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "TestProvider", provider.ProviderName())
	assert.Equal(t, "test-token", provider.GetToken())
	assert.NotNil(t, provider.Client())
}

func TestHTTPBaseProvider_DefaultAuthScheme(t *testing.T) {
	config := HTTPBaseConfig{
		BaseURL:      "https://api.example.com",
		ProviderName: "TestProvider",
		Token:        "test-token",
		// AuthScheme not set - should default to "Bearer"
	}

	provider := NewHTTPBaseProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "test-token", provider.GetToken())
}

func TestHTTPBaseProvider_SetToken(t *testing.T) {
	config := HTTPBaseConfig{
		BaseURL:      "https://api.example.com",
		ProviderName: "TestProvider",
		Token:        "initial-token",
	}

	provider := NewHTTPBaseProvider(config)
	provider.SetToken("new-token")

	assert.Equal(t, "new-token", provider.GetToken())
}

func TestHTTPBaseProvider_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	var result map[string]string
	err := provider.Get(context.Background(), "/test", &result)

	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestHTTPBaseProvider_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/create", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test", body["name"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	body := map[string]string{"name": "test"}
	var result map[string]string
	err := provider.Post(context.Background(), "/create", body, &result)

	require.NoError(t, err)
	assert.Equal(t, "123", result["id"])
}

func TestHTTPBaseProvider_Put(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/update/123", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"updated": "true"})
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	var result map[string]string
	err := provider.Put(context.Background(), "/update/123", nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "true", result["updated"])
}

func TestHTTPBaseProvider_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/delete/123", r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	err := provider.Delete(context.Background(), "/delete/123", nil)

	require.NoError(t, err)
}

func TestHTTPBaseProvider_GetWithQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		assert.Equal(t, "active", r.URL.Query().Get("status"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"count": 10})
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	params := map[string]string{
		"per_page": "100",
		"status":   "active",
	}

	var result map[string]interface{}
	err := provider.GetWithQuery(context.Background(), "/list", params, &result)

	require.NoError(t, err)
	assert.Equal(t, float64(10), result["count"])
}

func TestHTTPBaseProvider_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	var result map[string]string
	err := provider.Get(context.Background(), "/test", &result)

	require.Error(t, err)

	providerErr, ok := err.(*ProviderError)
	require.True(t, ok)
	assert.Equal(t, "TestProvider", providerErr.Provider)
	assert.Equal(t, 400, providerErr.Code)
}

func TestHTTPBaseProvider_DoRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "custom-value")
		_ = json.NewEncoder(w).Encode(map[string]string{"raw": "response"})
	}))
	defer server.Close()

	config := HTTPBaseConfig{
		BaseURL:      server.URL,
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	resp, err := provider.DoRaw(context.Background(), http.MethodGet, "/raw", nil)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "custom-value", resp.Headers.Get("X-Custom"))
	assert.True(t, resp.IsSuccess())
}

func TestHTTPBaseProvider_CheckResponseSuccess(t *testing.T) {
	config := HTTPBaseConfig{
		BaseURL:      "https://api.example.com",
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	tests := []struct {
		name         string
		statusCode   int
		successCodes []int
		wantErr      bool
	}{
		{
			name:       "2xx success",
			statusCode: 200,
			wantErr:    false,
		},
		{
			name:       "201 success",
			statusCode: 201,
			wantErr:    false,
		},
		{
			name:         "204 with additional success codes",
			statusCode:   204,
			successCodes: []int{204},
			wantErr:      false,
		},
		{
			name:       "400 error",
			statusCode: 400,
			wantErr:    true,
		},
		{
			name:       "500 error",
			statusCode: 500,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &httpclient.Response{
				StatusCode: tt.statusCode,
				Body:       []byte("test body"),
			}

			err := provider.CheckResponseSuccess(resp, tt.successCodes...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPBaseProvider_UnmarshalResponse(t *testing.T) {
	config := HTTPBaseConfig{
		BaseURL:      "https://api.example.com",
		ProviderName: "TestProvider",
		Token:        "test-token",
	}

	provider := NewHTTPBaseProvider(config)

	t.Run("successful unmarshal", func(t *testing.T) {
		resp := &httpclient.Response{
			StatusCode: 200,
			Body:       []byte(`{"name": "test", "value": 123}`),
		}

		var result struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		err := provider.UnmarshalResponse(resp, &result)

		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 123, result.Value)
	})

	t.Run("error response", func(t *testing.T) {
		resp := &httpclient.Response{
			StatusCode: 400,
			Body:       []byte(`{"error": "bad request"}`),
		}

		var result map[string]string
		err := provider.UnmarshalResponse(resp, &result)

		require.Error(t, err)
	})

	t.Run("empty body", func(t *testing.T) {
		resp := &httpclient.Response{
			StatusCode: 204,
			Body:       []byte{},
		}

		var result map[string]string
		err := provider.UnmarshalResponse(resp, &result)

		require.NoError(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		resp := &httpclient.Response{
			StatusCode: 200,
			Body:       []byte(`invalid json`),
		}

		var result map[string]string
		err := provider.UnmarshalResponse(resp, &result)

		require.Error(t, err)
	})
}

func TestBuildRecordData(t *testing.T) {
	tests := []struct {
		name     string
		record   *DNSRecord
		expected map[string]interface{}
	}{
		{
			name: "basic A record",
			record: &DNSRecord{
				Type: RecordTypeA,
				Name: "@",
				TTL:  3600,
			},
			expected: map[string]interface{}{
				"type": "A",
				"name": "@",
				"ttl":  3600,
			},
		},
		{
			name: "MX record with priority",
			record: &DNSRecord{
				Type:     RecordTypeMX,
				Name:     "@",
				TTL:      3600,
				Priority: IntPtr(10),
			},
			expected: map[string]interface{}{
				"type":     "MX",
				"name":     "@",
				"ttl":      3600,
				"priority": 10,
			},
		},
		{
			name: "SRV record with all fields",
			record: &DNSRecord{
				Type:     RecordTypeSRV,
				Name:     "_sip._tcp",
				TTL:      3600,
				Priority: IntPtr(10),
				Weight:   IntPtr(5),
				Port:     IntPtr(5060),
			},
			expected: map[string]interface{}{
				"type":     "SRV",
				"name":     "_sip._tcp",
				"ttl":      3600,
				"priority": 10,
				"weight":   5,
				"port":     5060,
			},
		},
		{
			name: "CAA record with flags and tag",
			record: &DNSRecord{
				Type:  RecordTypeCAA,
				Name:  "@",
				TTL:   3600,
				Flags: IntPtr(0),
				Tag:   StringPtr("issue"),
			},
			expected: map[string]interface{}{
				"type":  "CAA",
				"name":  "@",
				"ttl":   3600,
				"flags": 0,
				"tag":   "issue",
			},
		},
		{
			name: "record with comment",
			record: &DNSRecord{
				Type:    RecordTypeA,
				Name:    "www",
				TTL:     3600,
				Comment: StringPtr("Web server"),
			},
			expected: map[string]interface{}{
				"type":    "A",
				"name":    "www",
				"ttl":     3600,
				"comment": "Web server",
			},
		},
		{
			name: "record with empty comment",
			record: &DNSRecord{
				Type:    RecordTypeA,
				Name:    "www",
				TTL:     3600,
				Comment: StringPtr(""),
			},
			expected: map[string]interface{}{
				"type": "A",
				"name": "www",
				"ttl":  3600,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildRecordData(tt.record)

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s mismatch", key)
			}

			// Ensure no extra keys
			assert.Len(t, result, len(tt.expected))
		})
	}
}

func TestPointerHelpers(t *testing.T) {
	t.Run("IntPtr", func(t *testing.T) {
		ptr := IntPtr(42)
		require.NotNil(t, ptr)
		assert.Equal(t, 42, *ptr)
	})

	t.Run("StringPtr", func(t *testing.T) {
		ptr := StringPtr("test")
		require.NotNil(t, ptr)
		assert.Equal(t, "test", *ptr)
	})

	t.Run("BoolPtr", func(t *testing.T) {
		ptrTrue := BoolPtr(true)
		require.NotNil(t, ptrTrue)
		assert.True(t, *ptrTrue)

		ptrFalse := BoolPtr(false)
		require.NotNil(t, ptrFalse)
		assert.False(t, *ptrFalse)
	})
}
