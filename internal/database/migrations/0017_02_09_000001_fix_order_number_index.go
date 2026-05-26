package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0017_02_09_000001_fix_order_number_index",
		Name:      "Drop unique constraint on orders.order_number",
		Timestamp: time.Date(2017, 2, 9, 0, 0, 1, 0, time.UTC),
		Up:        fixOrderNumberIndexUp,
		Down:      fixOrderNumberIndexDown,
	})
}

// Historically this migration:
//   - dropped a UNIQUE index on orders.order_number
//   - changed the column to nullable
//   - backfilled rows where order_number was 0
//
// 0012 now creates `orders` with `order_number INTEGER NULL UNIQUE`,
// so the nullability is already correct. The intent here is to drop
// the UNIQUE constraint so duplicate or NULL order_numbers are
// permitted. On a fresh install there are no rows to backfill.
func fixOrderNumberIndexUp(db *gorm.DB) error {
	// Drop whichever unique constraint name happens to be in place.
	// Postgres auto-generates names like `orders_order_number_key` for
	// inline UNIQUE; legacy MySQL dumps may have other names.
	candidates := []string{
		"orders_order_number_key",
		"uni_orders_order_number",
		"idx_order_number",
		"lemon_squeezy_orders_order_number_unique",
	}
	for _, name := range candidates {
		db.Exec("ALTER TABLE orders DROP CONSTRAINT IF EXISTS " + name)
		db.Exec("DROP INDEX IF EXISTS " + name)
	}

	// Backfill any 0 / NULL order_numbers to the row's id so future
	// non-null inserts don't clash. Safe to run repeatedly.
	return db.Exec("UPDATE orders SET order_number = id WHERE order_number = 0 OR order_number IS NULL").Error
}

func fixOrderNumberIndexDown(db *gorm.DB) error {
	// Restoring the unique constraint would fail if duplicates exist.
	// This is a deliberate one-way migration.
	return nil
}
