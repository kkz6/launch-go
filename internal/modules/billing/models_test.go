package billing

import (
	"testing"
	"time"
)

func TestSubscription_IsActive(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		subscription *Subscription
		want         bool
	}{
		{
			name: "active status is active",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
			},
			want: true,
		},
		{
			name: "on_trial with future trial_ends_at is active",
			subscription: &Subscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: &future,
			},
			want: true,
		},
		{
			name: "on_trial with past trial_ends_at is not active",
			subscription: &Subscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: &past,
			},
			want: false,
		},
		{
			name: "on_trial with nil trial_ends_at is not active",
			subscription: &Subscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: nil,
			},
			want: false,
		},
		{
			name: "cancelled status is not active",
			subscription: &Subscription{
				Status: SubscriptionStatusCancelled,
			},
			want: false,
		},
		{
			name: "paused status is not active",
			subscription: &Subscription{
				Status: SubscriptionStatusPaused,
			},
			want: false,
		},
		{
			name: "expired status is not active",
			subscription: &Subscription{
				Status: SubscriptionStatusExpired,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.subscription.IsActive(); got != tt.want {
				t.Errorf("Subscription.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscription_OnTrial(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		subscription *Subscription
		want         bool
	}{
		{
			name: "on_trial with future trial_ends_at is on trial",
			subscription: &Subscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: &future,
			},
			want: true,
		},
		{
			name: "on_trial with past trial_ends_at is not on trial",
			subscription: &Subscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: &past,
			},
			want: false,
		},
		{
			name: "on_trial with nil trial_ends_at is not on trial",
			subscription: &Subscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: nil,
			},
			want: false,
		},
		{
			name: "active status is not on trial",
			subscription: &Subscription{
				Status:      SubscriptionStatusActive,
				TrialEndsAt: &future,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.subscription.OnTrial(); got != tt.want {
				t.Errorf("Subscription.OnTrial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscription_OnGracePeriod(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		subscription *Subscription
		want         bool
	}{
		{
			name: "cancelled with future ends_at is on grace period",
			subscription: &Subscription{
				Status: SubscriptionStatusCancelled,
				EndsAt: &future,
			},
			want: true,
		},
		{
			name: "cancelled with past ends_at is not on grace period",
			subscription: &Subscription{
				Status: SubscriptionStatusCancelled,
				EndsAt: &past,
			},
			want: false,
		},
		{
			name: "cancelled with nil ends_at is not on grace period",
			subscription: &Subscription{
				Status: SubscriptionStatusCancelled,
				EndsAt: nil,
			},
			want: false,
		},
		{
			name: "active with ends_at is not on grace period",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
				EndsAt: &future,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.subscription.OnGracePeriod(); got != tt.want {
				t.Errorf("Subscription.OnGracePeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscription_IsCancelled(t *testing.T) {
	tests := []struct {
		name         string
		subscription *Subscription
		want         bool
	}{
		{
			name: "cancelled status is cancelled",
			subscription: &Subscription{
				Status: SubscriptionStatusCancelled,
			},
			want: true,
		},
		{
			name: "active status is not cancelled",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.subscription.IsCancelled(); got != tt.want {
				t.Errorf("Subscription.IsCancelled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscription_IsPaused(t *testing.T) {
	tests := []struct {
		name         string
		subscription *Subscription
		want         bool
	}{
		{
			name: "paused status is paused",
			subscription: &Subscription{
				Status: SubscriptionStatusPaused,
			},
			want: true,
		},
		{
			name: "active status is not paused",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.subscription.IsPaused(); got != tt.want {
				t.Errorf("Subscription.IsPaused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscription_HasExpired(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		subscription *Subscription
		want         bool
	}{
		{
			name: "expired status has expired",
			subscription: &Subscription{
				Status: SubscriptionStatusExpired,
			},
			want: true,
		},
		{
			name: "active with past ends_at has expired",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
				EndsAt: &past,
			},
			want: true,
		},
		{
			name: "active with future ends_at has not expired",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
				EndsAt: &future,
			},
			want: false,
		},
		{
			name: "active with nil ends_at has not expired",
			subscription: &Subscription{
				Status: SubscriptionStatusActive,
				EndsAt: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.subscription.HasExpired(); got != tt.want {
				t.Errorf("Subscription.HasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrder_IsPaid(t *testing.T) {
	tests := []struct {
		name  string
		order *Order
		want  bool
	}{
		{
			name:  "paid status is paid",
			order: &Order{Status: OrderStatusPaid},
			want:  true,
		},
		{
			name:  "pending status is not paid",
			order: &Order{Status: OrderStatusPending},
			want:  false,
		},
		{
			name:  "refunded status is not paid",
			order: &Order{Status: OrderStatusRefunded},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.order.IsPaid(); got != tt.want {
				t.Errorf("Order.IsPaid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrder_IsRefunded(t *testing.T) {
	tests := []struct {
		name  string
		order *Order
		want  bool
	}{
		{
			name:  "refunded status is refunded",
			order: &Order{Status: OrderStatusRefunded},
			want:  true,
		},
		{
			name:  "paid status is not refunded",
			order: &Order{Status: OrderStatusPaid},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.order.IsRefunded(); got != tt.want {
				t.Errorf("Order.IsRefunded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrder_FormattedAmounts(t *testing.T) {
	order := &Order{
		Subtotal:      1999,
		DiscountTotal: 200,
		Tax:           100,
		Total:         1899,
	}

	if got := order.FormattedSubtotal(); got != 19.99 {
		t.Errorf("Order.FormattedSubtotal() = %v, want 19.99", got)
	}

	if got := order.FormattedDiscount(); got != 2.00 {
		t.Errorf("Order.FormattedDiscount() = %v, want 2.00", got)
	}

	if got := order.FormattedTax(); got != 1.00 {
		t.Errorf("Order.FormattedTax() = %v, want 1.00", got)
	}

	if got := order.FormattedTotal(); got != 18.99 {
		t.Errorf("Order.FormattedTotal() = %v, want 18.99", got)
	}
}

func TestWebhookEvent_MarkProcessed(t *testing.T) {
	event := &WebhookEvent{
		Processed: false,
	}

	event.MarkProcessed()

	if !event.Processed {
		t.Error("WebhookEvent.MarkProcessed() should set Processed to true")
	}

	if event.ProcessedAt == nil {
		t.Error("WebhookEvent.MarkProcessed() should set ProcessedAt")
	}
}

func TestWebhookEvent_MarkFailed(t *testing.T) {
	event := &WebhookEvent{
		RetryCount: 0,
	}

	event.MarkFailed("test error")

	if event.Error == nil || *event.Error != "test error" {
		t.Error("WebhookEvent.MarkFailed() should set Error")
	}

	if event.RetryCount != 1 {
		t.Error("WebhookEvent.MarkFailed() should increment RetryCount")
	}

	event.MarkFailed("another error")

	if event.RetryCount != 2 {
		t.Error("WebhookEvent.MarkFailed() should increment RetryCount again")
	}
}

func TestWebhookEvent_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		event      *WebhookEvent
		maxRetries int
		want       bool
	}{
		{
			name: "unprocessed with retries remaining can retry",
			event: &WebhookEvent{
				Processed:  false,
				RetryCount: 1,
			},
			maxRetries: 3,
			want:       true,
		},
		{
			name: "unprocessed at max retries cannot retry",
			event: &WebhookEvent{
				Processed:  false,
				RetryCount: 3,
			},
			maxRetries: 3,
			want:       false,
		},
		{
			name: "processed cannot retry",
			event: &WebhookEvent{
				Processed:  true,
				RetryCount: 0,
			},
			maxRetries: 3,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.CanRetry(tt.maxRetries); got != tt.want {
				t.Errorf("WebhookEvent.CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTeamSubscription_IsSubscribed(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name string
		ts   *TeamSubscription
		want bool
	}{
		{
			name: "active status is subscribed",
			ts: &TeamSubscription{
				Status: SubscriptionStatusActive,
			},
			want: true,
		},
		{
			name: "on_trial with future trial_ends_at is subscribed",
			ts: &TeamSubscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: &future,
			},
			want: true,
		},
		{
			name: "on_trial with past trial_ends_at is not subscribed",
			ts: &TeamSubscription{
				Status:      SubscriptionStatusOnTrial,
				TrialEndsAt: &past,
			},
			want: false,
		},
		{
			name: "cancelled with future ends_at is subscribed (grace period)",
			ts: &TeamSubscription{
				Status: SubscriptionStatusCancelled,
				EndsAt: &future,
			},
			want: true,
		},
		{
			name: "cancelled with past ends_at is not subscribed",
			ts: &TeamSubscription{
				Status: SubscriptionStatusCancelled,
				EndsAt: &past,
			},
			want: false,
		},
		{
			name: "expired status is not subscribed",
			ts: &TeamSubscription{
				Status: SubscriptionStatusExpired,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ts.IsSubscribed(); got != tt.want {
				t.Errorf("TeamSubscription.IsSubscribed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanOptions(t *testing.T) {
	plan := &Plan{
		ID:   "test",
		Name: "Test Plan",
		Options: PlanOptions{
			MaxServers:            10,
			MaxSitesPerServer:     20,
			MaxDeploymentsPerSite: 5,
			MaxTeamMembers:        3,
			HasBackups:            true,
			HasMonitoring:         true,
		},
	}

	if plan.Options.MaxServers != 10 {
		t.Errorf("PlanOptions.MaxServers = %v, want 10", plan.Options.MaxServers)
	}

	if plan.Options.MaxSitesPerServer != 20 {
		t.Errorf("PlanOptions.MaxSitesPerServer = %v, want 20", plan.Options.MaxSitesPerServer)
	}

	if plan.Options.MaxDeploymentsPerSite != 5 {
		t.Errorf("PlanOptions.MaxDeploymentsPerSite = %v, want 5", plan.Options.MaxDeploymentsPerSite)
	}

	if plan.Options.MaxTeamMembers != 3 {
		t.Errorf("PlanOptions.MaxTeamMembers = %v, want 3", plan.Options.MaxTeamMembers)
	}

	if !plan.Options.HasBackups {
		t.Error("PlanOptions.HasBackups should be true")
	}

	if !plan.Options.HasMonitoring {
		t.Error("PlanOptions.HasMonitoring should be true")
	}
}
