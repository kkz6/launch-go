package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "negative bytes",
			bytes:    -100,
			expected: "0 B",
		},
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "one KB",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "1.5 KB",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "one MB",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "2.3 MB",
			bytes:    2411724,
			expected: "2.3 MB",
		},
		{
			name:     "one GB",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "1.5 GB",
			bytes:    1610612736,
			expected: "1.5 GB",
		},
		{
			name:     "one TB",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TB",
		},
		{
			name:     "2.5 TB",
			bytes:    2748779069440,
			expected: "2.5 TB",
		},
		{
			name:     "just under KB",
			bytes:    1023,
			expected: "1023 B",
		},
		{
			name:     "just under MB",
			bytes:    1024*1024 - 1,
			expected: "1024.0 KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int64
		expected string
	}{
		{
			name:     "zero seconds",
			seconds:  0,
			expected: "0s",
		},
		{
			name:     "negative seconds",
			seconds:  -100,
			expected: "0s",
		},
		{
			name:     "45 seconds",
			seconds:  45,
			expected: "45s",
		},
		{
			name:     "30 minutes",
			seconds:  30 * 60,
			expected: "30m",
		},
		{
			name:     "1 hour",
			seconds:  60 * 60,
			expected: "1h 0m",
		},
		{
			name:     "12 hours 30 minutes",
			seconds:  12*60*60 + 30*60,
			expected: "12h 30m",
		},
		{
			name:     "1 day",
			seconds:  24 * 60 * 60,
			expected: "1d 0h 0m",
		},
		{
			name:     "5 days 12 hours 30 minutes",
			seconds:  5*24*60*60 + 12*60*60 + 30*60,
			expected: "5d 12h 30m",
		},
		{
			name:     "3 hours 45 minutes",
			seconds:  3*60*60 + 45*60,
			expected: "3h 45m",
		},
		{
			name:     "10 days 0 hours 0 minutes",
			seconds:  10 * 24 * 60 * 60,
			expected: "10d 0h 0m",
		},
		{
			name:     "2 days 5 hours 0 minutes",
			seconds:  2*24*60*60 + 5*60*60,
			expected: "2d 5h 0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUptime(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPercentage(t *testing.T) {
	tests := []struct {
		name     string
		used     float64
		total    float64
		expected string
	}{
		{
			name:     "zero total",
			used:     50,
			total:    0,
			expected: "0.0%",
		},
		{
			name:     "negative total",
			used:     50,
			total:    -100,
			expected: "0.0%",
		},
		{
			name:     "75.5 percent",
			used:     755,
			total:    1000,
			expected: "75.5%",
		},
		{
			name:     "100 percent",
			used:     1000,
			total:    1000,
			expected: "100.0%",
		},
		{
			name:     "0 percent",
			used:     0,
			total:    1000,
			expected: "0.0%",
		},
		{
			name:     "50 percent",
			used:     500,
			total:    1000,
			expected: "50.0%",
		},
		{
			name:     "small percentage",
			used:     1,
			total:    1000,
			expected: "0.1%",
		},
		{
			name:     "over 100 percent capped",
			used:     1500,
			total:    1000,
			expected: "100.0%",
		},
		{
			name:     "negative used capped at 0",
			used:     -100,
			total:    1000,
			expected: "0.0%",
		},
		{
			name:     "decimal precision",
			used:     333,
			total:    1000,
			expected: "33.3%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPercentage(tt.used, tt.total)
			assert.Equal(t, tt.expected, result)
		})
	}
}
