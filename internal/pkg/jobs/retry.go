package jobs

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"
)

func IsFinalAttempt(ctx context.Context, err error) bool {
	retried, hasRetried := asynq.GetRetryCount(ctx)
	maxRetry, hasMaxRetry := asynq.GetMaxRetry(ctx)
	return finalAttemptWithCounts(err, retried, hasRetried, maxRetry, hasMaxRetry)
}

func finalAttemptWithCounts(err error, retried int, hasRetried bool, maxRetry int, hasMaxRetry bool) bool {
	if errors.Is(err, asynq.SkipRetry) {
		return true
	}
	if !hasRetried || !hasMaxRetry {
		return true
	}
	return retried >= maxRetry
}
