package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database/migrations"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

const deviceTestJWTSecret = "device-test-secret"

type deviceTestFixture struct {
	db      *gorm.DB
	app     *fiber.App
	url     string
	handler *DeviceHandler
	now     time.Time
	user    models.User
	jwt     string
}

func newDeviceTestFixture(t *testing.T) *deviceTestFixture {
	t.Helper()
	dialector := sqlite.Open(filepath.Join(t.TempDir(), "device.db") + "?_busy_timeout=5000&_journal_mode=WAL")
	schema := ""
	// DEVICE_AUTH_TEST_DSN must point to a disposable PostgreSQL database.
	// Each test isolates a schema, but runs the complete migration set, whose
	// future changes could affect objects outside that schema.
	if dsn := os.Getenv("DEVICE_AUTH_TEST_DSN"); dsn != "" {
		schema = "device_auth_" + strings.ToLower(util.NewULID())
		if strings.Contains(dsn, "://") {
			parsed, err := url.Parse(dsn)
			require.NoError(t, err)
			query := parsed.Query()
			query.Set("search_path", schema)
			parsed.RawQuery = query.Encode()
			dsn = parsed.String()
		} else {
			dsn += " search_path=" + schema
		}
		dialector = postgres.Open(dsn)
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	if schema != "" {
		require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() { require.NoError(t, db.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
		log := zerolog.Nop()
		require.NoError(t, migrations.NewMigrator(db, &log).Migrate())
	} else {
		require.NoError(t, db.AutoMigrate(&models.User{}, &models.Team{}, &models.Session{}, &models.PersonalAccessToken{}, &models.DeviceAuthorization{}, &models.DeviceAuthorizationLimit{}))
	}
	f := &deviceTestFixture{db: db, now: time.Now().UTC()}
	f.user.Name, f.user.Email, f.user.Password = "CLI user", "cli@example.test", "unused"
	require.NoError(t, db.Create(&f.user).Error)
	session := models.Session{ID: "browser-session", UserID: &f.user.ID, Payload: "", LastActivity: int(time.Now().Unix())}
	require.NoError(t, db.Create(&session).Error)
	f.jwt = deviceTestJWT(t, f.user.ID, session.ID, nil)
	twoFactor := services.NewTwoFactorService(repositories.NewRegistry(db), &config.Config{}, nil)
	f.handler = NewDeviceHandler(db, twoFactor, deviceTestJWTSecret, "https://launchctl.example")
	f.handler.now = func() time.Time { return f.now }
	f.app = fiber.New(fiber.Config{DisableStartupMessage: true, ErrorHandler: fiberctx.NewErrorHandler()})
	f.app.Post("/auth/device/code", f.handler.Code)
	f.app.Post("/auth/device/token", fiberctx.Validate(f.handler.Token))
	f.app.Post("/auth/device/authorize", middleware.Auth(deviceTestJWTSecret, db), fiberctx.Validate(f.handler.Authorize))
	f.app.Get("/check", middleware.Auth(deviceTestJWTSecret, f.db), func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f.url = "http://" + listener.Addr().String()
	done := make(chan error, 1)
	go func() { done <- f.app.Listener(listener) }()
	t.Cleanup(func() {
		require.NoError(t, f.app.Shutdown())
		require.NoError(t, <-done)
	})
	return f
}

func deviceTestJWT(t *testing.T, userID, sessionID string, extra jwt.MapClaims) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": userID, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}
	if sessionID != "" {
		claims["session_id"] = sessionID
	}
	for key, value := range extra {
		claims[key] = value
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(deviceTestJWTSecret))
	require.NoError(t, err)
	return token
}

func deviceRequest(baseURL string, path, token string, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func readDeviceResponse[T any](t *testing.T, response *http.Response, wantStatus int) T {
	t.Helper()
	defer response.Body.Close()
	var body struct {
		Data T `json:"data"`
	}
	require.Equal(t, wantStatus, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	return body.Data
}

func (f *deviceTestFixture) start(t *testing.T) deviceCodeResponse {
	t.Helper()
	response, err := deviceRequest(f.url, "/auth/device/code", "", nil)
	require.NoError(t, err)
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	return readDeviceResponse[deviceCodeResponse](t, response, http.StatusOK)
}

func (f *deviceTestFixture) poll(t *testing.T, secret string) deviceTokenResponse {
	t.Helper()
	response, err := deviceRequest(f.url, "/auth/device/token", "", map[string]string{"device_code": secret})
	require.NoError(t, err)
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	return readDeviceResponse[deviceTokenResponse](t, response, http.StatusOK)
}

func (f *deviceTestFixture) decide(t *testing.T, userCode string, approve bool, code string, wantStatus int) {
	t.Helper()
	response, err := deviceRequest(f.url, "/auth/device/authorize", f.jwt, map[string]any{"user_code": userCode, "approve": approve, "code": code})
	require.NoError(t, err)
	readDeviceResponse[map[string]string](t, response, wantStatus)
}

func TestDeviceSignInIssuesOneRevocableCredential(t *testing.T) {
	f := newDeviceTestFixture(t)
	grant := f.start(t)
	require.Regexp(t, `^[0-9]{8}$`, grant.UserCode)
	require.Regexp(t, `^[0-9a-f]{64}$`, grant.DeviceCode)
	require.Equal(t, "https://launchctl.example/cli/auth", grant.VerificationURI)
	require.Equal(t, 600, grant.ExpiresIn)
	require.Equal(t, 5, grant.Interval)
	var stored models.DeviceAuthorization
	require.NoError(t, f.db.First(&stored).Error)
	require.Equal(t, deviceHash(grant.DeviceCode), stored.DeviceCodeHash)
	require.Equal(t, deviceHash(grant.UserCode), stored.UserCodeHash)
	require.Equal(t, "pending", f.poll(t, grant.DeviceCode).Status)
	require.Equal(t, "slow_down", f.poll(t, grant.DeviceCode).Status)
	f.decide(t, grant.UserCode, true, "", http.StatusOK)
	f.now = f.now.Add(devicePollInterval)
	result := f.poll(t, grant.DeviceCode)
	require.Equal(t, "approved", result.Status)
	require.NotEmpty(t, result.AccessToken)
	require.Equal(t, f.user.ID, result.User.ID)
	var pat models.PersonalAccessToken
	require.NoError(t, f.db.First(&pat).Error)
	require.Equal(t, "lctl (device sign-in)", pat.Name)
	require.Equal(t, f.user.ID, pat.TokenableID)
	require.Equal(t, deviceHash(result.AccessToken), pat.Token)
	require.Equal(t, `["*"]`, *pat.Abilities)
	require.Equal(t, "expired", f.poll(t, grant.DeviceCode).Status)
	f.decide(t, grant.UserCode, true, "", http.StatusConflict)
	// The issued credential uses the existing PAT validator and revocation path.
	req, err := http.NewRequest(http.MethodGet, f.url+"/check", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+result.AccessToken)
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())
	rows, err := repositories.NewPersonalAccessTokenRepository(f.db).DeleteByUser(t.Context(), pat.ID, f.user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)
	response, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	require.NoError(t, response.Body.Close())
}

func TestDeviceSignInDenialExpiryAndMissingUserIssueNoToken(t *testing.T) {
	for _, scenario := range []string{"denied", "expired", "suspended", "deleted", "unknown secret"} {
		t.Run(scenario, func(t *testing.T) {
			f := newDeviceTestFixture(t)
			grant := f.start(t)
			want := "denied"
			switch scenario {
			case "denied":
				f.decide(t, grant.UserCode, false, "", http.StatusOK)
				f.decide(t, grant.UserCode, true, "", http.StatusConflict)
			case "expired":
				f.now = f.now.Add(deviceGrantTTL)
				f.decide(t, grant.UserCode, true, "", http.StatusUnprocessableEntity)
				want = "expired"
			case "suspended", "deleted":
				f.decide(t, grant.UserCode, true, "", http.StatusOK)
				if scenario == "suspended" {
					require.NoError(t, f.db.Model(&f.user).Update("status", "suspended").Error)
				} else {
					require.NoError(t, f.db.Delete(&f.user).Error)
				}
			case "unknown secret":
				grant.DeviceCode = strings.Repeat("0", 64)
				want = "expired"
			}
			result := f.poll(t, grant.DeviceCode)
			require.Equal(t, want, result.Status)
			require.Empty(t, result.AccessToken)
			var count int64
			require.NoError(t, f.db.Model(&models.PersonalAccessToken{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestDeviceApprovalRequiresBrowserSessionAndRejectsRestrictedCredentials(t *testing.T) {
	for _, kind := range []string{"PAT", "sessionless JWT", "impersonation", "revoked session", "suspended account"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeviceTestFixture(t)
			grant := f.start(t)
			want := http.StatusForbidden
			switch kind {
			case "PAT":
				f.jwt = strings.Repeat("a", 64)
				require.NoError(t, f.db.Create(&models.PersonalAccessToken{TokenableType: "User", TokenableID: f.user.ID, Name: "restricted", Token: deviceHash(f.jwt)}).Error)
			case "sessionless JWT":
				f.jwt = deviceTestJWT(t, f.user.ID, "", nil)
			case "impersonation":
				f.jwt = deviceTestJWT(t, f.user.ID, "browser-session", jwt.MapClaims{"impersonation_sid": "staff-session", "read_only": false})
			case "revoked session":
				require.NoError(t, f.db.Delete(&models.Session{}, "id = ?", "browser-session").Error)
				want = http.StatusUnauthorized
			case "suspended account":
				require.NoError(t, f.db.Model(&f.user).Update("status", "suspended").Error)
			}
			f.decide(t, grant.UserCode, true, "", want)
			var stored models.DeviceAuthorization
			require.NoError(t, f.db.First(&stored).Error)
			require.Equal(t, "pending", stored.Status)
			require.Nil(t, stored.UserID)
		})
	}
}

func TestDeviceApprovalRequiresTwoFactorAndConsumesRecoveryCode(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(fmt.Sprint("recovery=", recovery), func(t *testing.T) {
			f := newDeviceTestFixture(t)
			secret := "JBSWY3DPEHPK3PXP"
			hash, err := bcrypt.GenerateFromPassword([]byte("recovery-code"), bcrypt.MinCost)
			require.NoError(t, err)
			require.NoError(t, f.db.Model(&f.user).Updates(map[string]any{"two_factor_secret": secret, "two_factor_confirmed_at": f.now, "two_factor_recovery_codes": string(hash)}).Error)
			grant := f.start(t)
			f.decide(t, grant.UserCode, true, "", http.StatusUnprocessableEntity)
			f.decide(t, grant.UserCode, true, "invalid", http.StatusUnprocessableEntity)
			code := "recovery-code"
			if !recovery {
				code, err = totp.GenerateCode(secret, time.Now())
				require.NoError(t, err)
			}
			f.decide(t, grant.UserCode, true, code, http.StatusOK)
			require.Equal(t, "approved", f.poll(t, grant.DeviceCode).Status)
			if recovery {
				var user models.User
				require.NoError(t, f.db.First(&user, "id = ?", f.user.ID).Error)
				require.NotNil(t, user.TwoFactorRecoveryCodes)
				require.Empty(t, *user.TwoFactorRecoveryCodes)
			}
		})
	}
}

func TestDeviceGrantAttemptLimit(t *testing.T) {
	f := newDeviceTestFixture(t)
	require.NoError(t, f.db.Model(&f.user).Updates(map[string]any{"two_factor_secret": "JBSWY3DPEHPK3PXP", "two_factor_confirmed_at": f.now}).Error)
	grant := f.start(t)
	for i := 0; i < deviceMaxAttempts; i++ {
		f.decide(t, grant.UserCode, true, "invalid", http.StatusUnprocessableEntity)
	}
	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	require.NoError(t, err)
	f.decide(t, grant.UserCode, true, code, http.StatusUnprocessableEntity)
	var stored models.DeviceAuthorization
	require.NoError(t, f.db.First(&stored).Error)
	require.Equal(t, deviceMaxAttempts, stored.AuthorizationAttempts)
	require.Equal(t, "pending", stored.Status)
}

func TestDevicePollConcurrentConsumptionIssuesExactlyOnePAT(t *testing.T) {
	f := newDeviceTestFixture(t)
	grant := f.start(t)
	f.decide(t, grant.UserCode, true, "", http.StatusOK)
	var wg sync.WaitGroup
	results := make(chan deviceTokenResponse, 8)
	for range 8 {
		wg.Go(func() {
			response, err := deviceRequest(f.url, "/auth/device/token", "", map[string]string{"device_code": grant.DeviceCode})
			if err != nil {
				t.Error(err)
				return
			}
			defer response.Body.Close()
			var envelope struct{ Data deviceTokenResponse }
			if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || response.StatusCode != http.StatusOK {
				t.Errorf("poll HTTP %d, decode error %v", response.StatusCode, err)
				return
			}
			results <- envelope.Data
		})
	}
	wg.Wait()
	close(results)
	approved := 0
	for result := range results {
		if result.Status == "approved" {
			approved++
			require.NotEmpty(t, result.AccessToken)
		} else {
			require.Equal(t, "expired", result.Status)
			require.Empty(t, result.AccessToken)
		}
	}
	require.Equal(t, 1, approved)
	var count int64
	require.NoError(t, f.db.Model(&models.PersonalAccessToken{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestDeviceTokenInsertionFailureRollsBackConsumption(t *testing.T) {
	f := newDeviceTestFixture(t)
	grant := f.start(t)
	f.decide(t, grant.UserCode, true, "", http.StatusOK)
	require.NoError(t, f.db.Migrator().DropTable(&models.PersonalAccessToken{}))
	response, err := deviceRequest(f.url, "/auth/device/token", "", map[string]string{"device_code": grant.DeviceCode})
	require.NoError(t, err)
	readDeviceResponse[any](t, response, http.StatusInternalServerError)
	var stored models.DeviceAuthorization
	require.NoError(t, f.db.First(&stored).Error)
	require.Equal(t, "approved", stored.Status)
	require.Nil(t, stored.LastPolledAt)
	require.NoError(t, f.db.AutoMigrate(&models.PersonalAccessToken{}))
	require.Equal(t, "approved", f.poll(t, grant.DeviceCode).Status)
}

func TestDeviceConcurrentApprovalsCannotChangeIdentity(t *testing.T) {
	f := newDeviceTestFixture(t)
	grant := f.start(t)
	other := models.User{Name: "Other user", Email: "other@example.test", Password: "unused"}
	require.NoError(t, f.db.Create(&other).Error)
	require.NoError(t, f.db.Create(&models.Session{ID: "other-session", UserID: &other.ID, Payload: "", LastActivity: int(time.Now().Unix())}).Error)
	otherJWT := deviceTestJWT(t, other.ID, "other-session", nil)
	type result struct {
		userID string
		status int
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, contender := range []struct{ userID, token string }{{f.user.ID, f.jwt}, {other.ID, otherJWT}} {
		wg.Go(func() {
			response, err := deviceRequest(f.url, "/auth/device/authorize", contender.token, map[string]any{"user_code": grant.UserCode, "approve": true})
			if err != nil {
				t.Error(err)
				return
			}
			if err := response.Body.Close(); err != nil {
				t.Error(err)
			}
			results <- result{contender.userID, response.StatusCode}
		})
	}
	wg.Wait()
	close(results)
	winner := ""
	for result := range results {
		if result.status == http.StatusOK {
			require.Empty(t, winner)
			winner = result.userID
		} else {
			require.Equal(t, http.StatusConflict, result.status)
		}
	}
	require.NotEmpty(t, winner)
	var stored models.DeviceAuthorization
	require.NoError(t, f.db.First(&stored).Error)
	require.Equal(t, winner, *stored.UserID)
	require.Equal(t, winner, f.poll(t, grant.DeviceCode).User.ID)
}

func TestDeviceUnknownCodeAttemptsAreLimitedPerUser(t *testing.T) {
	f := newDeviceTestFixture(t)
	for range 10 {
		f.decide(t, "00000000", true, "", http.StatusUnprocessableEntity)
	}
	f.decide(t, "00000000", true, "", http.StatusTooManyRequests)
}

func TestDeviceRecoveryCodeCannotApproveTwoGrantsConcurrentlyOnPostgres(t *testing.T) {
	if os.Getenv("DEVICE_AUTH_TEST_DSN") == "" {
		t.Skip("set DEVICE_AUTH_TEST_DSN to a disposable PostgreSQL database to verify row locking")
	}
	f := newDeviceTestFixture(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("single-use-recovery"), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&f.user).Updates(map[string]any{
		"two_factor_secret": "JBSWY3DPEHPK3PXP", "two_factor_confirmed_at": f.now, "two_factor_recovery_codes": string(hash),
	}).Error)
	grants := []deviceCodeResponse{f.start(t), f.start(t)}
	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, grant := range grants {
		wg.Go(func() {
			<-start
			response, err := deviceRequest(f.url, "/auth/device/authorize", f.jwt, map[string]any{
				"user_code": grant.UserCode, "approve": true, "code": "single-use-recovery",
			})
			if err != nil {
				t.Error(err)
				return
			}
			if err := response.Body.Close(); err != nil {
				t.Error(err)
			}
			statuses <- response.StatusCode
		})
	}
	close(start)
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	require.Equal(t, map[int]int{http.StatusOK: 1, http.StatusUnprocessableEntity: 1}, counts)
	var approved int64
	require.NoError(t, f.db.Model(&models.DeviceAuthorization{}).Where("status = ?", "approved").Count(&approved).Error)
	require.EqualValues(t, 1, approved)
	var user models.User
	require.NoError(t, f.db.First(&user, "id = ?", f.user.ID).Error)
	require.NotNil(t, user.TwoFactorRecoveryCodes)
	require.Empty(t, *user.TwoFactorRecoveryCodes)
}

func TestDeviceRateLimitCountsConcurrentRequests(t *testing.T) {
	f := newDeviceTestFixture(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for range 8 {
		wg.Go(func() {
			response, err := deviceRequest(f.url, "/auth/device/code", "", nil)
			if err != nil {
				t.Error(err)
				return
			}
			if err := response.Body.Close(); err != nil {
				t.Error(err)
			}
			statuses <- response.StatusCode
		})
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	require.Equal(t, map[int]int{http.StatusOK: 5, http.StatusTooManyRequests: 3}, counts)
}
