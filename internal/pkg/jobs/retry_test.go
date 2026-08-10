package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

func TestIsFinalAttemptWithoutWorkerMetadata(t *testing.T) {
	require.True(t, IsFinalAttempt(context.Background(), errors.New("failed")))
}

func TestIsFinalAttemptDecision(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		retried      int
		hasRetried   bool
		maxRetry     int
		hasMaxRetry  bool
		finalAttempt bool
	}{
		{
			name:         "skip retry",
			err:          fmt.Errorf("permanent: %w", asynq.SkipRetry),
			finalAttempt: true,
		},
		{
			name:         "missing retry count",
			hasMaxRetry:  true,
			finalAttempt: true,
		},
		{
			name:         "missing max retry",
			hasRetried:   true,
			finalAttempt: true,
		},
		{
			name:         "retry remains",
			retried:      1,
			hasRetried:   true,
			maxRetry:     2,
			hasMaxRetry:  true,
			finalAttempt: false,
		},
		{
			name:         "retry exhausted",
			retried:      2,
			hasRetried:   true,
			maxRetry:     2,
			hasMaxRetry:  true,
			finalAttempt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.finalAttempt, finalAttemptWithCounts(
				tt.err,
				tt.retried,
				tt.hasRetried,
				tt.maxRetry,
				tt.hasMaxRetry,
			))
		})
	}
}
