package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// Config defines the retry behavior with configurable backoff strategies.
type Config struct {
	// MaxAttempts is the maximum number of attempts (including the initial attempt).
	// Default: 3
	MaxAttempts int

	// Initial is the initial delay before the first retry.
	// Default: 500ms
	Initial time.Duration

	// Max is the maximum delay between retries.
	// Default: 30s
	Max time.Duration

	// Multiplier is the factor by which the delay increases after each retry.
	// Use 1.0 for fixed delay, >1.0 for exponential backoff.
	// Default: 1.0 (fixed delay)
	Multiplier float64

	// Jitter adds randomness to the delay to prevent thundering herd.
	// Value between 0 and 1 (0 = no jitter, 1 = full jitter).
	// Default: 0
	Jitter float64

	// RetryIf is a custom function to determine if an error should be retried.
	// If nil, all errors are retried.
	RetryIf func(err error) bool

	// OnRetry is called before each retry attempt.
	OnRetry func(attempt int, err error, delay time.Duration)
}

// Preset configurations for common use cases.
var (
	// DefaultFixed provides a simple fixed-delay retry configuration.
	// 3 attempts with 500ms delay between each.
	DefaultFixed = Config{
		MaxAttempts: 3,
		Initial:     500 * time.Millisecond,
		Multiplier:  1.0,
	}

	// DefaultExponential provides exponential backoff configuration.
	// 30 attempts starting at 10s, multiplying by 1.2, capped at 30s.
	// Useful for waiting for external services to become available.
	DefaultExponential = Config{
		MaxAttempts: 30,
		Initial:     10 * time.Second,
		Max:         30 * time.Second,
		Multiplier:  1.2,
	}

	// HTTPRetry provides configuration suitable for HTTP requests.
	// 3 attempts with exponential backoff starting at 100ms, capped at 10s.
	HTTPRetry = Config{
		MaxAttempts: 3,
		Initial:     100 * time.Millisecond,
		Max:         10 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.1,
	}

	// DBRetry provides configuration suitable for database operations.
	// 3 attempts with 500ms fixed delay.
	DBRetry = Config{
		MaxAttempts: 3,
		Initial:     500 * time.Millisecond,
		Multiplier:  1.0,
	}

	// ServerConnectionRetry provides configuration for waiting for server connectivity.
	// 30 attempts starting at 10s, exponential backoff capped at 30s.
	ServerConnectionRetry = Config{
		MaxAttempts: 30,
		Initial:     10 * time.Second,
		Max:         30 * time.Second,
		Multiplier:  1.2,
	}
)

// WithBackoff executes a function with retry logic and returns a result.
// It uses the provided configuration to determine retry behavior.
func WithBackoff[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	cfg = applyDefaults(cfg)
	delay := cfg.Initial

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Check if error is retryable
		if cfg.RetryIf != nil && !cfg.RetryIf(lastErr) {
			return result, lastErr
		}

		// Don't delay after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Apply jitter to the delay
		actualDelay := applyJitter(delay, cfg.Jitter)

		// Call OnRetry callback if provided
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt, lastErr, actualDelay)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(actualDelay):
		}

		// Calculate next delay
		if cfg.Multiplier > 1.0 {
			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if cfg.Max > 0 && delay > cfg.Max {
				delay = cfg.Max
			}
		}
	}

	return result, lastErr
}

// Do executes a function with retry logic that doesn't return a value.
// This is a convenience wrapper around WithBackoff.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	_, err := WithBackoff(ctx, cfg, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// DoWithAttempts is like Do but also returns the number of attempts made.
func DoWithAttempts(ctx context.Context, cfg Config, fn func() error) (int, error) {
	cfg = applyDefaults(cfg)
	delay := cfg.Initial
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return attempt - 1, err
		}
		lastErr = fn()
		if lastErr == nil {
			return attempt, nil
		}

		// Check if error is retryable
		if cfg.RetryIf != nil && !cfg.RetryIf(lastErr) {
			return attempt, lastErr
		}

		// Don't delay after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Apply jitter to the delay
		actualDelay := applyJitter(delay, cfg.Jitter)

		// Call OnRetry callback if provided
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt, lastErr, actualDelay)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return attempt, ctx.Err()
		case <-time.After(actualDelay):
		}

		// Calculate next delay
		if cfg.Multiplier > 1.0 {
			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if cfg.Max > 0 && delay > cfg.Max {
				delay = cfg.Max
			}
		}
	}

	return cfg.MaxAttempts, lastErr
}

