package services

import (
	"context"
	"sort"
	"time"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	staffdto "github.com/kkz6/launch-go/internal/modules/staff/dto"
)

const (
	overviewRecentPaymentsLimit = 10
	overviewTrendMonths         = 6
	defaultOverviewCurrency     = "USD"
	monthsPerYear               = 12
)

// SetMonthlyEquivByProduct injects the product -> monthly-equivalent-cents map
// used to compute MRR. Tests use this to control pricing without depending on
// plan config; in production the map is built lazily from the default plans.
func (s *Service) SetMonthlyEquivByProduct(m map[string]int64) {
	s.monthlyEquivByProduct = m
}

// monthlyEquivMap returns the product -> monthly-equivalent-cents map, building
// it from the default plan config on first use. A monthly product maps to its
// monthly price; a yearly product maps to its yearly price divided by 12.
func (s *Service) monthlyEquivMap() map[string]int64 {
	if s.monthlyEquivByProduct != nil {
		return s.monthlyEquivByProduct
	}

	built := make(map[string]int64)
	for _, plan := range billingmodels.DefaultPlans() {
		if plan.MonthlyID != "" {
			built[plan.MonthlyID] = plan.MonthlyPricing
		}

		if plan.YearlyID != "" {
			built[plan.YearlyID] = plan.YearlyPricing / monthsPerYear
		}
	}

	s.monthlyEquivByProduct = built
	return built
}

// Overview computes the revenue/MRR dashboard from existing billing data.
// `now` is injected so month-boundary math is testable; the handler passes
// time.Now(). Month boundaries are derived from now's location: the first
// instant of the current month, the previous month, and the six-month trend
// window. The "this month" window is [startThisMonth, now]; "last month" is
// [startLastMonth, startThisMonth).
func (s *Service) Overview(ctx context.Context, now time.Time) (*staffdto.AdminOverview, error) {
	startThisMonth := startOfMonth(now)
	startLastMonth := startThisMonth.AddDate(0, -1, 0)

	overview := &staffdto.AdminOverview{
		Currency:       defaultOverviewCurrency,
		RecentPayments: []staffdto.RecentPayment{},
		RevenueTrend:   []staffdto.RevenueMonth{},
	}

	mrr, err := s.calculateMRR(ctx)
	if err != nil {
		return nil, err
	}
	overview.MRRCents = mrr

	if err := s.populateSubscriptionCounts(ctx, overview, now, startThisMonth); err != nil {
		return nil, err
	}

	if err := s.populateRevenue(ctx, overview, now, startThisMonth, startLastMonth); err != nil {
		return nil, err
	}

	if err := s.populateRecentPayments(ctx, overview); err != nil {
		return nil, err
	}

	if err := s.populateRevenueTrend(ctx, overview, startThisMonth); err != nil {
		return nil, err
	}

	if err := s.populateCurrency(ctx, overview); err != nil {
		return nil, err
	}

	return overview, nil
}

// calculateMRR sums the monthly-equivalent price of every active/on_trial team
// subscription. Unknown products contribute 0.
func (s *Service) calculateMRR(ctx context.Context) (int64, error) {
	priceByProduct := s.monthlyEquivMap()

	var products []string
	err := s.repos.DB().WithContext(ctx).
		Model(&billingmodels.Subscription{}).
		Where("status IN ? AND billable_type IN ?",
			[]billingtypes.SubscriptionStatus{
				billingtypes.SubscriptionStatusActive,
				billingtypes.SubscriptionStatusOnTrial,
			},
			billingmodels.TeamBillableTypes()).
		Pluck("product_id", &products).Error
	if err != nil {
		return 0, err
	}

	var mrr int64
	for _, productID := range products {
		mrr += priceByProduct[productID]
	}

	return mrr, nil
}

// populateSubscriptionCounts fills the active/trial/new-MTD/cancelled-MTD
// counts. Active = status active. Trial = status on_trial with a future
// trial_ends_at. New MTD = created this month. Cancelled MTD = status cancelled
// with updated_at this month.
func (s *Service) populateSubscriptionCounts(ctx context.Context, overview *staffdto.AdminOverview, now, startThisMonth time.Time) error {
	teamTypes := billingmodels.TeamBillableTypes()

	countSubs := func(query string, args ...any) (int64, error) {
		var count int64
		err := s.repos.DB().WithContext(ctx).
			Model(&billingmodels.Subscription{}).
			Where("billable_type IN ?", teamTypes).
			Where(query, args...).
			Count(&count).Error
		return count, err
	}

	active, err := countSubs("status = ?", billingtypes.SubscriptionStatusActive)
	if err != nil {
		return err
	}
	overview.ActiveSubscriptions = active

	trial, err := countSubs("status = ? AND trial_ends_at IS NOT NULL AND trial_ends_at > ?", billingtypes.SubscriptionStatusOnTrial, now)
	if err != nil {
		return err
	}
	overview.TrialSubscriptions = trial

	newMTD, err := countSubs("created_at >= ?", startThisMonth)
	if err != nil {
		return err
	}
	overview.NewSubscriptionsMTD = newMTD

	cancelledMTD, err := countSubs("status = ? AND updated_at >= ?", billingtypes.SubscriptionStatusCancelled, startThisMonth)
	if err != nil {
		return err
	}
	overview.CancelledMTD = cancelledMTD

	return nil
}

