package health

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockChecker is a mock implementation of the Checker interface for testing
type mockChecker struct {
	name   string
	err    error
	delay  time.Duration
	called atomic.Int32
}

type blockingChecker struct {
	name    string
	release <-chan struct{}
}

func (c *blockingChecker) Name() string { return c.name }

func (c *blockingChecker) Check(context.Context) error {
	<-c.release
	return nil
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Check(ctx context.Context) error {
	m.called.Add(1)

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return m.err
}

func TestStatus_Constants(t *testing.T) {
	t.Run("status values are correct", func(t *testing.T) {
		assert.Equal(t, Status("healthy"), StatusHealthy)
		assert.Equal(t, Status("degraded"), StatusDegraded)
		assert.Equal(t, Status("unhealthy"), StatusUnhealthy)
	})
}

func TestNewAggregator(t *testing.T) {
	t.Run("creates empty aggregator", func(t *testing.T) {
		agg := NewAggregator()

		assert.NotNil(t, agg)
		assert.Empty(t, agg.checkers)
	})
}

func TestAggregator_Add(t *testing.T) {
	t.Run("adds checker to aggregator", func(t *testing.T) {
		agg := NewAggregator()
		checker := &mockChecker{name: "test"}

		result := agg.Add(checker)

		assert.Same(t, agg, result, "Add should return the aggregator for chaining")
		assert.Len(t, agg.checkers, 1)
	})

	t.Run("supports method chaining", func(t *testing.T) {
		agg := NewAggregator()
		checker1 := &mockChecker{name: "test1"}
		checker2 := &mockChecker{name: "test2"}

		agg.Add(checker1).Add(checker2)

		assert.Len(t, agg.checkers, 2)
	})

	t.Run("adds multiple checkers", func(t *testing.T) {
		agg := NewAggregator()

		for i := 0; i < 5; i++ {
			agg.Add(&mockChecker{name: "test"})
		}

		assert.Len(t, agg.checkers, 5)
	})
}

func TestAggregator_Check(t *testing.T) {
	t.Run("returns healthy when no checkers registered", func(t *testing.T) {
		agg := NewAggregator()
		ctx := context.Background()

		result := agg.Check(ctx)

		assert.Equal(t, StatusHealthy, result.Status)
		assert.Empty(t, result.Checks)
		assert.False(t, result.Timestamp.IsZero())
	})

	t.Run("returns healthy when all checks pass", func(t *testing.T) {
		agg := NewAggregator()
		agg.Add(&mockChecker{name: "db"})
		agg.Add(&mockChecker{name: "redis"})
		ctx := context.Background()

		result := agg.Check(ctx)

		assert.Equal(t, StatusHealthy, result.Status)
		assert.Len(t, result.Checks, 2)

		for _, check := range result.Checks {
			assert.Equal(t, StatusHealthy, check.Status)
			assert.Empty(t, check.Message)
		}
	})

	t.Run("returns degraded when one check fails", func(t *testing.T) {
		agg := NewAggregator()
		agg.Add(&mockChecker{name: "db"})
		agg.Add(&mockChecker{name: "redis", err: errors.New("connection refused")})
		ctx := context.Background()

		result := agg.Check(ctx)

		assert.Equal(t, StatusDegraded, result.Status)
		assert.Len(t, result.Checks, 2)

		var healthyCount, unhealthyCount int
		for _, check := range result.Checks {
			switch check.Status {
			case StatusHealthy:
				healthyCount++
			case StatusUnhealthy:
				unhealthyCount++
				assert.Equal(t, "connection refused", check.Message)
			}
		}

		assert.Equal(t, 1, healthyCount)
		assert.Equal(t, 1, unhealthyCount)
	})

	t.Run("returns degraded when all checks fail", func(t *testing.T) {
		agg := NewAggregator()
		agg.Add(&mockChecker{name: "db", err: errors.New("db error")})
		agg.Add(&mockChecker{name: "redis", err: errors.New("redis error")})
		ctx := context.Background()

		result := agg.Check(ctx)

		assert.Equal(t, StatusDegraded, result.Status)

		for _, check := range result.Checks {
			assert.Equal(t, StatusUnhealthy, check.Status)
			assert.NotEmpty(t, check.Message)
		}
	})

	t.Run("records latency for each check", func(t *testing.T) {
		agg := NewAggregator()
		agg.Add(&mockChecker{name: "slow", delay: 10 * time.Millisecond})
		ctx := context.Background()

		result := agg.Check(ctx)

		assert.Len(t, result.Checks, 1)
		assert.GreaterOrEqual(t, result.Checks[0].Latency, 10*time.Millisecond)
	})

	t.Run("runs checks concurrently", func(t *testing.T) {
		agg := NewAggregator()
		delay := 50 * time.Millisecond
		agg.Add(&mockChecker{name: "check1", delay: delay})
		agg.Add(&mockChecker{name: "check2", delay: delay})
		agg.Add(&mockChecker{name: "check3", delay: delay})
		ctx := context.Background()

		start := time.Now()
		result := agg.Check(ctx)
		elapsed := time.Since(start)

		assert.Len(t, result.Checks, 3)
		assert.Less(t, elapsed, 3*delay, "checks should run concurrently")
	})

	t.Run("preserves checker names", func(t *testing.T) {
		agg := NewAggregator()
		agg.Add(&mockChecker{name: "database"})
		agg.Add(&mockChecker{name: "cache"})
		ctx := context.Background()

		result := agg.Check(ctx)

		names := make(map[string]bool)
		for _, check := range result.Checks {
			names[check.Name] = true
		}

		assert.True(t, names["database"])
		assert.True(t, names["cache"])
	})

	t.Run("sets timestamp in UTC", func(t *testing.T) {
		agg := NewAggregator()
		ctx := context.Background()

		before := time.Now().UTC()
		result := agg.Check(ctx)
		after := time.Now().UTC()

		assert.True(t, result.Timestamp.Equal(before) || result.Timestamp.After(before))
		assert.True(t, result.Timestamp.Equal(after) || result.Timestamp.Before(after))
		assert.Equal(t, time.UTC, result.Timestamp.Location())
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		agg := NewAggregator()
		agg.Add(&mockChecker{name: "slow", delay: 5 * time.Second})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		result := agg.Check(ctx)

		assert.Equal(t, StatusDegraded, result.Status)
		assert.Len(t, result.Checks, 1)
		assert.Equal(t, StatusUnhealthy, result.Checks[0].Status)
	})

	t.Run("returns when a checker ignores cancellation", func(t *testing.T) {
		agg := NewAggregator()
		release := make(chan struct{})
		agg.Add(&blockingChecker{name: "stuck", release: release})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		defer close(release)

		start := time.Now()
		result := agg.Check(ctx)

		assert.Less(t, time.Since(start), time.Second)
		assert.Equal(t, StatusDegraded, result.Status)
		assert.Equal(t, context.DeadlineExceeded.Error(), result.Checks[0].Message)
	})

	t.Run("calls each checker exactly once", func(t *testing.T) {
		checker1 := &mockChecker{name: "check1"}
		checker2 := &mockChecker{name: "check2"}
		agg := NewAggregator()
		agg.Add(checker1).Add(checker2)
		ctx := context.Background()

		agg.Check(ctx)

		assert.Equal(t, int32(1), checker1.called.Load())
		assert.Equal(t, int32(1), checker2.called.Load())
	})
}

func TestAggregator_ConcurrentAccess(t *testing.T) {
	t.Run("supports concurrent Add and Check operations", func(t *testing.T) {
		agg := NewAggregator()
		ctx := context.Background()
		done := make(chan bool)

		go func() {
			for i := 0; i < 100; i++ {
				agg.Add(&mockChecker{name: "concurrent"})
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				agg.Check(ctx)
			}
			done <- true
		}()

		<-done
		<-done
	})
}
