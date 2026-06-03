package dto

import "time"

// RecentPayment is a single recent paid order surfaced on the overview
// dashboard. Currency is the order's own currency; Total is in cents.
type RecentPayment struct {
	TeamID    string     `json:"team_id"`
	TeamName  string     `json:"team_name"`
	Total     int64      `json:"total"`
	Currency  string     `json:"currency"`
	OrderedAt *time.Time `json:"ordered_at,omitempty"`
}

// RevenueMonth is one bucket of the revenue trend: the paid-order total for a
// calendar month, labelled "YYYY-MM".
type RevenueMonth struct {
	Month string `json:"month"`
	Total int64  `json:"total"`
}

// AdminOverview is the Stripe-like revenue/MRR dashboard payload computed from
// existing billing data. All money fields are in cents.
type AdminOverview struct {
	Currency              string          `json:"currency"`
	MRRCents              int64           `json:"mrr_cents"`
	TotalRevenueCents     int64           `json:"total_revenue_cents"`
	ActiveSubscriptions   int64           `json:"active_subscriptions"`
	TrialSubscriptions    int64           `json:"trial_subscriptions"`
	NewSubscriptionsMTD   int64           `json:"new_subscriptions_mtd"`
	CancelledMTD          int64           `json:"cancelled_mtd"`
	RevenueThisMonthCents int64           `json:"revenue_this_month_cents"`
	RevenueLastMonthCents int64           `json:"revenue_last_month_cents"`
	RecentPayments        []RecentPayment `json:"recent_payments"`
	RevenueTrend          []RevenueMonth  `json:"revenue_trend"`
}
