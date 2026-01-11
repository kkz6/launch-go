package billing

import (
	"testing"
)

func TestSubscriptionStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   string
	}{
		{"on_trial", SubscriptionStatusOnTrial, "on_trial"},
		{"active", SubscriptionStatusActive, "active"},
		{"paused", SubscriptionStatusPaused, "paused"},
		{"past_due", SubscriptionStatusPastDue, "past_due"},
		{"unpaid", SubscriptionStatusUnpaid, "unpaid"},
		{"cancelled", SubscriptionStatusCancelled, "cancelled"},
		{"expired", SubscriptionStatusExpired, "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("SubscriptionStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   bool
	}{
		{"on_trial is valid", SubscriptionStatusOnTrial, true},
		{"active is valid", SubscriptionStatusActive, true},
		{"paused is valid", SubscriptionStatusPaused, true},
		{"past_due is valid", SubscriptionStatusPastDue, true},
		{"unpaid is valid", SubscriptionStatusUnpaid, true},
		{"cancelled is valid", SubscriptionStatusCancelled, true},
		{"expired is valid", SubscriptionStatusExpired, true},
		{"invalid status", SubscriptionStatus("invalid"), false},
		{"empty status", SubscriptionStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("SubscriptionStatus.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionStatus_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   bool
	}{
		{"active is active", SubscriptionStatusActive, true},
		{"on_trial is active", SubscriptionStatusOnTrial, true},
		{"paused is not active", SubscriptionStatusPaused, false},
		{"past_due is not active", SubscriptionStatusPastDue, false},
		{"cancelled is not active", SubscriptionStatusCancelled, false},
		{"expired is not active", SubscriptionStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsActive(); got != tt.want {
				t.Errorf("SubscriptionStatus.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionStatus_IsCancelled(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   bool
	}{
		{"cancelled is cancelled", SubscriptionStatusCancelled, true},
		{"active is not cancelled", SubscriptionStatusActive, false},
		{"expired is not cancelled", SubscriptionStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsCancelled(); got != tt.want {
				t.Errorf("SubscriptionStatus.IsCancelled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionStatus_IsPaused(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   bool
	}{
		{"paused is paused", SubscriptionStatusPaused, true},
		{"active is not paused", SubscriptionStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsPaused(); got != tt.want {
				t.Errorf("SubscriptionStatus.IsPaused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionStatus_IsOnTrial(t *testing.T) {
	tests := []struct {
		name   string
		status SubscriptionStatus
		want   bool
	}{
		{"on_trial is on trial", SubscriptionStatusOnTrial, true},
		{"active is not on trial", SubscriptionStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsOnTrial(); got != tt.want {
				t.Errorf("SubscriptionStatus.IsOnTrial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllSubscriptionStatuses(t *testing.T) {
	statuses := AllSubscriptionStatuses()
	if len(statuses) != 7 {
		t.Errorf("AllSubscriptionStatuses() returned %d statuses, want 7", len(statuses))
	}

	for _, status := range statuses {
		if !status.IsValid() {
			t.Errorf("AllSubscriptionStatuses() returned invalid status: %v", status)
		}
	}
}

func TestPlanInterval_String(t *testing.T) {
	tests := []struct {
		name     string
		interval PlanInterval
		want     string
	}{
		{"monthly", PlanIntervalMonthly, "monthly"},
		{"yearly", PlanIntervalYearly, "yearly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.interval.String(); got != tt.want {
				t.Errorf("PlanInterval.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanInterval_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		interval PlanInterval
		want     bool
	}{
		{"monthly is valid", PlanIntervalMonthly, true},
		{"yearly is valid", PlanIntervalYearly, true},
		{"invalid interval", PlanInterval("weekly"), false},
		{"empty interval", PlanInterval(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.interval.IsValid(); got != tt.want {
				t.Errorf("PlanInterval.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanInterval_IsMonthly(t *testing.T) {
	tests := []struct {
		name     string
		interval PlanInterval
		want     bool
	}{
		{"monthly is monthly", PlanIntervalMonthly, true},
		{"yearly is not monthly", PlanIntervalYearly, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.interval.IsMonthly(); got != tt.want {
				t.Errorf("PlanInterval.IsMonthly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanInterval_IsYearly(t *testing.T) {
	tests := []struct {
		name     string
		interval PlanInterval
		want     bool
	}{
		{"yearly is yearly", PlanIntervalYearly, true},
		{"monthly is not yearly", PlanIntervalMonthly, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.interval.IsYearly(); got != tt.want {
				t.Errorf("PlanInterval.IsYearly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllPlanIntervals(t *testing.T) {
	intervals := AllPlanIntervals()
	if len(intervals) != 2 {
		t.Errorf("AllPlanIntervals() returned %d intervals, want 2", len(intervals))
	}

	for _, interval := range intervals {
		if !interval.IsValid() {
			t.Errorf("AllPlanIntervals() returned invalid interval: %v", interval)
		}
	}
}

func TestOrderStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status OrderStatus
		want   string
	}{
		{"pending", OrderStatusPending, "pending"},
		{"paid", OrderStatusPaid, "paid"},
		{"failed", OrderStatusFailed, "failed"},
		{"refunded", OrderStatusRefunded, "refunded"},
		{"disputed", OrderStatusDisputed, "disputed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("OrderStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status OrderStatus
		want   bool
	}{
		{"pending is valid", OrderStatusPending, true},
		{"paid is valid", OrderStatusPaid, true},
		{"failed is valid", OrderStatusFailed, true},
		{"refunded is valid", OrderStatusRefunded, true},
		{"disputed is valid", OrderStatusDisputed, true},
		{"invalid status", OrderStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("OrderStatus.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderStatus_IsPaid(t *testing.T) {
	tests := []struct {
		name   string
		status OrderStatus
		want   bool
	}{
		{"paid is paid", OrderStatusPaid, true},
		{"pending is not paid", OrderStatusPending, false},
		{"failed is not paid", OrderStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsPaid(); got != tt.want {
				t.Errorf("OrderStatus.IsPaid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllOrderStatuses(t *testing.T) {
	statuses := AllOrderStatuses()
	if len(statuses) != 5 {
		t.Errorf("AllOrderStatuses() returned %d statuses, want 5", len(statuses))
	}

	for _, status := range statuses {
		if !status.IsValid() {
			t.Errorf("AllOrderStatuses() returned invalid status: %v", status)
		}
	}
}

func TestWebhookEventType_String(t *testing.T) {
	tests := []struct {
		name  string
		event WebhookEventType
		want  string
	}{
		{"subscription_created", WebhookEventSubscriptionCreated, "subscription_created"},
		{"subscription_updated", WebhookEventSubscriptionUpdated, "subscription_updated"},
		{"order_created", WebhookEventOrderCreated, "order_created"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.String(); got != tt.want {
				t.Errorf("WebhookEventType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookEventType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		event WebhookEventType
		want  bool
	}{
		{"subscription_created is valid", WebhookEventSubscriptionCreated, true},
		{"subscription_updated is valid", WebhookEventSubscriptionUpdated, true},
		{"subscription_cancelled is valid", WebhookEventSubscriptionCancelled, true},
		{"subscription_resumed is valid", WebhookEventSubscriptionResumed, true},
		{"subscription_expired is valid", WebhookEventSubscriptionExpired, true},
		{"subscription_paused is valid", WebhookEventSubscriptionPaused, true},
		{"subscription_unpaused is valid", WebhookEventSubscriptionUnpaused, true},
		{"subscription_payment_success is valid", WebhookEventSubscriptionPaymentSuccess, true},
		{"subscription_payment_failed is valid", WebhookEventSubscriptionPaymentFailed, true},
		{"subscription_payment_recovered is valid", WebhookEventSubscriptionPaymentRecovered, true},
		{"order_created is valid", WebhookEventOrderCreated, true},
		{"order_refunded is valid", WebhookEventOrderRefunded, true},
		{"invalid event", WebhookEventType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsValid(); got != tt.want {
				t.Errorf("WebhookEventType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookEventType_IsSubscriptionEvent(t *testing.T) {
	tests := []struct {
		name  string
		event WebhookEventType
		want  bool
	}{
		{"subscription_created is subscription event", WebhookEventSubscriptionCreated, true},
		{"subscription_updated is subscription event", WebhookEventSubscriptionUpdated, true},
		{"subscription_cancelled is subscription event", WebhookEventSubscriptionCancelled, true},
		{"subscription_payment_success is subscription event", WebhookEventSubscriptionPaymentSuccess, true},
		{"order_created is not subscription event", WebhookEventOrderCreated, false},
		{"order_refunded is not subscription event", WebhookEventOrderRefunded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsSubscriptionEvent(); got != tt.want {
				t.Errorf("WebhookEventType.IsSubscriptionEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookEventType_IsOrderEvent(t *testing.T) {
	tests := []struct {
		name  string
		event WebhookEventType
		want  bool
	}{
		{"order_created is order event", WebhookEventOrderCreated, true},
		{"order_refunded is order event", WebhookEventOrderRefunded, true},
		{"subscription_created is not order event", WebhookEventSubscriptionCreated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsOrderEvent(); got != tt.want {
				t.Errorf("WebhookEventType.IsOrderEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllWebhookEventTypes(t *testing.T) {
	events := AllWebhookEventTypes()
	if len(events) != 12 {
		t.Errorf("AllWebhookEventTypes() returned %d events, want 12", len(events))
	}

	for _, event := range events {
		if !event.IsValid() {
			t.Errorf("AllWebhookEventTypes() returned invalid event: %v", event)
		}
	}
}

func TestUserRole_String(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want string
	}{
		{"customer", UserRoleCustomer, "customer"},
		{"manager", UserRoleManager, "manager"},
		{"admin", UserRoleAdmin, "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("UserRole.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserRole_IsValid(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want bool
	}{
		{"customer is valid", UserRoleCustomer, true},
		{"manager is valid", UserRoleManager, true},
		{"admin is valid", UserRoleAdmin, true},
		{"invalid role", UserRole("superuser"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.want {
				t.Errorf("UserRole.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserRole_IsAdmin(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want bool
	}{
		{"admin is admin", UserRoleAdmin, true},
		{"manager is admin", UserRoleManager, true},
		{"customer is not admin", UserRoleCustomer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.IsAdmin(); got != tt.want {
				t.Errorf("UserRole.IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllUserRoles(t *testing.T) {
	roles := AllUserRoles()
	if len(roles) != 3 {
		t.Errorf("AllUserRoles() returned %d roles, want 3", len(roles))
	}

	for _, role := range roles {
		if !role.IsValid() {
			t.Errorf("AllUserRoles() returned invalid role: %v", role)
		}
	}
}
