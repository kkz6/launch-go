package cronutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDueInWindow(t *testing.T) {
	base := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		expression string
		start      time.Time
		due        bool
	}{
		{name: "every minute", expression: "* * * * *", start: base, due: true},
		{name: "daily at midnight", expression: "0 0 * * *", start: base, due: true},
		{name: "daily outside midnight", expression: "0 0 * * *", start: base.Add(time.Minute), due: false},
		{name: "quarter hour boundary", expression: "*/15 * * * *", start: base.Add(15 * time.Minute), due: true},
		{name: "quarter hour outside boundary", expression: "*/15 * * * *", start: base.Add(16 * time.Minute), due: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			due, err := DueInWindow(tt.expression, tt.start, tt.start.Add(time.Minute))
			require.NoError(t, err)
			require.Equal(t, tt.due, due)
		})
	}
}

func TestDueInWindowRejectsInvalidExpression(t *testing.T) {
	start := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	_, err := DueInWindow("not a cron", start, start.Add(time.Minute))
	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	require.NoError(t, Validate("0 0 * * *"))
	require.Error(t, Validate("0 0 * *"))
}
