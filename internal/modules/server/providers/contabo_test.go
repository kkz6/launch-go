package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContaboClientListsInstancesWithAccountToken(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.NoError(t, r.ParseForm())
			assert.Equal(t, "password", r.Form.Get("grant_type"))
			assert.Equal(t, "id", r.Form.Get("client_id"))
			assert.Equal(t, "a+b&c", r.Form.Get("client_secret"))
			assert.Equal(t, "test@example.com", r.Form.Get("username"))
			assert.Equal(t, " p+a&ss ", r.Form.Get("password"))
			fmt.Fprint(w, `{"access_token":"test-token"}`)
		case "/instances":
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Len(t, r.Header.Get("X-Request-ID"), 36)
			assert.Equal(t, "2", r.URL.Query().Get("page"))
			assert.Equal(t, "25", r.URL.Query().Get("size"))
			fmt.Fprint(w, `{"_pagination":{"totalPages":3},"data":[{"instanceId":9007199254740993,"tenantId":"INT","osType":"Linux","status":"running","ipConfig":{"v4":{"ip":"8.8.4.4"}}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()

	client := NewContaboClient()
	client.tokenURL = remote.URL + "/token"
	client.apiURL = remote.URL + "/instances"
	page, err := client.ListInstances(context.Background(), ContaboCredentials{
		ClientID: "id", ClientSecret: "a+b&c", APIUser: "test@example.com", APIPassword: " p+a&ss ",
	}, 2, 25)
	require.NoError(t, err)
	require.Equal(t, 3, page.Pagination.TotalPages)
	require.Len(t, page.Data, 1)
	require.Equal(t, "INT:9007199254740993", page.Data[0].Key())
	require.True(t, page.Data[0].CanImport())
}
