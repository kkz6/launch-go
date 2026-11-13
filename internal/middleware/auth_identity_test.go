package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/kkz6/launch-go/tests/testutil"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const authIdentitySecret = "http-identity-test-secret"

func authIdentityFixture(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	InitSessionValidator(nil)
	InitUserStatus(statusLoader)
	t.Cleanup(func() {
		InitSessionValidator(nil)
		InitUserStatus(nil)
	})
	db := testutil.SQLite(t)
	require.NoError(t, db.Exec("CREATE TABLE users (id TEXT PRIMARY KEY, locale TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO users VALUES ('u1', 'en'), ('u2', 'en')").Error)
	require.NoError(t, db.Exec("CREATE TABLE personal_access_tokens (id TEXT PRIMARY KEY, tokenable_id TEXT, token TEXT, expires_at DATETIME, last_used_at DATETIME)").Error)
	app := fiber.New()
	identity := func(c *fiber.Ctx) error {
		userID, _ := c.Locals("userID").(string)
		if userID == "" {
			userID = "anonymous"
		}
		return c.SendString(userID)
	}
	app.Post("/required", Auth(authIdentitySecret, db), identity)
	app.Post("/optional", OptionalAuth(authIdentitySecret, db), identity)
	return app, db
}

func storeIdentityPAT(t *testing.T, db *gorm.DB, token, userID string, expiresAt *time.Time) {
	t.Helper()
	hash := sha256.Sum256([]byte(token))
	require.NoError(t, db.Exec("INSERT INTO personal_access_tokens (id, tokenable_id, token, expires_at) VALUES (?, ?, ?, ?)", "pat-1", userID, hex.EncodeToString(hash[:]), expiresAt).Error)
}

func requestIdentity(t *testing.T, app *fiber.App, path, token string) (int, string) {
	t.Helper()
	request := httptest.NewRequest("POST", path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := app.Test(request)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(body)
}

func TestAuthIdentityPreservesPATFallback(t *testing.T) {
	expired := time.Now().Add(-time.Hour)
	for _, tc := range []struct {
		name      string
		userID    string
		expiresAt *time.Time
		status    int
		optional  string
	}{
		{"active", "u2", nil, fiber.StatusOK, "u2"},
		{"expired", "u2", &expired, fiber.StatusUnauthorized, "anonymous"},
		{"suspended", "u1", nil, fiber.StatusForbidden, "u1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, db := authIdentityFixture(t)
			InitSessionValidator(func(string) bool { t.Error("PAT must not check JWT sessions"); return false })
			storeIdentityPAT(t, db, "personal-access-token", tc.userID, tc.expiresAt)
			status, _ := requestIdentity(t, app, "/required", "personal-access-token")
			require.Equal(t, tc.status, status)
			status, body := requestIdentity(t, app, "/optional", "personal-access-token")
			require.Equal(t, fiber.StatusOK, status)
			require.Equal(t, tc.optional, body)
			var row struct{ LastUsedAt *time.Time }
			require.NoError(t, db.Table("personal_access_tokens").First(&row).Error)
			require.Equal(t, tc.expiresAt == nil, row.LastUsedAt != nil)
		})
	}
}

func TestOptionalAuthPreservesRestrictedIdentity(t *testing.T) {
	app, _ := authIdentityFixture(t)
	for _, tc := range []struct{ name, token, userID string }{
		{"suspended", mintAccessToken(t, authIdentitySecret, "u1"), "u1"},
		{"read-only", signIdentityClaims(t, jwt.MapClaims{"sub": "u2", "type": "access", "read_only": true, "impersonation_sid": "spectate"}), "u2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := requestIdentity(t, app, "/required", tc.token)
			require.Equal(t, fiber.StatusForbidden, status)
			status, body := requestIdentity(t, app, "/optional", tc.token)
			require.Equal(t, fiber.StatusOK, status)
			require.Equal(t, tc.userID, body)
		})
	}
}

func signIdentityClaims(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(authIdentitySecret))
	require.NoError(t, err)
	return token
}

func TestAuthIdentityRevokedJWTDoesNotFallBackToPAT(t *testing.T) {
	app, db := authIdentityFixture(t)
	InitSessionValidator(func(string) bool { return false })
	token := mintAccessTokenWithSession(t, authIdentitySecret, "u2", "revoked")
	// Even a matching PAT record cannot turn a revoked session into valid auth.
	storeIdentityPAT(t, db, token, "u2", nil)
	status, _ := requestIdentity(t, app, "/required", token)
	require.Equal(t, fiber.StatusUnauthorized, status)
	status, body := requestIdentity(t, app, "/optional", token)
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, "anonymous", body)
	var row struct{ LastUsedAt *time.Time }
	require.NoError(t, db.Table("personal_access_tokens").First(&row).Error)
	require.Nil(t, row.LastUsedAt, "revoked JWT must not reach PAT authentication")
}
