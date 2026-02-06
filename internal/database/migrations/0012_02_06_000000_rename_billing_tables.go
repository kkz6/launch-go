package migrations

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0012_02_06_000000_rename_billing_tables",
		Name:      "Rename billing tables from lemon_squeezy to generic names",
		Timestamp: time.Date(2012, 2, 6, 0, 0, 0, 0, time.UTC),
		Up:        renameBillingTablesUp,
		Down:      renameBillingTablesDown,
	})
}

func renameBillingTablesUp(db *gorm.DB) error {
	// Rename lemon_squeezy_subscriptions to subscriptions
	if err := db.Exec("RENAME TABLE `lemon_squeezy_subscriptions` TO `subscriptions`").Error; err != nil {
		return fmt.Errorf("failed to rename lemon_squeezy_subscriptions: %w", err)
	}

	// Rename lemon_squeezy_orders to orders
	if err := db.Exec("RENAME TABLE `lemon_squeezy_orders` TO `orders`").Error; err != nil {
		return fmt.Errorf("failed to rename lemon_squeezy_orders: %w", err)
	}

	// Rename lemon_squeezy_webhook_events to billing_webhook_events
	if err := db.Exec("RENAME TABLE `lemon_squeezy_webhook_events` TO `billing_webhook_events`").Error; err != nil {
		return fmt.Errorf("failed to rename lemon_squeezy_webhook_events: %w", err)
	}

	// Rename lemon_squeezy_id column to provider_subscription_id in subscriptions table
	if err := db.Exec("ALTER TABLE `subscriptions` CHANGE COLUMN `lemon_squeezy_id` `provider_subscription_id` VARCHAR(255) NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to rename lemon_squeezy_id column in subscriptions: %w", err)
	}

	// Rename lemon_squeezy_id column to provider_order_id in orders table
	if err := db.Exec("ALTER TABLE `orders` CHANGE COLUMN `lemon_squeezy_id` `provider_order_id` VARCHAR(255) NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to rename lemon_squeezy_id column in orders: %w", err)
	}

	// Add provider column to subscriptions table
	if err := db.Exec("ALTER TABLE `subscriptions` ADD COLUMN `provider` VARCHAR(50) NOT NULL DEFAULT 'dodo_payments' AFTER `type`").Error; err != nil {
		return fmt.Errorf("failed to add provider column to subscriptions: %w", err)
	}

	// Add provider column to orders table
	if err := db.Exec("ALTER TABLE `orders` ADD COLUMN `provider` VARCHAR(50) NOT NULL DEFAULT 'dodo_payments' AFTER `billable_id`").Error; err != nil {
		return fmt.Errorf("failed to add provider column to orders: %w", err)
	}

	return nil
}

func renameBillingTablesDown(db *gorm.DB) error {
	// Remove provider columns
	db.Exec("ALTER TABLE `subscriptions` DROP COLUMN `provider`")
	db.Exec("ALTER TABLE `orders` DROP COLUMN `provider`")

	// Rename columns back
	db.Exec("ALTER TABLE `subscriptions` CHANGE COLUMN `provider_subscription_id` `lemon_squeezy_id` VARCHAR(255) NOT NULL")
	db.Exec("ALTER TABLE `orders` CHANGE COLUMN `provider_order_id` `lemon_squeezy_id` VARCHAR(255) NOT NULL")

	// Rename tables back
	db.Exec("RENAME TABLE `subscriptions` TO `lemon_squeezy_subscriptions`")
	db.Exec("RENAME TABLE `orders` TO `lemon_squeezy_orders`")
	db.Exec("RENAME TABLE `billing_webhook_events` TO `lemon_squeezy_webhook_events`")

	return nil
}
