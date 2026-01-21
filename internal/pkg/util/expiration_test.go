package util

import (
	"testing"
	"time"
)

func TestExpiresAt(t *testing.T) {
	expirationTime := time.Now().Add(24 * time.Hour)
	exp := ExpiresAt(expirationTime)

	if !exp.Time().Equal(expirationTime) {
		t.Errorf("expected time %v, got %v", expirationTime, exp.Time())
	}
}

func TestExpiresAtPtr(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		exp := ExpiresAtPtr(nil)

		if exp != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("non-nil input returns expiration", func(t *testing.T) {
		expirationTime := time.Now().Add(24 * time.Hour)
		exp := ExpiresAtPtr(&expirationTime)

		if exp == nil {
			t.Fatal("expected non-nil expiration")
		}

		if !exp.Time().Equal(expirationTime) {
			t.Errorf("expected time %v, got %v", expirationTime, exp.Time())
		}
	})
}

func TestIsExpired(t *testing.T) {
	t.Run("future time is not expired", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(24 * time.Hour))

		if exp.IsExpired() {
			t.Error("expected future time to not be expired")
		}
	})

	t.Run("past time is expired", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-24 * time.Hour))

		if !exp.IsExpired() {
			t.Error("expected past time to be expired")
		}
	})
}

func TestIsExpiredWithGrace(t *testing.T) {
	t.Run("expired but within grace period", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-1 * time.Hour))
		grace := 2 * time.Hour

		if exp.IsExpiredWithGrace(grace) {
			t.Error("expected to not be expired within grace period")
		}
	})

	t.Run("expired and past grace period", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-3 * time.Hour))
		grace := 2 * time.Hour

		if !exp.IsExpiredWithGrace(grace) {
			t.Error("expected to be expired past grace period")
		}
	})

	t.Run("not expired ignores grace period", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(1 * time.Hour))
		grace := 2 * time.Hour

		if exp.IsExpiredWithGrace(grace) {
			t.Error("expected not expired time to remain not expired")
		}
	})
}

func TestExpiresWithin(t *testing.T) {
	t.Run("expires within duration", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(12 * time.Hour))

		if !exp.ExpiresWithin(24 * time.Hour) {
			t.Error("expected to expire within 24 hours")
		}
	})

	t.Run("does not expire within duration", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(48 * time.Hour))

		if exp.ExpiresWithin(24 * time.Hour) {
			t.Error("expected to not expire within 24 hours")
		}
	})

	t.Run("already expired returns false", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-1 * time.Hour))

		if exp.ExpiresWithin(24 * time.Hour) {
			t.Error("expected already expired to return false")
		}
	})
}

func TestTimeRemaining(t *testing.T) {
	t.Run("positive remaining time", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(24 * time.Hour))
		remaining := exp.TimeRemaining()

		if remaining <= 0 {
			t.Error("expected positive remaining time")
		}

		if remaining > 24*time.Hour {
			t.Error("expected remaining time to be less than or equal to 24 hours")
		}
	})

	t.Run("negative remaining time for expired", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-24 * time.Hour))
		remaining := exp.TimeRemaining()

		if remaining >= 0 {
			t.Error("expected negative remaining time for expired")
		}
	})
}

func TestDaysRemaining(t *testing.T) {
	t.Run("multiple days remaining", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(72 * time.Hour))
		days := exp.DaysRemaining()

		if days < 2 || days > 3 {
			t.Errorf("expected 2-3 days remaining, got %d", days)
		}
	})

	t.Run("less than one day remaining", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(12 * time.Hour))
		days := exp.DaysRemaining()

		if days != 0 {
			t.Errorf("expected 0 days remaining, got %d", days)
		}
	})

	t.Run("negative days for expired", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-48 * time.Hour))
		days := exp.DaysRemaining()

		if days >= 0 {
			t.Errorf("expected negative days, got %d", days)
		}
	})
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	expectedCritical := 7 * 24 * time.Hour
	expectedWarning := 30 * 24 * time.Hour

	if thresholds.Critical != expectedCritical {
		t.Errorf("expected critical threshold %v, got %v", expectedCritical, thresholds.Critical)
	}

	if thresholds.Warning != expectedWarning {
		t.Errorf("expected warning threshold %v, got %v", expectedWarning, thresholds.Warning)
	}
}

