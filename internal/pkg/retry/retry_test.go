package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithBackoff_Success(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 3,
		Initial:     10 * time.Millisecond,
	}

	result, err := WithBackoff(ctx, cfg, func() (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
}

func TestWithBackoff_RetryUntilSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 5,
		Initial:     10 * time.Millisecond,
	}

	var attempts int32
	result, err := WithBackoff(ctx, cfg, func() (string, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			return "", errors.New("temporary failure")
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %q", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithBackoff_AllAttemptsFail(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 3,
		Initial:     10 * time.Millisecond,
	}

	var attempts int32
	expectedErr := errors.New("persistent failure")
	_, err := WithBackoff(ctx, cfg, func() (string, error) {
		atomic.AddInt32(&attempts, 1)
		return "", expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithBackoff_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		MaxAttempts: 10,
		Initial:     100 * time.Millisecond,
	}

	var attempts int32
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := WithBackoff(ctx, cfg, func() (string, error) {
		atomic.AddInt32(&attempts, 1)
		return "", errors.New("failure")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestWithBackoff_ExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 4,
		Initial:     10 * time.Millisecond,
		Max:         100 * time.Millisecond,
		Multiplier:  2.0,
	}

	var delays []time.Duration
	cfg.OnRetry = func(attempt int, err error, delay time.Duration) {
		delays = append(delays, delay)
	}

	var attempts int32
	_, _ = WithBackoff(ctx, cfg, func() (struct{}, error) {
		atomic.AddInt32(&attempts, 1)
		return struct{}{}, errors.New("failure")
	})

	// We should have 3 delays (between attempts 1-2, 2-3, 3-4)
	if len(delays) != 3 {
		t.Fatalf("expected 3 delays, got %d", len(delays))
	}

	// First delay should be around 10ms
	if delays[0] < 5*time.Millisecond || delays[0] > 15*time.Millisecond {
		t.Errorf("expected first delay around 10ms, got %v", delays[0])
	}

	// Second delay should be around 20ms
	if delays[1] < 15*time.Millisecond || delays[1] > 25*time.Millisecond {
		t.Errorf("expected second delay around 20ms, got %v", delays[1])
	}

	// Third delay should be around 40ms
	if delays[2] < 35*time.Millisecond || delays[2] > 45*time.Millisecond {
		t.Errorf("expected third delay around 40ms, got %v", delays[2])
	}
}

func TestWithBackoff_MaxDelayCap(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 5,
		Initial:     10 * time.Millisecond,
		Max:         25 * time.Millisecond,
		Multiplier:  2.0,
	}

	var delays []time.Duration
	cfg.OnRetry = func(attempt int, err error, delay time.Duration) {
		delays = append(delays, delay)
	}

	_, _ = WithBackoff(ctx, cfg, func() (struct{}, error) {
		return struct{}{}, errors.New("failure")
	})

	// Last delays should be capped at 25ms (with some tolerance for jitter-less check)
	for i := 2; i < len(delays); i++ {
		if delays[i] > 30*time.Millisecond {
			t.Errorf("expected delay %d to be capped at 25ms, got %v", i+1, delays[i])
		}
	}
}

func TestWithBackoff_RetryIfFunc(t *testing.T) {
	ctx := context.Background()

	retryableErr := errors.New("retryable error")
	nonRetryableErr := errors.New("non-retryable error")

	cfg := Config{
		MaxAttempts: 5,
		Initial:     10 * time.Millisecond,
		RetryIf: func(err error) bool {
			return errors.Is(err, retryableErr)
		},
	}

	var attempts int32

	// Test non-retryable error
	attempts = 0
	_, err := WithBackoff(ctx, cfg, func() (string, error) {
		atomic.AddInt32(&attempts, 1)
		return "", nonRetryableErr
	})

	if err != nonRetryableErr {
		t.Errorf("expected non-retryable error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}

	// Test retryable error
	attempts = 0
	_, err = WithBackoff(ctx, cfg, func() (string, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			return "", retryableErr
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 3,
		Initial:     10 * time.Millisecond,
	}

	var attempts int32
	err := Do(ctx, cfg, func() error {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 2 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoWithAttempts(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 5,
		Initial:     10 * time.Millisecond,
	}

	attempts, err := DoWithAttempts(ctx, cfg, func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}

	// Test with retries
	var counter int32
	attempts, err = DoWithAttempts(ctx, cfg, func() error {
		count := atomic.AddInt32(&counter, 1)
		if count < 3 {
			return errors.New("failure")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestUntil(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 10,
		Initial:     10 * time.Millisecond,
	}

	var attempts int32
	success := Until(ctx, cfg, func() bool {
		attempt := atomic.AddInt32(&attempts, 1)
		return attempt >= 3
	})

	if !success {
		t.Error("expected success")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestUntil_MaxAttemptsReached(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxAttempts: 3,
		Initial:     10 * time.Millisecond,
	}

	var attempts int32
	success := Until(ctx, cfg, func() bool {
		atomic.AddInt32(&attempts, 1)
		return false
	})

	if success {
		t.Error("expected failure")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestUntil_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		MaxAttempts: 100,
		Initial:     50 * time.Millisecond,
	}

	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	success := Until(ctx, cfg, func() bool {
		return false
	})

	if success {
		t.Error("expected failure due to cancellation")
	}
}

func TestBuilder(t *testing.T) {
	ctx := context.Background()

	var attempts int32
	err := New().
		WithMaxAttempts(5).
		WithInitialDelay(10*time.Millisecond).
		WithMaxDelay(100*time.Millisecond).
		Exponential(2.0).
		WithJitter(0.1).
		Do(ctx, func() error {
			attempt := atomic.AddInt32(&attempts, 1)
			if attempt < 3 {
				return errors.New("failure")
			}
			return nil
		})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDefaultConfigs(t *testing.T) {
	// Verify DefaultFixed
	if DefaultFixed.MaxAttempts != 3 {
		t.Errorf("DefaultFixed.MaxAttempts: expected 3, got %d", DefaultFixed.MaxAttempts)
	}
	if DefaultFixed.Initial != 500*time.Millisecond {
		t.Errorf("DefaultFixed.Initial: expected 500ms, got %v", DefaultFixed.Initial)
	}
	if DefaultFixed.Multiplier != 1.0 {
		t.Errorf("DefaultFixed.Multiplier: expected 1.0, got %v", DefaultFixed.Multiplier)
	}

	// Verify DefaultExponential
	if DefaultExponential.MaxAttempts != 30 {
		t.Errorf("DefaultExponential.MaxAttempts: expected 30, got %d", DefaultExponential.MaxAttempts)
	}
	if DefaultExponential.Initial != 10*time.Second {
		t.Errorf("DefaultExponential.Initial: expected 10s, got %v", DefaultExponential.Initial)
	}
	if DefaultExponential.Multiplier != 1.2 {
		t.Errorf("DefaultExponential.Multiplier: expected 1.2, got %v", DefaultExponential.Multiplier)
	}

	// Verify HTTPRetry
	if HTTPRetry.MaxAttempts != 3 {
		t.Errorf("HTTPRetry.MaxAttempts: expected 3, got %d", HTTPRetry.MaxAttempts)
	}
	if HTTPRetry.Jitter != 0.1 {
		t.Errorf("HTTPRetry.Jitter: expected 0.1, got %v", HTTPRetry.Jitter)
	}
}

func TestIsContextError(t *testing.T) {
	if !IsContextError(context.Canceled) {
		t.Error("expected context.Canceled to be recognized")
	}
	if !IsContextError(context.DeadlineExceeded) {
		t.Error("expected context.DeadlineExceeded to be recognized")
	}
	if IsContextError(errors.New("other error")) {
		t.Error("expected other errors not to be context errors")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := Config{}
	cfg = applyDefaults(cfg)

	if cfg.MaxAttempts != 3 {
		t.Errorf("expected default MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.Initial != 500*time.Millisecond {
		t.Errorf("expected default Initial 500ms, got %v", cfg.Initial)
	}
	if cfg.Multiplier != 1.0 {
		t.Errorf("expected default Multiplier 1.0, got %v", cfg.Multiplier)
	}
}

func TestApplyJitter(t *testing.T) {
	delay := 100 * time.Millisecond

	// No jitter
	result := applyJitter(delay, 0)
	if result != delay {
		t.Errorf("expected no jitter to return original delay, got %v", result)
	}

	// With jitter - should be within range
	jitter := 0.5
	for i := 0; i < 100; i++ {
		result := applyJitter(delay, jitter)
		minDelay := time.Duration(float64(delay) * (1 - jitter))
		maxDelay := time.Duration(float64(delay) * (1 + jitter))
		if result < minDelay || result > maxDelay {
			t.Errorf("jittered delay %v outside expected range [%v, %v]", result, minDelay, maxDelay)
		}
	}
}
