package enums

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending  OrderStatus = "pending"
	OrderStatusPaid     OrderStatus = "paid"
	OrderStatusFailed   OrderStatus = "failed"
	OrderStatusRefunded OrderStatus = "refunded"
	OrderStatusDisputed OrderStatus = "disputed"
)

// String returns the string representation of the order status
func (o OrderStatus) String() string {
	return string(o)
}

// IsValid checks if the order status is valid
func (o OrderStatus) IsValid() bool {
	switch o {
	case OrderStatusPending, OrderStatusPaid, OrderStatusFailed, OrderStatusRefunded, OrderStatusDisputed:
		return true
	default:
		return false
	}
}

// IsPaid checks if the order is paid
func (o OrderStatus) IsPaid() bool {
	return o == OrderStatusPaid
}

// AllOrderStatuses returns all valid order statuses
func AllOrderStatuses() []OrderStatus {
	return []OrderStatus{
		OrderStatusPending,
		OrderStatusPaid,
		OrderStatusFailed,
		OrderStatusRefunded,
		OrderStatusDisputed,
	}
}
