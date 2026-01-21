// Package constants provides centralized constant definitions
// to eliminate magic strings and numbers throughout the codebase.
package constants

// Cron expressions for common scheduling patterns.
// Format: minute hour day-of-month month day-of-week
const (
	// CronEveryMinute runs every minute.
	CronEveryMinute = "* * * * *"

	// CronEvery5Minutes runs every 5 minutes.
	CronEvery5Minutes = "*/5 * * * *"

	// CronEvery10Minutes runs every 10 minutes.
	CronEvery10Minutes = "*/10 * * * *"

	// CronEvery15Minutes runs every 15 minutes.
	CronEvery15Minutes = "*/15 * * * *"

	// CronEvery30Minutes runs every 30 minutes.
	CronEvery30Minutes = "*/30 * * * *"

	// CronEveryHour runs at the start of every hour.
	CronEveryHour = "0 * * * *"

	// CronEvery2Hours runs every 2 hours.
	CronEvery2Hours = "0 */2 * * *"

	// CronEvery6Hours runs every 6 hours.
	CronEvery6Hours = "0 */6 * * *"

	// CronEvery12Hours runs every 12 hours.
	CronEvery12Hours = "0 */12 * * *"

	// CronDaily runs once daily at midnight.
	CronDaily = "0 0 * * *"

	// CronDailyAt3AM runs once daily at 3 AM.
	CronDailyAt3AM = "0 3 * * *"

	// CronWeekly runs once weekly on Sunday at midnight.
	CronWeekly = "0 0 * * 0"

	// CronMonthly runs once monthly on the 1st at midnight.
	CronMonthly = "0 0 1 * *"

	// CronYearly runs once yearly on January 1st at midnight.
	CronYearly = "0 0 1 1 *"
)

// CronAt returns a cron expression for running at a specific hour (0-23).
func CronAt(hour int) string {
	if hour < 0 {
		hour = 0
	}
	if hour > 23 {
		hour = 23
	}
	return "0 " + itoa(hour) + " * * *"
}

// CronAtMinute returns a cron expression for running at a specific minute (0-59).
func CronAtMinute(minute int) string {
	if minute < 0 {
		minute = 0
	}
	if minute > 59 {
		minute = 59
	}
	return itoa(minute) + " * * * *"
}

// CronEveryNMinutes returns a cron expression for running every N minutes.
func CronEveryNMinutes(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 59 {
		n = 59
	}
	return "*/" + itoa(n) + " * * * *"
}

// CronEveryNHours returns a cron expression for running every N hours.
func CronEveryNHours(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 23 {
		n = 23
	}
	return "0 */" + itoa(n) + " * * *"
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + itoa(-i)
	}
	var result []byte
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}
	return string(result)
}
