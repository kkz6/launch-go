package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0016_02_09_000000_add_customer_id_to_subscriptions",
		Name:      "Add customer_id column to subscriptions table",
		Timestamp: time.Date(2016, 2, 9, 0, 0, 0, 0, time.UTC),
		Up:        addCustomerIDToSubscriptionsUp,
	})
}

func addCustomerIDToSubscriptionsUp(db *gorm.DB) error {
	if !columnExists(db, "subscriptions", "customer_id") {
		return db.Exec("ALTER TABLE subscriptions ADD COLUMN customer_id VARCHAR(255) NULL").Error
	}
	return nil
}
