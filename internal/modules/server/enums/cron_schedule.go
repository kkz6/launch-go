package enums

import (
	"database/sql/driver"

	baseenums "github.com/kkz6/launch-go/internal/pkg/enums"
)

// CronSchedule represents predefined cron schedule expressions
type CronSchedule string

const (
	CronEveryMinute    CronSchedule = "* * * * *"
	CronEvery5Minutes  CronSchedule = "*/5 * * * *"
	CronEvery15Minutes CronSchedule = "*/15 * * * *"
	CronEvery30Minutes CronSchedule = "*/30 * * * *"
	CronHourly         CronSchedule = "0 * * * *"
	CronDaily          CronSchedule = "0 0 * * *"
	CronDaily2AM       CronSchedule = "0 2 * * *"
	CronDaily3AM       CronSchedule = "0 3 * * *"
	CronWeekly         CronSchedule = "0 0 * * 0"
	CronMonthly        CronSchedule = "0 0 1 * *"
)

var cronScheduleDescriptions = map[CronSchedule]string{
	CronEveryMinute:    "Every minute",
	CronEvery5Minutes:  "Every 5 minutes",
	CronEvery15Minutes: "Every 15 minutes",
	CronEvery30Minutes: "Every 30 minutes",
	CronHourly:         "Every hour",
	CronDaily:          "Every day at midnight",
	CronDaily2AM:       "Every day at 2:00 AM",
	CronDaily3AM:       "Every day at 3:00 AM",
	CronWeekly:         "Every week on Sunday at midnight",
	CronMonthly:        "Every month on the 1st at midnight",
}

var cronScheduleFrequencyNames = map[CronSchedule]string{
	CronEveryMinute:    "every_minute",
	CronEvery5Minutes:  "every_5_minutes",
	CronEvery15Minutes: "every_15_minutes",
	CronEvery30Minutes: "every_30_minutes",
	CronHourly:         "hourly",
	CronDaily:          "daily",
	CronDaily2AM:       "daily_2am",
	CronDaily3AM:       "daily_3am",
	CronWeekly:         "weekly",
	CronMonthly:        "monthly",
}

func (c CronSchedule) String() string {
	return string(c)
}

// Expression returns the cron expression string
func (c CronSchedule) Expression() string {
	return string(c)
}

// Description returns a human-readable description
func (c CronSchedule) Description() string {
	if desc, ok := cronScheduleDescriptions[c]; ok {
		return desc
	}

	return "Custom schedule"
}

// FrequencyName returns a simple frequency identifier like "every_minute", "hourly", "daily", "weekly"
func (c CronSchedule) FrequencyName() string {
	if name, ok := cronScheduleFrequencyNames[c]; ok {
		return name
	}

	return "custom"
}

// IsValid checks if the schedule is a known constant
func (c CronSchedule) IsValid() bool {
	switch c {
	case CronEveryMinute, CronEvery5Minutes, CronEvery15Minutes, CronEvery30Minutes,
		CronHourly, CronDaily, CronDaily2AM, CronDaily3AM, CronWeekly, CronMonthly:
		return true
	}

	return false
}

// FromExpression tries to match an expression to a known schedule
func FromExpression(expr string) (CronSchedule, bool) {
	schedule := CronSchedule(expr)
	if schedule.IsValid() {
		return schedule, true
	}

	return "", false
}

func (c *CronSchedule) Scan(value interface{}) error {
	return baseenums.ScanString(c, value)
}

func (c CronSchedule) Value() (driver.Value, error) {
	return baseenums.ValueString(c)
}

// AllCronSchedules returns all predefined cron schedules
func AllCronSchedules() []CronSchedule {
	return []CronSchedule{
		CronEveryMinute, CronEvery5Minutes, CronEvery15Minutes, CronEvery30Minutes,
		CronHourly, CronDaily, CronDaily2AM, CronDaily3AM, CronWeekly, CronMonthly,
	}
}
