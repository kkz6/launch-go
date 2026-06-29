package migrations

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0012_02_06_000000_rename_billing_tables",
		Name:      "Create billing tables (subscriptions, orders, webhook events)",
		Timestamp: time.Date(2012, 2, 6, 0, 0, 0, 0, time.UTC),
		Up:        renameBillingTablesUp,
	})
}

func tableExists(db *gorm.DB, name string) bool {
	return db.Migrator().HasTable(name)
}

func columnExists(db *gorm.DB, table, column string) bool {
	var count int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?", table, column).Scan(&count)
	return count > 0
}

// renameBillingTablesUp historically renamed lemon_squeezy_* tables to
// their generic provider-agnostic names. The Laravel ancestry that
// owned those tables is long gone; this migration now just creates the
// final shape directly. Production data lands via pgloader, not via
// re-running these migrations, so the rename path is dead weight.
func renameBillingTablesUp(db *gorm.DB) error {
	if !tableExists(db, "subscriptions") {
		if err := db.Exec(`CREATE TABLE subscriptions (
			id BIGSERIAL PRIMARY KEY,
			billable_type VARCHAR(255) NOT NULL,
			billable_id CHAR(26) NOT NULL,
			type VARCHAR(255) NOT NULL,
			provider VARCHAR(50) NOT NULL DEFAULT 'dodo_payments',
			provider_subscription_id VARCHAR(255) NOT NULL UNIQUE,
			customer_id VARCHAR(255) NULL,
			status VARCHAR(255) NOT NULL,
			product_id VARCHAR(255) NOT NULL,
			variant_id VARCHAR(255) NOT NULL,
			card_brand VARCHAR(255) NULL,
			card_last_four VARCHAR(255) NULL,
			pause_mode VARCHAR(255) NULL,
			pause_resumes_at TIMESTAMP NULL,
			trial_ends_at TIMESTAMP NULL,
			renews_at TIMESTAMP NULL,
			ends_at TIMESTAMP NULL,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL
		)`).Error; err != nil {
			return fmt.Errorf("failed to create subscriptions table: %w", err)
		}
		if err := db.Exec(`CREATE INDEX subscriptions_billable_idx ON subscriptions(billable_type, billable_id)`).Error; err != nil {
			return fmt.Errorf("failed to create subscriptions billable index: %w", err)
		}
	}

	if !tableExists(db, "orders") {
		if err := db.Exec(`CREATE TABLE orders (
			id BIGSERIAL PRIMARY KEY,
			billable_type VARCHAR(255) NOT NULL,
			billable_id CHAR(26) NOT NULL,
			provider VARCHAR(50) NOT NULL DEFAULT 'dodo_payments',
			provider_order_id VARCHAR(255) NOT NULL UNIQUE,
			customer_id VARCHAR(255) NOT NULL,
			identifier CHAR(36) NOT NULL UNIQUE,
			product_id VARCHAR(255) NOT NULL,
			variant_id VARCHAR(255) NOT NULL,
			order_number INTEGER NULL UNIQUE,
			currency VARCHAR(255) NOT NULL,
			subtotal INTEGER NOT NULL,
			discount_total INTEGER NOT NULL,
			tax INTEGER NOT NULL,
			total INTEGER NOT NULL,
			tax_name VARCHAR(255) NULL,
			status VARCHAR(255) NOT NULL,
			receipt_url VARCHAR(255) NULL,
			refunded BOOLEAN NOT NULL DEFAULT FALSE,
			refunded_at TIMESTAMP NULL,
			ordered_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL
		)`).Error; err != nil {
			return fmt.Errorf("failed to create orders table: %w", err)
		}
		if err := db.Exec(`CREATE INDEX orders_billable_idx ON orders(billable_type, billable_id)`).Error; err != nil {
			return fmt.Errorf("failed to create orders billable index: %w", err)
		}
		if err := db.Exec(`CREATE INDEX orders_product_id_index ON orders(product_id)`).Error; err != nil {
			return fmt.Errorf("failed to create orders product_id index: %w", err)
		}
		if err := db.Exec(`CREATE INDEX orders_variant_id_index ON orders(variant_id)`).Error; err != nil {
			return fmt.Errorf("failed to create orders variant_id index: %w", err)
		}
	}

	if !tableExists(db, "billing_webhook_events") {
		if err := db.Exec(`CREATE TABLE billing_webhook_events (
			id CHAR(26) PRIMARY KEY,
			event_name VARCHAR(100) NOT NULL,
			payload TEXT NOT NULL,
			signature VARCHAR(255) NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT FALSE,
			processed_at TIMESTAMP NULL,
			error TEXT NULL,
			retry_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL
		)`).Error; err != nil {
			return fmt.Errorf("failed to create billing_webhook_events table: %w", err)
		}
		if err := db.Exec(`CREATE INDEX idx_event_name ON billing_webhook_events(event_name)`).Error; err != nil {
			return fmt.Errorf("failed to create idx_event_name index: %w", err)
		}
	}

	return nil
}
