package taskrunner

import (
	"fmt"
	"testing"
)

func TestTaskResult_IsSuccessful(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskResult
		expected bool
	}{
		{
			name:     "exit code 0, no error, no timeout",
			result:   &TaskResult{ExitCode: 0},
			expected: true,
		},
		{
			name:     "non-zero exit code",
			result:   &TaskResult{ExitCode: 1},
			expected: false,
		},
		{
			name:     "timed out",
			result:   &TaskResult{ExitCode: 124, TimedOut: true},
			expected: false,
		},
		{
			name:     "error set with zero exit code",
			result:   &TaskResult{ExitCode: 0, Error: fmt.Errorf("SSH monitoring failed")},
			expected: false,
		},
		{
			name:     "error set with non-zero exit code",
			result:   &TaskResult{ExitCode: 1, Error: fmt.Errorf("connection dropped")},
			expected: false,
		},
		{
			name:     "timed out with zero exit code",
			result:   &TaskResult{ExitCode: 0, TimedOut: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsSuccessful(); got != tt.expected {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskResult_IsFailed(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskResult
		expected bool
	}{
		{
			name:     "successful",
			result:   &TaskResult{ExitCode: 0},
			expected: false,
		},
		{
			name:     "non-zero exit code",
			result:   &TaskResult{ExitCode: 1},
			expected: true,
		},
		{
			name:     "error set",
			result:   &TaskResult{Error: fmt.Errorf("connection failed")},
			expected: true,
		},
		{
			name:     "zero exit code with error",
			result:   &TaskResult{ExitCode: 0, Error: fmt.Errorf("SSH drop")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsFailed(); got != tt.expected {
				t.Errorf("IsFailed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTaskResult_ErrorPreventsSuccessful verifies that when an error is set
// (e.g., from SSH monitoring failure), the result is NOT considered successful
// even if exit code is 0. This prevents a bug where SSH monitoring drops
// (exit code defaults to 0) from marking tasks as "finished".
func TestTaskResult_ErrorPreventsSuccessful(t *testing.T) {
	result := &TaskResult{
		ExitCode: 0,
		Error:    fmt.Errorf("SSH monitoring failed: connection reset"),
	}

	if result.IsSuccessful() {
		t.Fatal("IsSuccessful() should return false when Error is set, even with ExitCode 0")
	}
	if !result.IsFailed() {
		t.Fatal("IsFailed() should return true when Error is set")
	}
}
