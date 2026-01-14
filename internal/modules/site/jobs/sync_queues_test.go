package jobs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncQueuesJob_Type(t *testing.T) {
	job := &SyncQueuesJob{}
	assert.Equal(t, TypeSyncQueues, job.Type())
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected string
	}{
		{
			name:     "zero seconds",
			seconds:  0,
			expected: "",
		},
		{
			name:     "30 seconds",
			seconds:  30,
			expected: "30 seconds",
		},
		{
			name:     "1 second",
			seconds:  1,
			expected: "1 second",
		},
		{
			name:     "5 minutes",
			seconds:  300,
			expected: "5 minutes",
		},
		{
			name:     "1 minute",
			seconds:  60,
			expected: "1 minute",
		},
		{
			name:     "1 hour",
			seconds:  3600,
			expected: "1 hour",
		},
		{
			name:     "2 hours",
			seconds:  7200,
			expected: "2 hours",
		},
		{
			name:     "1 hour 30 minutes",
			seconds:  5400,
			expected: "1 hour, 30 minutes",
		},
		{
			name:     "1 day",
			seconds:  86400,
			expected: "1 day",
		},
		{
			name:     "2 days 3 hours",
			seconds:  183600, // 2 * 86400 + 3 * 3600
			expected: "2 days, 3 hours",
		},
		{
			name:     "1 day 1 hour 1 minute",
			seconds:  90060, // 86400 + 3600 + 60
			expected: "1 day, 1 hour, 1 minute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUptime(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncQueuesJob_ParseDaemonStatus(t *testing.T) {
	job := &SyncQueuesJob{}

	t.Run("parse valid JSON lines", func(t *testing.T) {
		output := `{"daemon_id":"01abc123","status":"RUNNING","pid":"12345","uptime_seconds":3600,"description":"pid 12345, uptime 1:00:00","error":""}
{"daemon_id":"01def456","status":"STOPPED","pid":"","uptime_seconds":0,"description":"Not started","error":"Not started"}
===STATUS_CHECK_COMPLETE===`

		results := job.parseDaemonStatus(output)

		require.Len(t, results, 2)

		assert.Equal(t, "01abc123", results[0].DaemonID)
		assert.Equal(t, "RUNNING", results[0].Status)
		assert.Equal(t, "12345", results[0].PID)
		assert.Equal(t, 3600, results[0].UptimeSeconds)

		assert.Equal(t, "01def456", results[1].DaemonID)
		assert.Equal(t, "STOPPED", results[1].Status)
		assert.Equal(t, "", results[1].PID)
		assert.Equal(t, 0, results[1].UptimeSeconds)
	})

	t.Run("skip empty lines", func(t *testing.T) {
		output := `
{"daemon_id":"01abc123","status":"RUNNING","pid":"12345","uptime_seconds":3600,"description":"","error":""}

===STATUS_CHECK_COMPLETE===
`
		results := job.parseDaemonStatus(output)

		require.Len(t, results, 1)
		assert.Equal(t, "01abc123", results[0].DaemonID)
	})

	t.Run("skip non-JSON lines", func(t *testing.T) {
		output := `Some random output
{"daemon_id":"01abc123","status":"RUNNING","pid":"12345","uptime_seconds":3600,"description":"","error":""}
More random output
===STATUS_CHECK_COMPLETE===`

		results := job.parseDaemonStatus(output)

		require.Len(t, results, 1)
		assert.Equal(t, "01abc123", results[0].DaemonID)
	})

	t.Run("handle empty output", func(t *testing.T) {
		output := ""
		results := job.parseDaemonStatus(output)
		assert.Empty(t, results)
	})

	t.Run("handle only completion marker", func(t *testing.T) {
		output := "===STATUS_CHECK_COMPLETE==="
		results := job.parseDaemonStatus(output)
		assert.Empty(t, results)
	})
}

func TestDaemonStatusInfo_JSON(t *testing.T) {
	info := DaemonStatusInfo{
		Uptime: "2 hours, 30 minutes",
		State:  "RUNNING",
		PID:    "12345",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var parsed DaemonStatusInfo
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2 hours, 30 minutes", parsed.Uptime)
	assert.Equal(t, "RUNNING", parsed.State)
	assert.Equal(t, "12345", parsed.PID)
	assert.Empty(t, parsed.Error)
}

func TestDaemonStatusInfo_JSON_WithError(t *testing.T) {
	info := DaemonStatusInfo{
		State: "STOPPED",
		Error: "Process exited with code 1",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var parsed DaemonStatusInfo
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "STOPPED", parsed.State)
	assert.Equal(t, "Process exited with code 1", parsed.Error)
	assert.Empty(t, parsed.Uptime)
	assert.Empty(t, parsed.PID)
}

func TestDaemonStatusResult_JSON(t *testing.T) {
	result := daemonStatusResult{
		DaemonID:      "01abc123",
		Status:        "RUNNING",
		PID:           "54321",
		UptimeSeconds: 7265,
		Description:   "pid 54321, uptime 2:01:05",
		Error:         "",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var parsed daemonStatusResult
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "01abc123", parsed.DaemonID)
	assert.Equal(t, "RUNNING", parsed.Status)
	assert.Equal(t, "54321", parsed.PID)
	assert.Equal(t, 7265, parsed.UptimeSeconds)
}
