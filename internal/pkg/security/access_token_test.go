package security

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const accessTokenTestSecret = "access-token-test-secret"

func resetAccessTokenChecks(t *testing.T) {
	t.Helper()
	SetSessionValidator(nil)
	SetUserSuspensionChecker(nil)
	t.Cleanup(func() {
		SetSessionValidator(nil)
		SetUserSuspensionChecker(nil)
	})
}

func signAccessToken(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestParseAccessTokenRejectsInvalidTokens(t *testing.T) {
	resetAccessTokenChecks(t)
	sessionChecks := 0
	SetSessionValidator(func(string) bool {
		sessionChecks++
		return true
	})

	for _, tc := range []struct {
		name   string
		claims jwt.MapClaims
		secret string
	}{
		{"wrong signature", jwt.MapClaims{"sub": "user-1", "type": "access", "session_id": "session-1"}, "wrong-secret"},
		{"refresh token", jwt.MapClaims{"sub": "user-1", "type": "refresh"}, accessTokenTestSecret},
		{"missing type", jwt.MapClaims{"sub": "user-1"}, accessTokenTestSecret},
		{"non-string type", jwt.MapClaims{"sub": "user-1", "type": true}, accessTokenTestSecret},
		{"missing subject", jwt.MapClaims{"type": "access"}, accessTokenTestSecret},
		{"empty subject", jwt.MapClaims{"sub": "", "type": "access"}, accessTokenTestSecret},
		{"non-string subject", jwt.MapClaims{"sub": 123, "type": "access"}, accessTokenTestSecret},
		{"expired", jwt.MapClaims{"sub": "user-1", "type": "access", "session_id": "session-1", "exp": time.Now().Add(-time.Hour).Unix()}, accessTokenTestSecret},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := ParseAccessToken(signAccessToken(t, tc.claims, tc.secret), accessTokenTestSecret)
			if err == nil || claims != nil {
				t.Fatalf("invalid token returned claims=%v, err=%v", claims, err)
			}
		})
	}

	t.Run("malformed", func(t *testing.T) {
		if claims, err := ParseAccessToken("not-a-jwt", accessTokenTestSecret); err == nil || claims != nil {
			t.Fatalf("malformed token returned claims=%v, err=%v", claims, err)
		}
	})
	t.Run("unsigned", func(t *testing.T) {
		token, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": "user-1", "type": "access"}).SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatal(err)
		}
		if claims, err := ParseAccessToken(token, accessTokenTestSecret); err == nil || claims != nil {
			t.Fatalf("unsigned token returned claims=%v, err=%v", claims, err)
		}
	})
	if sessionChecks != 0 {
		t.Errorf("invalid tokens caused %d session checks", sessionChecks)
	}
}

func TestParseAccessTokenSessionValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sessionID any
		validator bool
		live      bool
		wantCalls int
		wantErr   error
	}{
		{"live session", "session-1", true, true, 1, nil},
		{"revoked session", "session-1", true, false, 1, ErrSessionRevoked},
		{"sessionless token", nil, true, false, 0, nil},
		{"empty session", "", true, false, 0, nil},
		{"non-string session", 123, true, false, 0, nil},
		{"validator not configured", "session-1", false, false, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAccessTokenChecks(t)
			calls := 0
			if tc.validator {
				SetSessionValidator(func(sessionID string) bool {
					calls++
					if sessionID != tc.sessionID {
						t.Errorf("validated session %q, want %v", sessionID, tc.sessionID)
					}
					return tc.live
				})
			}
			claims := jwt.MapClaims{"sub": "user-1", "type": "access", "team_id": "team-1"}
			if tc.sessionID != nil {
				claims["session_id"] = tc.sessionID
			}
			got, err := ParseAccessToken(signAccessToken(t, claims, accessTokenTestSecret), accessTokenTestSecret)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseAccessToken() error=%v, want %v", err, tc.wantErr)
			}
			if calls != tc.wantCalls {
				t.Errorf("session validator called %d times, want %d", calls, tc.wantCalls)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Errorf("revoked token returned claims: %v", got)
				}
			} else if got["sub"] != "user-1" || got["team_id"] != "team-1" {
				t.Errorf("validated token lost claims: %v", got)
			}
		})
	}
}

func TestAccessTokenSuspensionPolicyIsSeparateFromParsing(t *testing.T) {
	resetAccessTokenChecks(t)
	if IsUserSuspended("user-1") {
		t.Fatal("a missing suspension checker must preserve existing access")
	}
	calls := 0
	SetUserSuspensionChecker(func(userID string) bool {
		calls++
		return userID == "suspended-user"
	})
	token := signAccessToken(t, jwt.MapClaims{"sub": "suspended-user", "type": "access"}, accessTokenTestSecret)
	if _, err := ParseAccessToken(token, accessTokenTestSecret); err != nil {
		t.Fatalf("parsing must leave account-state policy to the caller: %v", err)
	}
	if calls != 0 {
		t.Fatal("parsing unexpectedly checked suspension")
	}
	if !IsUserSuspended("suspended-user") || IsUserSuspended("active-user") || calls != 2 {
		t.Fatalf("suspension lookup did not delegate both user IDs; calls=%d", calls)
	}
	SetUserSuspensionChecker(nil)
	if IsUserSuspended("suspended-user") {
		t.Fatal("clearing the suspension checker did not restore default behavior")
	}
}

func TestIsReadOnlyImpersonation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		claims jwt.MapClaims
		want   bool
	}{
		{"read-only impersonation", jwt.MapClaims{"impersonation_sid": "staff-session", "read_only": true}, true},
		{"writable impersonation", jwt.MapClaims{"impersonation_sid": "staff-session", "read_only": false}, false},
		{"missing read-only claim", jwt.MapClaims{"impersonation_sid": "staff-session"}, false},
		{"string read-only claim", jwt.MapClaims{"impersonation_sid": "staff-session", "read_only": "true"}, false},
		{"ordinary read-only claim", jwt.MapClaims{"read_only": true}, false},
		{"empty string retains read-only restriction", jwt.MapClaims{"impersonation_sid": "", "read_only": true}, true},
		{"non-string impersonation session", jwt.MapClaims{"impersonation_sid": 123, "read_only": true}, false},
		{"nil claims", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsReadOnlyImpersonation(tc.claims); got != tc.want {
				t.Errorf("IsReadOnlyImpersonation()=%v, want %v", got, tc.want)
			}
		})
	}
}
