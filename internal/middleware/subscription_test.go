package middleware

import (
	"context"
	"github.com/kkz6/launch-go/tests/testutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authrepos "github.com/kkz6/launch-go/internal/modules/auth/repositories"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingrepos "github.com/kkz6/launch-go/internal/modules/billing/repositories"
)

// setupSubscriptionDB returns an in-memory sqlite DB holding the minimal
// `users` and `subscriptions` tables that the subscription middleware queries.
// Eligibility queries use string columns and the cancellation end timestamp.
func setupSubscriptionDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := testutil.SQLite(t)
	t.Cleanup(func() { InitSubscriptionMiddleware(nil, false) })

	require.NoError(t, db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		staff_role TEXT
	)`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		billable_id TEXT,
		billable_type TEXT,
		status TEXT,
		ends_at DATETIME
	)`).Error)

	return db
}

func seedUser(t *testing.T, db *gorm.DB, id string, staffRole *string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, staff_role) VALUES (?, ?)`, id, staffRole,
	).Error)
}

func seedSubscription(t *testing.T, db *gorm.DB, teamID, status string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO subscriptions (billable_id, billable_type, status) VALUES (?, ?, ?)`,
		teamID, "Modules\\Auth\\Models\\Team", status,
	).Error)
}

// appWithSubscription builds a tiny Fiber app that seeds the auth context
// (userID/teamID) the way the real Auth + TeamScope middleware would, then runs
// VerifySubscription as the route guard.
func appWithSubscription(userID, teamID string) *fiber.App {
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", userID)
		if teamID != "" {
			c.Locals("teamID", teamID)
		}
		return c.Next()
	})

	app.Get("/protected", VerifySubscription(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	return app
}

// TestVerifySubscription_StaffBypassRegression guards the staff_role repoint:
// VerifySubscription's admin bypass now reads users.staff_role (replacing the
// dropped Spatie model_has_roles lookup). These cases prove the bypass still
// fires for staff and that ordinary customers remain gated.
func TestVerifySubscription_StaffBypassRegression(t *testing.T) {
	superAdmin := "super_admin"
	support := "support"

	cases := []struct {
		name           string
		staffRole      *string
		teamSubscribed bool
		teamStatus     string
		wantStatus     int
	}{
		{
			name:       "super_admin staff bypasses unsubscribed team",
			staffRole:  &superAdmin,
			teamStatus: "canceled",
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "support staff bypasses unsubscribed team",
			staffRole:  &support,
			teamStatus: "canceled",
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "non-staff with unsubscribed team is blocked",
			staffRole:  nil,
			teamStatus: "canceled",
			wantStatus: fiber.StatusPaymentRequired,
		},
		{
			name:       "non-staff with active subscription is allowed",
			staffRole:  nil,
			teamStatus: "active",
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "non-staff on trial is allowed",
			staffRole:  nil,
			teamStatus: "on_trial",
			wantStatus: fiber.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSubscriptionDB(t)
			InitSubscriptionMiddleware(db, true)

			seedUser(t, db, "u1", tc.staffRole)
			seedSubscription(t, db, "team1", tc.teamStatus)

			app := appWithSubscription("u1", "team1")
			resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

// TestVerifySubscription_DisabledSkips confirms the global config switch still
// short-circuits the whole check regardless of staff_role / subscription state.
func TestVerifySubscription_DisabledSkips(t *testing.T) {
	db := setupSubscriptionDB(t)
	InitSubscriptionMiddleware(db, false)

	seedUser(t, db, "u1", nil)
	// No subscription seeded; with subscriptions disabled this must still pass.

	app := appWithSubscription("u1", "team1")
	resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestSeparationGuarantee exercises the two independent authorization axes
// together — the property staff_test.go can't show at the middleware level
// because it only tests one axis in isolation:
//
//   - Tenant axis: team_user.role / subscription state, enforced per-team.
//   - Staff axis:  users.staff_role, a product-wide back-office grant.
//
// The guarantee is that the axes don't leak into each other. A staff member
// needs NO team and NO subscription to clear a subscription-gated route (the
// staff grant alone suffices), while a paying-or-not customer's access is
// decided purely by their own subscription, never by any staff grant they
// don't have. VerifySubscription is the one middleware where both axes meet
// (admin bypass vs. team subscription), so it is the right place to pin the
// separation down end-to-end.
func TestSeparationGuarantee(t *testing.T) {
	support := "support"

	t.Run("staffer with no team and no subscription is admitted", func(t *testing.T) {
		db := setupSubscriptionDB(t)
		InitSubscriptionMiddleware(db, true)
		seedUser(t, db, "staff", &support)
		// Deliberately no team context and no subscription row.

		app := appWithSubscription("staff", "")
		resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode,
			"staff axis must clear the route without any tenant-axis state")
	})

	t.Run("customer is judged only by their own subscription", func(t *testing.T) {
		db := setupSubscriptionDB(t)
		InitSubscriptionMiddleware(db, true)
		seedUser(t, db, "customer", nil)
		seedSubscription(t, db, "teamA", "canceled")

		app := appWithSubscription("customer", "teamA")
		resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusPaymentRequired, resp.StatusCode,
			"a non-staff customer never inherits a staff bypass")
	})
}

func TestVerifySubscriptionMatchesBillingEligibility(t *testing.T) {
	past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	for _, billableType := range billingmodels.TeamBillableTypes() {
		t.Run(billableType, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				status     string
				endsAt     *time.Time
				subscribed bool
			}{
				{"active", "active", nil, true},
				{"trial", "on_trial", nil, true},
				{"cancelled within grace period", "cancelled", &future, true},
				{"cancelled after grace period", "cancelled", &past, false},
				{"cancelled without end date", "cancelled", nil, false},
				{"expired", "expired", &future, false},
				{"paused", "paused", &future, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					db := setupSubscriptionDB(t)
					InitSubscriptionMiddleware(db, true)
					seedUser(t, db, "customer", nil)
					require.NoError(t, db.Exec(
						"INSERT INTO subscriptions (billable_id, billable_type, status, ends_at) VALUES (?, ?, ?, ?)",
						"team-1", billableType, tc.status, tc.endsAt,
					).Error)
					subscribed, err := billingrepos.NewSubscriptionRepository(db).IsTeamSubscribed(context.Background(), "team-1")
					require.NoError(t, err)
					require.Equal(t, tc.subscribed, subscribed)
					require.Equal(t, tc.subscribed, authrepos.NewRegistry(db).IsTeamSubscribed(context.Background(), "team-1"),
						"user subscription flags must agree with billing and API access")

					response, err := appWithSubscription("customer", "team-1").Test(httptest.NewRequest("GET", "/protected", nil))
					require.NoError(t, err)
					defer response.Body.Close()
					wantStatus := fiber.StatusPaymentRequired
					if tc.subscribed {
						wantStatus = fiber.StatusOK
					}
					assert.Equal(t, wantStatus, response.StatusCode)
				})
			}
		})
	}
}

