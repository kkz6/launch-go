package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCronConstants(t *testing.T) {
	t.Run("every minute", func(t *testing.T) {
		assert.Equal(t, "* * * * *", CronEveryMinute)
	})

	t.Run("every 5 minutes", func(t *testing.T) {
		assert.Equal(t, "*/5 * * * *", CronEvery5Minutes)
	})

	t.Run("every hour", func(t *testing.T) {
		assert.Equal(t, "0 * * * *", CronEveryHour)
	})

	t.Run("daily", func(t *testing.T) {
		assert.Equal(t, "0 0 * * *", CronDaily)
	})

	t.Run("weekly", func(t *testing.T) {
		assert.Equal(t, "0 0 * * 0", CronWeekly)
	})

	t.Run("monthly", func(t *testing.T) {
		assert.Equal(t, "0 0 1 * *", CronMonthly)
	})
}

func TestCronAt(t *testing.T) {
	t.Run("valid hour", func(t *testing.T) {
		assert.Equal(t, "0 9 * * *", CronAt(9))
		assert.Equal(t, "0 0 * * *", CronAt(0))
		assert.Equal(t, "0 23 * * *", CronAt(23))
	})

	t.Run("clamps negative hour", func(t *testing.T) {
		assert.Equal(t, "0 0 * * *", CronAt(-1))
	})

	t.Run("clamps hour over 23", func(t *testing.T) {
		assert.Equal(t, "0 23 * * *", CronAt(24))
		assert.Equal(t, "0 23 * * *", CronAt(100))
	})
}

func TestCronAtMinute(t *testing.T) {
	t.Run("valid minute", func(t *testing.T) {
		assert.Equal(t, "30 * * * *", CronAtMinute(30))
		assert.Equal(t, "0 * * * *", CronAtMinute(0))
		assert.Equal(t, "59 * * * *", CronAtMinute(59))
	})

	t.Run("clamps negative minute", func(t *testing.T) {
		assert.Equal(t, "0 * * * *", CronAtMinute(-1))
	})

	t.Run("clamps minute over 59", func(t *testing.T) {
		assert.Equal(t, "59 * * * *", CronAtMinute(60))
	})
}

func TestCronEveryNMinutes(t *testing.T) {
	t.Run("valid intervals", func(t *testing.T) {
		assert.Equal(t, "*/5 * * * *", CronEveryNMinutes(5))
		assert.Equal(t, "*/10 * * * *", CronEveryNMinutes(10))
		assert.Equal(t, "*/1 * * * *", CronEveryNMinutes(1))
	})

	t.Run("clamps to minimum 1", func(t *testing.T) {
		assert.Equal(t, "*/1 * * * *", CronEveryNMinutes(0))
		assert.Equal(t, "*/1 * * * *", CronEveryNMinutes(-5))
	})

	t.Run("clamps to maximum 59", func(t *testing.T) {
		assert.Equal(t, "*/59 * * * *", CronEveryNMinutes(60))
		assert.Equal(t, "*/59 * * * *", CronEveryNMinutes(100))
	})
}

func TestCronEveryNHours(t *testing.T) {
	t.Run("valid intervals", func(t *testing.T) {
		assert.Equal(t, "0 */2 * * *", CronEveryNHours(2))
		assert.Equal(t, "0 */6 * * *", CronEveryNHours(6))
		assert.Equal(t, "0 */1 * * *", CronEveryNHours(1))
	})

	t.Run("clamps to minimum 1", func(t *testing.T) {
		assert.Equal(t, "0 */1 * * *", CronEveryNHours(0))
		assert.Equal(t, "0 */1 * * *", CronEveryNHours(-2))
	})

	t.Run("clamps to maximum 23", func(t *testing.T) {
		assert.Equal(t, "0 */23 * * *", CronEveryNHours(24))
		assert.Equal(t, "0 */23 * * *", CronEveryNHours(48))
	})
}

func TestItoa(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		assert.Equal(t, "0", itoa(0))
	})

	t.Run("positive numbers", func(t *testing.T) {
		assert.Equal(t, "1", itoa(1))
		assert.Equal(t, "10", itoa(10))
		assert.Equal(t, "123", itoa(123))
	})

	t.Run("negative numbers", func(t *testing.T) {
		assert.Equal(t, "-1", itoa(-1))
		assert.Equal(t, "-42", itoa(-42))
	})
}
