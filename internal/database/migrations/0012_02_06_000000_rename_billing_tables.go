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

func tableExists(db *gorm.DB, name string) bool {
	return db.Migrator().HasTable(name)
}

func columnExists(db *gorm.DB, table, column string) bool {
	var count int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&count)
	return count > 0
}

func renameBillingTablesUp(db *gorm.DB) error {
	// Rename lemon_squeezy_subscriptions to subscriptions (if old table still exists)
	if tableExists(db, "lemon_squeezy_subscriptions") {
		if err := db.Exec("RENAME TABLE `lemon_squeezy_subscriptions` TO `subscriptions`").Error; err != nil {
			return fmt.Errorf("failed to rename lemon_squeezy_subscriptions: %w", err)
		}
	}

	// Rename lemon_squeezy_orders to orders (if old table still exists)
	if tableExists(db, "lemon_squeezy_orders") {
		if err := db.Exec("RENAME TABLE `lemon_squeezy_orders` TO `orders`").Error; err != nil {
			return fmt.Errorf("failed to rename lemon_squeezy_orders: %w", err)
		}
	}

	// Rename lemon_squeezy_id column to provider_subscription_id in subscriptions table
	if tableExists(db, "subscriptions") && columnExists(db, "subscriptions", "lemon_squeezy_id") {
		if err := db.Exec("ALTER TABLE `subscriptions` CHANGE COLUMN `lemon_squeezy_id` `provider_subscription_id` VARCHAR(255) NOT NULL").Error; err != nil {
			return fmt.Errorf("failed to rename lemon_squeezy_id column in subscriptions: %w", err)
		}
	}

	// Rename lemon_squeezy_id column to provider_order_id in orders table
	if tableExists(db, "orders") && columnExists(db, "orders", "lemon_squeezy_id") {
		if err := db.Exec("ALTER TABLE `orders` CHANGE COLUMN `lemon_squeezy_id` `provider_order_id` VARCHAR(255) NOT NULL").Error; err != nil {
			return fmt.Errorf("failed to rename lemon_squeezy_id column in orders: %w", err)
		}
	}

	// Add provider column to subscriptions table
	if tableExists(db, "subscriptions") && !columnExists(db, "subscriptions", "provider") {
		if err := db.Exec("ALTER TABLE `subscriptions` ADD COLUMN `provider` VARCHAR(50) NOT NULL DEFAULT 'dodo_payments' AFTER `type`").Error; err != nil {
			return fmt.Errorf("failed to add provider column to subscriptions: %w", err)
		}
	}

	// Add provider column to orders table
	if tableExists(db, "orders") && !columnExists(db, "orders", "provider") {
		if err := db.Exec("ALTER TABLE `orders` ADD COLUMN `provider` VARCHAR(50) NOT NULL DEFAULT 'dodo_payments' AFTER `billable_id`").Error; err != nil {
			return fmt.Errorf("failed to add provider column to orders: %w", err)
		}
	}

	// Create billing_webhook_events table (was never created as lemon_squeezy_webhook_events)
	if !tableExists(db, "billing_webhook_events") {
		if err := db.Exec(`CREATE TABLE billing_webhook_events (
			id CHAR(26) PRIMARY KEY,
			event_name VARCHAR(100) NOT NULL,
			payload TEXT NOT NULL,
			signature VARCHAR(255) NOT NULL,
			processed TINYINT(1) NOT NULL DEFAULT 0,
			processed_at TIMESTAMP NULL,
			error TEXT NULL,
			retry_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL,
			INDEX idx_event_name (event_name)
		)`).Error; err != nil {
			return fmt.Errorf("failed to create billing_webhook_events table: %w", err)
		}
	}

	return nil
}

func renameBillingTablesDown(db *gorm.DB) error {
	db.Exec("DROP TABLE IF EXISTS `billing_webhook_events`")

	db.Exec("ALTER TABLE `subscriptions` DROP COLUMN `provider`")
	db.Exec("ALTER TABLE `orders` DROP COLUMN `provider`")

	db.Exec("ALTER TABLE `subscriptions` CHANGE COLUMN `provider_subscription_id` `lemon_squeezy_id` VARCHAR(255) NOT NULL")
	db.Exec("ALTER TABLE `orders` CHANGE COLUMN `provider_order_id` `lemon_squeezy_id` VARCHAR(255) NOT NULL")

	db.Exec("RENAME TABLE `subscriptions` TO `lemon_squeezy_subscriptions`")
	db.Exec("RENAME TABLE `orders` TO `lemon_squeezy_orders`")

	return nil
}