func TestVerifySubscriptionKeepsEligibilityTeamScoped(t *testing.T) {
	db := setupSubscriptionDB(t)
	InitSubscriptionMiddleware(db, true)
	seedUser(t, db, "customer", nil)
	seedSubscription(t, db, "other-team", "active")
	require.NoError(t, db.Exec(
		"INSERT INTO subscriptions (billable_id, billable_type, status, ends_at) VALUES (?, ?, ?, ?)",
		"team-1", "User", "cancelled", time.Now().Add(time.Hour),
	).Error)
	require.False(t, authrepos.NewRegistry(db).IsTeamSubscribed(context.Background(), "team-1"))
	response, err := appWithSubscription("customer", "team-1").Test(httptest.NewRequest("GET", "/protected", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, fiber.StatusPaymentRequired, response.StatusCode)
}

func TestVerifySubscriptionStorageErrorRemainsPaymentRequired(t *testing.T) {
	db := setupSubscriptionDB(t)
	InitSubscriptionMiddleware(db, true)
	seedUser(t, db, "customer", nil)
	require.NoError(t, db.Exec("DROP TABLE subscriptions").Error)
	subscribed, err := billingrepos.NewSubscriptionRepository(db).IsTeamSubscribed(context.Background(), "team-1")
	require.Error(t, err)
	require.False(t, subscribed)
	require.False(t, authrepos.NewRegistry(db).IsTeamSubscribed(context.Background(), "team-1"))
	response, err := appWithSubscription("customer", "team-1").Test(httptest.NewRequest("GET", "/protected", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, fiber.StatusPaymentRequired, response.StatusCode)
}
