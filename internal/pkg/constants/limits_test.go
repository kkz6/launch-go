package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginationLimits(t *testing.T) {
	t.Run("default pagination limit is reasonable", func(t *testing.T) {
		assert.Equal(t, 15, DefaultPaginationLimit)
		assert.Greater(t, DefaultPaginationLimit, 0)
	})

	t.Run("max pagination limit is greater than default", func(t *testing.T) {
		assert.Equal(t, 100, MaxPaginationLimit)
		assert.Greater(t, MaxPaginationLimit, DefaultPaginationLimit)
	})

	t.Run("API page sizes are appropriate", func(t *testing.T) {
		assert.Equal(t, 100, DefaultAPIPageSize)
		assert.Equal(t, 200, MaxAPIPageSize)
		assert.Greater(t, MaxAPIPageSize, DefaultAPIPageSize)
	})
}

func TestUploadLimits(t *testing.T) {
	t.Run("max upload size is 100 MB", func(t *testing.T) {
		assert.Equal(t, 100*MB, MaxUploadSize)
	})

	t.Run("max request body size is 10 MB", func(t *testing.T) {
		assert.Equal(t, 10*MB, MaxRequestBodySize)
	})

	t.Run("max log file size is 50 MB", func(t *testing.T) {
		assert.Equal(t, 50*MB, MaxLogFileSize)
	})

	t.Run("max backup file size is 5 GB", func(t *testing.T) {
		assert.Equal(t, 5*GB, MaxBackupFileSize)
	})
}

func TestByteUnits(t *testing.T) {
	t.Run("KB is 1024 bytes", func(t *testing.T) {
		assert.Equal(t, 1024, KB)
	})

	t.Run("MB is 1024 KB", func(t *testing.T) {
		assert.Equal(t, 1024*1024, MB)
		assert.Equal(t, 1024*KB, MB)
	})

	t.Run("GB is 1024 MB", func(t *testing.T) {
		assert.Equal(t, 1024*1024*1024, GB)
		assert.Equal(t, 1024*MB, GB)
	})
}

func TestQueryLimits(t *testing.T) {
	t.Run("default task limit is 50", func(t *testing.T) {
		assert.Equal(t, 50, DefaultTasksLimit)
	})

	t.Run("default metrics limit is 100", func(t *testing.T) {
		assert.Equal(t, 100, DefaultMetricsLimit)
	})

	t.Run("max deployments to keep is 10", func(t *testing.T) {
		assert.Equal(t, 10, MaxDeploymentsToKeep)
	})

	t.Run("max releases to keep is 5", func(t *testing.T) {
		assert.Equal(t, 5, MaxReleasesToKeep)
	})
}

func TestRetryLimits(t *testing.T) {
	t.Run("max retry attempts is 3", func(t *testing.T) {
		assert.Equal(t, 3, MaxRetryAttempts)
	})

	t.Run("max webhook retries is 5", func(t *testing.T) {
		assert.Equal(t, 5, MaxWebhookRetries)
	})

	t.Run("max provisioning retries is 2", func(t *testing.T) {
		assert.Equal(t, 2, MaxProvisioningRetries)
	})
}

func TestBufferSizes(t *testing.T) {
	t.Run("default buffer size is 32 KB", func(t *testing.T) {
		assert.Equal(t, 32*KB, DefaultBufferSize)
	})

	t.Run("SSH buffer size is 64 KB", func(t *testing.T) {
		assert.Equal(t, 64*KB, SSHBufferSize)
	})

	t.Run("WebSocket buffer size is 16 KB", func(t *testing.T) {
		assert.Equal(t, 16*KB, WebSocketBufferSize)
	})
}

func TestStringLengthLimits(t *testing.T) {
	t.Run("max name length is 100", func(t *testing.T) {
		assert.Equal(t, 100, MaxNameLength)
	})

	t.Run("max description length is 500", func(t *testing.T) {
		assert.Equal(t, 500, MaxDescriptionLength)
	})

	t.Run("max URL length is 2048", func(t *testing.T) {
		assert.Equal(t, 2048, MaxURLLength)
	})
}

func TestRateLimits(t *testing.T) {
	t.Run("default rate limit per minute is 60", func(t *testing.T) {
		assert.Equal(t, 60, DefaultRateLimitPerMinute)
	})

	t.Run("auth rate limit per minute is 10", func(t *testing.T) {
		assert.Equal(t, 10, AuthRateLimitPerMinute)
		assert.Less(t, AuthRateLimitPerMinute, DefaultRateLimitPerMinute)
	})

	t.Run("webhook rate limit per minute is 100", func(t *testing.T) {
		assert.Equal(t, 100, WebhookRateLimitPerMinute)
	})
}
