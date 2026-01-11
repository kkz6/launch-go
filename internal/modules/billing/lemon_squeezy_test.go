package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLemonSqueezyConfig(t *testing.T) {
	config := providers.DefaultLemonSqueezyConfig()

	assert.Equal(t, "https://api.lemonsqueezy.com/v1", config.BaseURL)
	assert.NotZero(t, config.Timeout)
}

func TestNewLemonSqueezyClient(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("with default values", func(t *testing.T) {
		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		assert.NotNil(t, client)
		// Default BaseURL is set internally when empty
		assert.Equal(t, "https://api.lemonsqueezy.com/v1", config.BaseURL)
	})

	t.Run("with custom values", func(t *testing.T) {
		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: "https://custom.api.com",
			Timeout: 60 * time.Second,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		assert.NotNil(t, client)
		// Config values are preserved
		assert.Equal(t, "https://custom.api.com", config.BaseURL)
		assert.Equal(t, 60*time.Second, config.Timeout)
	})
}

func TestLemonSqueezyClient_CreateCheckout(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("successful checkout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/checkouts", r.URL.Path)
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"attributes": map[string]interface{}{
						"url": "https://checkout.lemonsqueezy.com/checkout/123",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		url, err := client.CreateCheckout(context.Background(), "456", "Pro Plan", "team_1", "https://redirect.com")
		require.NoError(t, err)
		assert.Contains(t, url, "https://checkout.lemonsqueezy.com/checkout/123")
	})

	t.Run("successful checkout without redirect", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"attributes": map[string]interface{}{
						"url": "https://checkout.lemonsqueezy.com/checkout/123",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		url, err := client.CreateCheckout(context.Background(), "456", "Pro Plan", "team_1", "")
		require.NoError(t, err)
		assert.Equal(t, "https://checkout.lemonsqueezy.com/checkout/123", url)
	})

	t.Run("invalid variant ID", func(t *testing.T) {
		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		_, err := client.CreateCheckout(context.Background(), "invalid_variant", "Pro Plan", "team_1", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid variant ID")
	})

	t.Run("api error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		_, err := client.CreateCheckout(context.Background(), "456", "Pro Plan", "team_1", "https://redirect.com")
		assert.Error(t, err)
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "invalid_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		_, err := client.CreateCheckout(context.Background(), "456", "Pro Plan", "team_1", "https://redirect.com")
		assert.ErrorIs(t, err, providers.ErrLemonSqueezyUnauthorized)
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		_, err := client.CreateCheckout(context.Background(), "456", "Pro Plan", "team_1", "https://redirect.com")
		assert.ErrorIs(t, err, providers.ErrLemonSqueezyRateLimited)
	})
}

func TestLemonSqueezyClient_GetSubscription(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("successful get", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id": "sub_123",
					"attributes": map[string]interface{}{
						"status":     "active",
						"product_id": 1,
						"variant_id": 1,
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		response, err := client.GetSubscription(context.Background(), "sub_123")
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		_, err := client.GetSubscription(context.Background(), "sub_123")
		assert.ErrorIs(t, err, providers.ErrLemonSqueezyNotFound)
	})
}