// populateRevenue fills total/this-month/last-month paid revenue.
func (s *Service) populateRevenue(ctx context.Context, overview *staffdto.AdminOverview, now, startThisMonth, startLastMonth time.Time) error {
	total, err := s.sumPaidOrders(ctx, nil, time.Time{}, time.Time{})
	if err != nil {
		return err
	}
	overview.TotalRevenueCents = total

	thisMonth, err := s.sumPaidOrders(ctx, &startThisMonth, startThisMonth, now)
	if err != nil {
		return err
	}
	overview.RevenueThisMonthCents = thisMonth

	lastMonth, err := s.sumPaidOrders(ctx, &startLastMonth, startLastMonth, startThisMonth)
	if err != nil {
		return err
	}
	overview.RevenueLastMonthCents = lastMonth

	return nil
}

// sumPaidOrders sums Total over paid orders. When `window` is nil it sums all
// paid orders; otherwise it sums orders with ordered_at in [from, to).
func (s *Service) sumPaidOrders(ctx context.Context, window *time.Time, from, to time.Time) (int64, error) {
	query := s.repos.DB().WithContext(ctx).
		Model(&billingmodels.Order{}).
		Where("status = ?", billingtypes.OrderStatusPaid)

	if window != nil {
		query = query.Where("ordered_at >= ? AND ordered_at < ?", from, to)
	}

	var total int64
	if err := query.Select("COALESCE(SUM(total), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

// populateRecentPayments loads the latest paid orders and resolves team names.
func (s *Service) populateRecentPayments(ctx context.Context, overview *staffdto.AdminOverview) error {
	var orders []billingmodels.Order
	err := s.repos.DB().WithContext(ctx).
		Model(&billingmodels.Order{}).
		Where("status = ?", billingtypes.OrderStatusPaid).
		Order("ordered_at DESC").
		Limit(overviewRecentPaymentsLimit).
		Find(&orders).Error
	if err != nil {
		return err
	}

	if len(orders) == 0 {
		return nil
	}

	teamIDs := make([]string, 0, len(orders))
	for i := range orders {
		teamIDs = append(teamIDs, orders[i].BillableID)
	}

	teamNames, err := s.teamNamesByID(ctx, teamIDs)
	if err != nil {
		return err
	}

	payments := make([]staffdto.RecentPayment, 0, len(orders))
	for i := range orders {
		order := orders[i]
		orderedAt := order.OrderedAt
		payments = append(payments, staffdto.RecentPayment{
			TeamID:    order.BillableID,
			TeamName:  teamNames[order.BillableID],
			Total:     order.Total,
			Currency:  order.Currency,
			OrderedAt: &orderedAt,
		})
	}

	overview.RecentPayments = payments
	return nil
}

// teamNamesByID resolves team display names for the given ids in one query.
func (s *Service) teamNamesByID(ctx context.Context, teamIDs []string) (map[string]string, error) {
	names := make(map[string]string, len(teamIDs))
	if len(teamIDs) == 0 {
		return names, nil
	}

	var teams []authmodels.Team
	err := s.repos.DB().WithContext(ctx).
		Model(&authmodels.Team{}).
		Where("id IN ?", teamIDs).
		Find(&teams).Error
	if err != nil {
		return nil, err
	}

	for i := range teams {
		names[teams[i].ID] = teams[i].Name
	}

	return names, nil
}

// populateRevenueTrend builds the last six months of paid revenue ascending,
// each bucket labelled "YYYY-MM".
func (s *Service) populateRevenueTrend(ctx context.Context, overview *staffdto.AdminOverview, startThisMonth time.Time) error {
	trend := make([]staffdto.RevenueMonth, 0, overviewTrendMonths)

	for i := overviewTrendMonths - 1; i >= 0; i-- {
		monthStart := startThisMonth.AddDate(0, -i, 0)
		monthEnd := monthStart.AddDate(0, 1, 0)

		total, err := s.sumPaidOrders(ctx, &monthStart, monthStart, monthEnd)
		if err != nil {
			return err
		}

		trend = append(trend, staffdto.RevenueMonth{
			Month: monthStart.Format("2006-01"),
			Total: total,
		})
	}

	overview.RevenueTrend = trend
	return nil
}

// populateCurrency sets the dominant currency among paid orders, defaulting to
// USD when there are no paid orders.
func (s *Service) populateCurrency(ctx context.Context, overview *staffdto.AdminOverview) error {
	type currencyCount struct {
		Currency string
		Count    int64
	}

	var rows []currencyCount
	err := s.repos.DB().WithContext(ctx).
		Model(&billingmodels.Order{}).
		Select("currency, COUNT(*) as count").
		Where("status = ?", billingtypes.OrderStatusPaid).
		Group("currency").
		Scan(&rows).Error
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	sort.Slice(rows, func(a, b int) bool {
		if rows[a].Count != rows[b].Count {
			return rows[a].Count > rows[b].Count
		}
		return rows[a].Currency < rows[b].Currency
	})

	if rows[0].Currency != "" {
		overview.Currency = rows[0].Currency
	}

	return nil
}

// startOfMonth returns the first instant of now's calendar month, preserving
// the location.
func startOfMonth(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}
