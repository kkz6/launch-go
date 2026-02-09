package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0017_02_09_000001_fix_order_number_index",
		Name:      "Change order_number from unique index to regular index and allow null",
		Timestamp: time.Date(2017, 2, 9, 0, 0, 1, 0, time.UTC),
		Up:        fixOrderNumberIndexUp,
		Down:      fixOrderNumberIndexDown,
	})
}

func fixOrderNumberIndexUp(db *gorm.DB) error {
	// Drop the unique index on order_number
	if err := db.Exec("ALTER TABLE `orders` DROP INDEX `uni_orders_order_number`").Error; err != nil {
		// Try alternate index name
		db.Exec("ALTER TABLE `orders` DROP INDEX `idx_order_number`")
	}

	// Change column to allow null and remove not null constraint
	if err := db.Exec("ALTER TABLE `orders` MODIFY COLUMN `order_number` INT NULL DEFAULT NULL").Error; err != nil {
		return err
	}

	// Set existing 0 values to their ID
	return db.Exec("UPDATE `orders` SET `order_number` = `id` WHERE `order_number` = 0 OR `order_number` IS NULL").Error
}

func fixOrderNumberIndexDown(db *gorm.DB) error {
	return db.Exec("ALTER TABLE `orders` MODIFY COLUMN `order_number` INT NOT NULL DEFAULT 0").Error
}
