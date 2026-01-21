package enums

import (
	"testing"
)

func TestCronSchedule_Expression(t *testing.T) {
	tests := []struct {
		schedule CronSchedule
		expected string
	}{
		{CronEveryMinute, "* * * * *"},
		{CronEvery5Minutes, "*/5 * * * *"},
		{CronEvery15Minutes, "*/15 * * * *"},
		{CronEvery30Minutes, "*/30 * * * *"},
		{CronHourly, "0 * * * *"},
		{CronDaily, "0 0 * * *"},
		{CronDaily2AM, "0 2 * * *"},
		{CronDaily3AM, "0 3 * * *"},
		{CronWeekly, "0 0 * * 0"},
		{CronMonthly, "0 0 1 * *"},
	}

	for _, tt := range tests {
		result := tt.schedule.Expression()
		if result != tt.expected {
			t.Errorf("CronSchedule(%q).Expression() = %q, want %q", tt.schedule, result, tt.expected)
		}
	}
}

func TestCronSchedule_Description(t *testing.T) {
	tests := []struct {
		schedule    CronSchedule
		expected    string
		description string
	}{
		{CronEveryMinute, "Every minute", "every minute schedule"},
		{CronEvery5Minutes, "Every 5 minutes", "every 5 minutes schedule"},
		{CronEvery15Minutes, "Every 15 minutes", "every 15 minutes schedule"},
		{CronEvery30Minutes, "Every 30 minutes", "every 30 minutes schedule"},
		{CronHourly, "Every hour", "hourly schedule"},
		{CronDaily, "Every day at midnight", "daily schedule"},
		{CronDaily2AM, "Every day at 2:00 AM", "daily 2am schedule"},
		{CronDaily3AM, "Every day at 3:00 AM", "daily 3am schedule"},
		{CronWeekly, "Every week on Sunday at midnight", "weekly schedule"},
		{CronMonthly, "Every month on the 1st at midnight", "monthly schedule"},
	}

	for _, tt := range tests {
		result := tt.schedule.Description()
		if result != tt.expected {
			t.Errorf("CronSchedule(%q).Description() = %q, want %q (%s)", tt.schedule, result, tt.expected, tt.description)
		}
	}
}

func TestCronSchedule_Description_UnknownSchedule(t *testing.T) {
	unknownSchedule := CronSchedule("0 5 * * *")
	expected := "Custom schedule"
	result := unknownSchedule.Description()

	if result != expected {
		t.Errorf("Unknown schedule description = %q, want %q", result, expected)
	}
}

func TestCronSchedule_FrequencyName(t *testing.T) {
	tests := []struct {
		schedule CronSchedule
		expected string
	}{
		{CronEveryMinute, "every_minute"},
		{CronEvery5Minutes, "every_5_minutes"},
		{CronEvery15Minutes, "every_15_minutes"},
		{CronEvery30Minutes, "every_30_minutes"},
		{CronHourly, "hourly"},
		{CronDaily, "daily"},
		{CronDaily2AM, "daily_2am"},
		{CronDaily3AM, "daily_3am"},
		{CronWeekly, "weekly"},
		{CronMonthly, "monthly"},
	}

	for _, tt := range tests {
		result := tt.schedule.FrequencyName()
		if result != tt.expected {
			t.Errorf("CronSchedule(%q).FrequencyName() = %q, want %q", tt.schedule, result, tt.expected)
		}
	}
}

func TestCronSchedule_FrequencyName_UnknownSchedule(t *testing.T) {
	unknownSchedule := CronSchedule("0 5 * * *")
	expected := "custom"
	result := unknownSchedule.FrequencyName()

	if result != expected {
		t.Errorf("Unknown schedule frequency name = %q, want %q", result, expected)
	}
}

func TestFromExpression(t *testing.T) {
	tests := []struct {
		expression string
		expected   CronSchedule
		shouldFind bool
	}{
		{"* * * * *", CronEveryMinute, true},
		{"*/5 * * * *", CronEvery5Minutes, true},
		{"*/15 * * * *", CronEvery15Minutes, true},
		{"*/30 * * * *", CronEvery30Minutes, true},
		{"0 * * * *", CronHourly, true},
		{"0 0 * * *", CronDaily, true},
		{"0 2 * * *", CronDaily2AM, true},
		{"0 3 * * *", CronDaily3AM, true},
		{"0 0 * * 0", CronWeekly, true},
		{"0 0 1 * *", CronMonthly, true},
		{"0 5 * * *", "", false},
		{"invalid", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		result, found := FromExpression(tt.expression)
		if found != tt.shouldFind {
			t.Errorf("FromExpression(%q) found = %v, want %v", tt.expression, found, tt.shouldFind)
		}

		if found && result != tt.expected {
			t.Errorf("FromExpression(%q) = %q, want %q", tt.expression, result, tt.expected)
		}
	}
}

func TestCronSchedule_IsValid(t *testing.T) {
	tests := []struct {
		schedule CronSchedule
		expected bool
	}{
		{CronEveryMinute, true},
		{CronEvery5Minutes, true},
		{CronEvery15Minutes, true},
		{CronEvery30Minutes, true},
		{CronHourly, true},
		{CronDaily, true},
		{CronDaily2AM, true},
		{CronDaily3AM, true},
		{CronWeekly, true},
		{CronMonthly, true},
		{CronSchedule("0 5 * * *"), false},
		{CronSchedule("invalid"), false},
		{CronSchedule(""), false},
	}

	for _, tt := range tests {
		result := tt.schedule.IsValid()
		if result != tt.expected {
			t.Errorf("CronSchedule(%q).IsValid() = %v, want %v", tt.schedule, result, tt.expected)
		}
	}
}

func TestCronSchedule_String(t *testing.T) {
	tests := []struct {
		schedule CronSchedule
		expected string
	}{
		{CronEveryMinute, "* * * * *"},
		{CronHourly, "0 * * * *"},
		{CronDaily, "0 0 * * *"},
	}

	for _, tt := range tests {
		result := tt.schedule.String()
		if result != tt.expected {
			t.Errorf("CronSchedule(%q).String() = %q, want %q", tt.schedule, result, tt.expected)
		}
	}
}

func TestAllCronSchedules(t *testing.T) {
	schedules := AllCronSchedules()

	expectedCount := 10
	if len(schedules) != expectedCount {
		t.Errorf("AllCronSchedules() returned %d schedules, want %d", len(schedules), expectedCount)
	}

	for _, schedule := range schedules {
		if !schedule.IsValid() {
			t.Errorf("AllCronSchedules() returned invalid schedule: %q", schedule)
		}
	}
}