func TestLemonSqueezyClient_CancelSubscription(t *testing.T) {
	logger := zerolog.Nop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id": "sub_123",
				"attributes": map[string]interface{}{
					"status": "cancelled",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &providers.LemonSqueezyConfig{
		APIKey:  "test_key",
		StoreID: 123,
		BaseURL: server.URL,
	}
	client := providers.NewLemonSqueezyClient(config, &logger)

	err := client.CancelSubscription(context.Background(), "sub_123")
	assert.NoError(t, err)
}

func TestLemonSqueezyClient_ResumeSubscription(t *testing.T) {
	logger := zerolog.Nop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id": "sub_123",
				"attributes": map[string]interface{}{
					"status": "active",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &providers.LemonSqueezyConfig{
		APIKey:  "test_key",
		StoreID: 123,
		BaseURL: server.URL,
	}
	client := providers.NewLemonSqueezyClient(config, &logger)

	err := client.ResumeSubscription(context.Background(), "sub_123")
	assert.NoError(t, err)
}

func TestLemonSqueezyClient_PauseSubscription(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("without resumes_at", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PATCH", r.Method)
			assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id": "sub_123",
					"attributes": map[string]interface{}{
						"status": "paused",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		err := client.PauseSubscription(context.Background(), "sub_123", "void", nil)
		assert.NoError(t, err)
	})

	t.Run("with resumes_at", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PATCH", r.Method)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id": "sub_123",
					"attributes": map[string]interface{}{
						"status": "paused",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		// Use a fixed time for testing
		resumesAt := parseTestTime("2024-02-01T00:00:00Z")
		err := client.PauseSubscription(context.Background(), "sub_123", "free", &resumesAt)
		assert.NoError(t, err)
	})
}

func TestLemonSqueezyClient_UnpauseSubscription(t *testing.T) {
	logger := zerolog.Nop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id": "sub_123",
				"attributes": map[string]interface{}{
					"status": "active",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &providers.LemonSqueezyConfig{
		APIKey:  "test_key",
		StoreID: 123,
		BaseURL: server.URL,
	}
	client := providers.NewLemonSqueezyClient(config, &logger)

	err := client.UnpauseSubscription(context.Background(), "sub_123")
	assert.NoError(t, err)
}

func TestLemonSqueezyClient_UpdateSubscription(t *testing.T) {
	logger := zerolog.Nop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id": "sub_123",
				"attributes": map[string]interface{}{
					"status": "active",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := &providers.LemonSqueezyConfig{
		APIKey:  "test_key",
		StoreID: 123,
		BaseURL: server.URL,
	}
	client := providers.NewLemonSqueezyClient(config, &logger)

	err := client.UpdateSubscription(context.Background(), "sub_123", 456)
	assert.NoError(t, err)
}

func TestLemonSqueezyClient_GetUpdatePaymentMethodURL(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("with URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id": "sub_123",
					"attributes": map[string]interface{}{
						"urls": map[string]interface{}{
							"update_payment_method": "https://update.payment.url",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		url, err := client.GetUpdatePaymentMethodURL(context.Background(), "sub_123")
		require.NoError(t, err)
		assert.Equal(t, "https://update.payment.url", url)
	})

	t.Run("without URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id":         "sub_123",
					"attributes": map[string]interface{}{},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		url, err := client.GetUpdatePaymentMethodURL(context.Background(), "sub_123")
		require.NoError(t, err)
		assert.Empty(t, url)
	})
}

func TestLemonSqueezyClient_GetCustomerPortalURL(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("with URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/subscriptions/sub_123", r.URL.Path)

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id": "sub_123",
					"attributes": map[string]interface{}{
						"urls": map[string]interface{}{
							"customer_portal": "https://customer.portal.url",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		url, err := client.GetCustomerPortalURL(context.Background(), "sub_123")
		require.NoError(t, err)
		assert.Equal(t, "https://customer.portal.url", url)
	})

	t.Run("without URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"id":         "sub_123",
					"attributes": map[string]interface{}{},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &providers.LemonSqueezyConfig{
			APIKey:  "test_key",
			StoreID: 123,
			BaseURL: server.URL,
		}
		client := providers.NewLemonSqueezyClient(config, &logger)

		url, err := client.GetCustomerPortalURL(context.Background(), "sub_123")
		require.NoError(t, err)
		assert.Empty(t, url)
	})
}

func TestParseLemonSqueezyTime(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		wantNil  bool
	}{
		{
			name:     "valid RFC3339 time",
			input:    strPtr("2024-01-15T10:30:00Z"),
			wantNil:  false,
		},
		{
			name:     "valid RFC3339 with timezone",
			input:    strPtr("2024-01-15T10:30:00+05:00"),
			wantNil:  false,
		},
		{
			name:     "nil input",
			input:    nil,
			wantNil:  true,
		},
		{
			name:     "empty string",
			input:    strPtr(""),
			wantNil:  true,
		},
		{
			name:     "invalid format",
			input:    strPtr("not-a-date"),
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := providers.ParseLemonSqueezyTime(tt.input)

			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func parseTestTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