func TestStatus(t *testing.T) {
	thresholds := DefaultThresholds()

	t.Run("expired status", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(-1 * time.Hour))
		status := exp.Status(thresholds)

		if status != StatusExpired {
			t.Errorf("expected status %q, got %q", StatusExpired, status)
		}
	})

	t.Run("critical status", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(3 * 24 * time.Hour))
		status := exp.Status(thresholds)

		if status != StatusCritical {
			t.Errorf("expected status %q, got %q", StatusCritical, status)
		}
	})

	t.Run("warning status", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(15 * 24 * time.Hour))
		status := exp.Status(thresholds)

		if status != StatusWarning {
			t.Errorf("expected status %q, got %q", StatusWarning, status)
		}
	})

	t.Run("valid status", func(t *testing.T) {
		exp := ExpiresAt(time.Now().Add(60 * 24 * time.Hour))
		status := exp.Status(thresholds)

		if status != StatusValid {
			t.Errorf("expected status %q, got %q", StatusValid, status)
		}
	})

	t.Run("custom thresholds", func(t *testing.T) {
		customThresholds := StatusThresholds{
			Critical: 1 * 24 * time.Hour,
			Warning:  3 * 24 * time.Hour,
		}

		exp := ExpiresAt(time.Now().Add(2 * 24 * time.Hour))
		status := exp.Status(customThresholds)

		if status != StatusWarning {
			t.Errorf("expected status %q with custom thresholds, got %q", StatusWarning, status)
		}
	})
}

func TestSubscriptionStatus(t *testing.T) {
	t.Run("trialing status", func(t *testing.T) {
		trialEnds := time.Now().Add(7 * 24 * time.Hour)
		sub := SubscriptionStatus{
			TrialEndsAt: &trialEnds,
		}

		status := sub.Status()

		if status != SubscriptionTrialing {
			t.Errorf("expected status %q, got %q", SubscriptionTrialing, status)
		}
	})

	t.Run("trialing with future end date", func(t *testing.T) {
		trialEnds := time.Now().Add(7 * 24 * time.Hour)
		endsAt := time.Now().Add(30 * 24 * time.Hour)
		sub := SubscriptionStatus{
			TrialEndsAt: &trialEnds,
			EndsAt:      &endsAt,
		}

		status := sub.Status()

		if status != SubscriptionTrialing {
			t.Errorf("expected status %q, got %q", SubscriptionTrialing, status)
		}
	})

	t.Run("active status with no end date", func(t *testing.T) {
		sub := SubscriptionStatus{}

		status := sub.Status()

		if status != SubscriptionActive {
			t.Errorf("expected status %q, got %q", SubscriptionActive, status)
		}
	})

	t.Run("active status with far future end date", func(t *testing.T) {
		endsAt := time.Now().Add(60 * 24 * time.Hour)
		sub := SubscriptionStatus{
			EndsAt: &endsAt,
		}

		status := sub.Status()

		if status != SubscriptionActive {
			t.Errorf("expected status %q, got %q", SubscriptionActive, status)
		}
	})

	t.Run("expiring soon status", func(t *testing.T) {
		endsAt := time.Now().Add(3 * 24 * time.Hour)
		sub := SubscriptionStatus{
			EndsAt: &endsAt,
		}

		status := sub.Status()

		if status != SubscriptionExpiringSoon {
			t.Errorf("expected status %q, got %q", SubscriptionExpiringSoon, status)
		}
	})

	t.Run("grace period status", func(t *testing.T) {
		endsAt := time.Now().Add(-1 * 24 * time.Hour)
		sub := SubscriptionStatus{
			EndsAt:      &endsAt,
			GracePeriod: 3 * 24 * time.Hour,
		}

		status := sub.Status()

		if status != SubscriptionGracePeriod {
			t.Errorf("expected status %q, got %q", SubscriptionGracePeriod, status)
		}
	})

	t.Run("expired status no grace period", func(t *testing.T) {
		endsAt := time.Now().Add(-1 * 24 * time.Hour)
		sub := SubscriptionStatus{
			EndsAt: &endsAt,
		}

		status := sub.Status()

		if status != SubscriptionExpired {
			t.Errorf("expected status %q, got %q", SubscriptionExpired, status)
		}
	})

	t.Run("expired status past grace period", func(t *testing.T) {
		endsAt := time.Now().Add(-5 * 24 * time.Hour)
		sub := SubscriptionStatus{
			EndsAt:      &endsAt,
			GracePeriod: 3 * 24 * time.Hour,
		}

		status := sub.Status()

		if status != SubscriptionExpired {
			t.Errorf("expected status %q, got %q", SubscriptionExpired, status)
		}
	})

	t.Run("trial ended transitions to active", func(t *testing.T) {
		trialEnds := time.Now().Add(-1 * 24 * time.Hour)
		endsAt := time.Now().Add(30 * 24 * time.Hour)
		sub := SubscriptionStatus{
			TrialEndsAt: &trialEnds,
			EndsAt:      &endsAt,
		}

		status := sub.Status()

		if status != SubscriptionActive {
			t.Errorf("expected status %q after trial ended, got %q", SubscriptionActive, status)
		}
	})

	t.Run("trial ended transitions to expiring soon", func(t *testing.T) {
		trialEnds := time.Now().Add(-1 * 24 * time.Hour)
		endsAt := time.Now().Add(3 * 24 * time.Hour)
		sub := SubscriptionStatus{
			TrialEndsAt: &trialEnds,
			EndsAt:      &endsAt,
		}

		status := sub.Status()

		if status != SubscriptionExpiringSoon {
			t.Errorf("expected status %q after trial ended, got %q", SubscriptionExpiringSoon, status)
		}
	})
}
