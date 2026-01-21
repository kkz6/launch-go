package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// OrderRepository handles database operations for orders
type OrderRepository struct {
	repository.Base[models.Order]
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{
		Base: repository.NewBase[models.Order](db),
	}
}

// Create creates a new order
func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	return r.DB.WithContext(ctx).Create(order).Error
}

// FindByID finds an order by ID
func (r *OrderRepository) FindByID(ctx context.Context, id string) (*models.Order, error) {
	var order models.Order
	err := r.DB.WithContext(ctx).First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// FindByLemonSqueezyID finds an order by LemonSqueezy ID
func (r *OrderRepository) FindByLemonSqueezyID(ctx context.Context, lemonSqueezyID string) (*models.Order, error) {
	var order models.Order
	err := r.DB.WithContext(ctx).First(&order, "lemon_squeezy_id = ?", lemonSqueezyID).Error
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// FindByTeam finds all orders for a team
func (r *OrderRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Order, error) {
	var orders []models.Order
	err := r.DB.WithContext(ctx).
		Where("billable_type IN ? AND billable_id = ?", models.TeamBillableTypes(), teamID).
		Order("created_at DESC").
		Find(&orders).Error

	return orders, err
}

// FindBySubscription finds all orders for a subscription
func (r *OrderRepository) FindBySubscription(ctx context.Context, subscriptionID string) ([]models.Order, error) {
	var orders []models.Order
	err := r.DB.WithContext(ctx).
		Where("subscription_id = ?", subscriptionID).
		Order("created_at DESC").
		Find(&orders).Error

	return orders, err
}

// Update updates an order
func (r *OrderRepository) Update(ctx context.Context, order *models.Order) error {
	return r.DB.WithContext(ctx).Save(order).Error
}

// UpdateStatus updates only the status of an order
func (r *OrderRepository) UpdateStatus(ctx context.Context, id string, status enums.OrderStatus) error {
	return r.DB.WithContext(ctx).
		Model(&models.Order{}).
		Where("id = ?", id).
		Update("status", status).Error
}
