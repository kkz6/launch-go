package billing

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
)

func TestSubscriptionStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status enums.SubscriptionStatus
		want   string
	}{
		{"on_trial", enums.SubscriptionStatusOnTrial, "on_trial"},
		{"active", enums.SubscriptionStatusActive, "active"},
		{"paused", enums.SubscriptionStatusPaused, "paused"},
		{"past_due", enums.SubscriptionStatusPastDue, "past_due"},
		{"unpaid", enums.SubscriptionStatusUnpaid, "unpaid"},
		{"cancelled", enums.SubscriptionStatusCancelled, "cancelled"},
		{"expired", enums.SubscriptionStatusExpired, "expired"},
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
		status enums.SubscriptionStatus
		want   bool
	}{
		{"on_trial is valid", enums.SubscriptionStatusOnTrial, true},
		{"active is valid", enums.SubscriptionStatusActive, true},
		{"paused is valid", enums.SubscriptionStatusPaused, true},
		{"past_due is valid", enums.SubscriptionStatusPastDue, true},
		{"unpaid is valid", enums.SubscriptionStatusUnpaid, true},
		{"cancelled is valid", enums.SubscriptionStatusCancelled, true},
		{"expired is valid", enums.SubscriptionStatusExpired, true},
		{"invalid status", enums.SubscriptionStatus("invalid"), false},
		{"empty status", enums.SubscriptionStatus(""), false},
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
		status enums.SubscriptionStatus
		want   bool
	}{
		{"active is active", enums.SubscriptionStatusActive, true},
		{"on_trial is active", enums.SubscriptionStatusOnTrial, true},
		{"paused is not active", enums.SubscriptionStatusPaused, false},
		{"past_due is not active", enums.SubscriptionStatusPastDue, false},
		{"cancelled is not active", enums.SubscriptionStatusCancelled, false},
		{"expired is not active", enums.SubscriptionStatusExpired, false},
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
		status enums.SubscriptionStatus
		want   bool
	}{
		{"cancelled is cancelled", enums.SubscriptionStatusCancelled, true},
		{"active is not cancelled", enums.SubscriptionStatusActive, false},
		{"expired is not cancelled", enums.SubscriptionStatusExpired, false},
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
		status enums.SubscriptionStatus
		want   bool
	}{
		{"paused is paused", enums.SubscriptionStatusPaused, true},
		{"active is not paused", enums.SubscriptionStatusActive, false},
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
		status enums.SubscriptionStatus
		want   bool
	}{
		{"on_trial is on trial", enums.SubscriptionStatusOnTrial, true},
		{"active is not on trial", enums.SubscriptionStatusActive, false},
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
	statuses := enums.AllSubscriptionStatuses()
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
		interval enums.PlanInterval
		want     string
	}{
		{"monthly", enums.PlanIntervalMonthly, "monthly"},
		{"yearly", enums.PlanIntervalYearly, "yearly"},
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
		interval enums.PlanInterval
		want     bool
	}{
		{"monthly is valid", enums.PlanIntervalMonthly, true},
		{"yearly is valid", enums.PlanIntervalYearly, true},
		{"invalid interval", enums.PlanInterval("weekly"), false},
		{"empty interval", enums.PlanInterval(""), false},
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
		interval enums.PlanInterval
		want     bool
	}{
		{"monthly is monthly", enums.PlanIntervalMonthly, true},
		{"yearly is not monthly", enums.PlanIntervalYearly, false},
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
		interval enums.PlanInterval
		want     bool
	}{
		{"yearly is yearly", enums.PlanIntervalYearly, true},
		{"monthly is not yearly", enums.PlanIntervalMonthly, false},
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
	intervals := enums.AllPlanIntervals()
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
		status enums.OrderStatus
		want   string
	}{
		{"pending", enums.OrderStatusPending, "pending"},
		{"paid", enums.OrderStatusPaid, "paid"},
		{"failed", enums.OrderStatusFailed, "failed"},
		{"refunded", enums.OrderStatusRefunded, "refunded"},
		{"disputed", enums.OrderStatusDisputed, "disputed"},
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
		status enums.OrderStatus
		want   bool
	}{
		{"pending is valid", enums.OrderStatusPending, true},
		{"paid is valid", enums.OrderStatusPaid, true},
		{"failed is valid", enums.OrderStatusFailed, true},
		{"refunded is valid", enums.OrderStatusRefunded, true},
		{"disputed is valid", enums.OrderStatusDisputed, true},
		{"invalid status", enums.OrderStatus("invalid"), false},
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
		status enums.OrderStatus
		want   bool
	}{
		{"paid is paid", enums.OrderStatusPaid, true},
		{"pending is not paid", enums.OrderStatusPending, false},
		{"failed is not paid", enums.OrderStatusFailed, false},
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
	statuses := enums.AllOrderStatuses()
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
		event enums.WebhookEventType
		want  string
	}{
		{"subscription_created", enums.WebhookEventSubscriptionCreated, "subscription_created"},
		{"subscription_updated", enums.WebhookEventSubscriptionUpdated, "subscription_updated"},
		{"order_created", enums.WebhookEventOrderCreated, "order_created"},
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
		event enums.WebhookEventType
		want  bool
	}{
		{"subscription_created is valid", enums.WebhookEventSubscriptionCreated, true},
		{"subscription_updated is valid", enums.WebhookEventSubscriptionUpdated, true},
		{"subscription_cancelled is valid", enums.WebhookEventSubscriptionCancelled, true},
		{"subscription_resumed is valid", enums.WebhookEventSubscriptionResumed, true},
		{"subscription_expired is valid", enums.WebhookEventSubscriptionExpired, true},
		{"subscription_paused is valid", enums.WebhookEventSubscriptionPaused, true},
		{"subscription_unpaused is valid", enums.WebhookEventSubscriptionUnpaused, true},
		{"subscription_payment_success is valid", enums.WebhookEventSubscriptionPaymentSuccess, true},
		{"subscription_payment_failed is valid", enums.WebhookEventSubscriptionPaymentFailed, true},
		{"subscription_payment_recovered is valid", enums.WebhookEventSubscriptionPaymentRecovered, true},
		{"order_created is valid", enums.WebhookEventOrderCreated, true},
		{"order_refunded is valid", enums.WebhookEventOrderRefunded, true},
		{"invalid event", enums.WebhookEventType("invalid"), false},
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
		event enums.WebhookEventType
		want  bool
	}{
		{"subscription_created is subscription event", enums.WebhookEventSubscriptionCreated, true},
		{"subscription_updated is subscription event", enums.WebhookEventSubscriptionUpdated, true},
		{"subscription_cancelled is subscription event", enums.WebhookEventSubscriptionCancelled, true},
		{"subscription_payment_success is subscription event", enums.WebhookEventSubscriptionPaymentSuccess, true},
		{"order_created is not subscription event", enums.WebhookEventOrderCreated, false},
		{"order_refunded is not subscription event", enums.WebhookEventOrderRefunded, false},
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
		event enums.WebhookEventType
		want  bool
	}{
		{"order_created is order event", enums.WebhookEventOrderCreated, true},
		{"order_refunded is order event", enums.WebhookEventOrderRefunded, true},
		{"subscription_created is not order event", enums.WebhookEventSubscriptionCreated, false},
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
	events := enums.AllWebhookEventTypes()
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
		role enums.UserRole
		want string
	}{
		{"customer", enums.UserRoleCustomer, "customer"},
		{"manager", enums.UserRoleManager, "manager"},
		{"admin", enums.UserRoleAdmin, "admin"},
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
		role enums.UserRole
		want bool
	}{
		{"customer is valid", enums.UserRoleCustomer, true},
		{"manager is valid", enums.UserRoleManager, true},
		{"admin is valid", enums.UserRoleAdmin, true},
		{"invalid role", enums.UserRole("superuser"), false},
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
		role enums.UserRole
		want bool
	}{
		{"admin is admin", enums.UserRoleAdmin, true},
		{"manager is admin", enums.UserRoleManager, true},
		{"customer is not admin", enums.UserRoleCustomer, false},
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
	roles := enums.AllUserRoles()
	if len(roles) != 3 {
		t.Errorf("AllUserRoles() returned %d roles, want 3", len(roles))
	}

	for _, role := range roles {
		if !role.IsValid() {
			t.Errorf("AllUserRoles() returned invalid role: %v", role)
		}
	}
}
