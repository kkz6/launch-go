package cronutil

import (
	"time"

	"github.com/robfig/cron/v3"
)

var parser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// Validate checks a standard five-field cron expression.
func Validate(expression string) error {
	_, err := parser.Parse(expression)
	return err
}

// DueInWindow reports whether a standard five-field cron expression fires in
// the half-open interval [windowStart, windowEnd).
func DueInWindow(expression string, windowStart, windowEnd time.Time) (bool, error) {
	schedule, err := parser.Parse(expression)
	if err != nil {
		return false, err
	}
	next := schedule.Next(windowStart.Add(-time.Nanosecond))
	return !next.Before(windowStart) && next.Before(windowEnd), nil
}