// Until retries until the function returns true or the context is cancelled.
// The function should return true to stop retrying, false to continue.
func Until(ctx context.Context, cfg Config, fn func() bool) bool {
	cfg = applyDefaults(cfg)
	delay := cfg.Initial

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return false
		}
		if fn() {
			return true
		}

		// Don't delay after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Apply jitter to the delay
		actualDelay := applyJitter(delay, cfg.Jitter)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return false
		case <-time.After(actualDelay):
		}

		// Calculate next delay
		if cfg.Multiplier > 1.0 {
			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if cfg.Max > 0 && delay > cfg.Max {
				delay = cfg.Max
			}
		}
	}

	return false
}

// IsContextError returns true if the error is due to context cancellation or deadline.
func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// applyDefaults fills in default values for unset configuration options.
func applyDefaults(cfg Config) Config {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.Initial <= 0 {
		cfg.Initial = 500 * time.Millisecond
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 1.0
	}
	return cfg
}

// applyJitter adds random jitter to a delay.
func applyJitter(delay time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return delay
	}

	// Calculate jitter range: [-jitterRange, +jitterRange]
	jitterRange := float64(delay) * jitter
	jitterAmount := (rand.Float64()*2 - 1) * jitterRange
	result := float64(delay) + jitterAmount

	// Ensure delay is not negative
	if result < 0 {
		return 0
	}

	return time.Duration(result)
}

// Builder provides a fluent interface for configuring retry behavior.
type Builder struct {
	cfg Config
}

// New creates a new retry builder with default configuration.
func New() *Builder {
	return &Builder{
		cfg: Config{
			MaxAttempts: 3,
			Initial:     500 * time.Millisecond,
			Multiplier:  1.0,
		},
	}
}

// WithMaxAttempts sets the maximum number of attempts.
func (b *Builder) WithMaxAttempts(n int) *Builder {
	b.cfg.MaxAttempts = n
	return b
}

// WithInitialDelay sets the initial delay between retries.
func (b *Builder) WithInitialDelay(d time.Duration) *Builder {
	b.cfg.Initial = d
	return b
}

// WithMaxDelay sets the maximum delay between retries.
func (b *Builder) WithMaxDelay(d time.Duration) *Builder {
	b.cfg.Max = d
	return b
}

// WithMultiplier sets the backoff multiplier.
func (b *Builder) WithMultiplier(m float64) *Builder {
	b.cfg.Multiplier = m
	return b
}

// WithJitter sets the jitter factor (0-1).
func (b *Builder) WithJitter(j float64) *Builder {
	b.cfg.Jitter = j
	return b
}

// WithRetryIf sets a custom function to determine if an error should be retried.
func (b *Builder) WithRetryIf(fn func(err error) bool) *Builder {
	b.cfg.RetryIf = fn
	return b
}

// WithOnRetry sets a callback to be called before each retry.
func (b *Builder) WithOnRetry(fn func(attempt int, err error, delay time.Duration)) *Builder {
	b.cfg.OnRetry = fn
	return b
}

// Exponential configures exponential backoff with the given multiplier.
func (b *Builder) Exponential(multiplier float64) *Builder {
	b.cfg.Multiplier = multiplier
	return b
}

// Fixed configures fixed delay (multiplier = 1.0).
func (b *Builder) Fixed() *Builder {
	b.cfg.Multiplier = 1.0
	return b
}

// Config returns the built configuration.
func (b *Builder) Config() Config {
	return b.cfg
}

// Do executes the function with the configured retry behavior.
func (b *Builder) Do(ctx context.Context, fn func() error) error {
	return Do(ctx, b.cfg, fn)
}

// Run executes the function with the configured retry behavior and returns a result.
func (b *Builder) Run(ctx context.Context, fn func() (any, error)) (any, error) {
	return WithBackoff(ctx, b.cfg, fn)
}
