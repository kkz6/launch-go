package httpclient

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryableError is an interface for errors that can indicate retryability.
type RetryableError interface {
	error
	Retryable() bool
}

// RetryOptions configures retry behavior.
type RetryOptions struct {
	// MaxAttempts is the maximum number of attempts (including the initial request).
	// Default: 3
	MaxAttempts int

	// InitialDelay is the initial delay before the first retry.
	// Default: 100ms
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	// Default: 10s
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases after each retry.
	// Default: 2.0
	Multiplier float64

	// Jitter adds randomness to the delay to prevent thundering herd.
	// Value between 0 and 1 (0 = no jitter, 1 = full jitter).
	// Default: 0.1
	Jitter float64

	// RetryIf is a custom function to determine if a request should be retried.
	// If nil, uses default retry logic (retry on 5xx and network errors).
	RetryIf func(resp *http.Response, err error) bool

	// OnRetry is called before each retry attempt.
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultRetryOptions returns sensible default retry options.
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// retryableError wraps an error to indicate it's retryable.
type retryableError struct {
	err error
}

func (e *retryableError) Error() string {
	return e.err.Error()
}

func (e *retryableError) Retryable() bool {
	return true
}

func (e *retryableError) Unwrap() error {
	return e.err
}

// NewRetryableError wraps an error to indicate it should be retried.
func NewRetryableError(err error) error {
	return &retryableError{err: err}
}

// IsRetryable checks if an error is marked as retryable.
func IsRetryable(err error) bool {
	var re RetryableError
	if errors.As(err, &re) {
		return re.Retryable()
	}

	// Check for HTTP errors
	if httpErr, ok := IsHTTPError(err); ok {
		return isRetryableStatusCode(httpErr.StatusCode)
	}

	return false
}

// isRetryableStatusCode checks if an HTTP status code should trigger a retry.
func isRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// shouldRetry determines if a request should be retried based on the response and error.
func shouldRetry(opts *RetryOptions, resp *http.Response, err error) bool {
	if opts.RetryIf != nil {
		return opts.RetryIf(resp, err)
	}

	// Network errors are retryable
	if err != nil {
		// Don't retry context cancellation or deadline exceeded
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true
	}

	// Retry on specific status codes
	if resp != nil {
		return isRetryableStatusCode(resp.StatusCode)
	}

	return false
}

// calculateDelay calculates the delay for the given attempt using exponential backoff.
func calculateDelay(opts *RetryOptions, attempt int) time.Duration {
	// Exponential backoff: initialDelay * multiplier^attempt
	delay := float64(opts.InitialDelay) * math.Pow(opts.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(opts.MaxDelay) {
		delay = float64(opts.MaxDelay)
	}

	// Apply jitter
	if opts.Jitter > 0 {
		jitterRange := delay * opts.Jitter
		jitter := (rand.Float64()*2 - 1) * jitterRange // Random value in [-jitterRange, +jitterRange]
		delay += jitter
	}

	// Ensure delay is not negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// doWithRetry executes a request with retry logic.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	opts := c.retryOpts
	if opts == nil {
		return c.httpClient.Do(req)
	}

	// Fill in defaults
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.InitialDelay <= 0 {
		opts.InitialDelay = 100 * time.Millisecond
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 10 * time.Second
	}
	if opts.Multiplier <= 0 {
		opts.Multiplier = 2.0
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
		// Clone the request for retry (body needs to be reset)
		reqClone := req.Clone(req.Context())
		if req.Body != nil {
			// GetBody is set by http.NewRequest for certain body types
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				reqClone.Body = body
			}
		}

		resp, err := c.httpClient.Do(reqClone)

		// Success - return immediately
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// Check if we should retry
		if !shouldRetry(opts, resp, err) {
			if err != nil {
				return nil, err
			}
			return resp, nil
		}

		// Close the response body if we got one (we'll retry)
		if resp != nil {
			_ = resp.Body.Close()
		}

		lastErr = err
		lastResp = resp

		// Don't delay after the last attempt
		if attempt < opts.MaxAttempts-1 {
			delay := calculateDelay(opts, attempt)

			if opts.OnRetry != nil {
				retryErr := err
				if retryErr == nil && resp != nil {
					retryErr = &HTTPError{
						StatusCode: resp.StatusCode,
						Status:     resp.Status,
					}
				}
				opts.OnRetry(attempt+1, retryErr, delay)
			}

			// Wait before retrying
			timer := time.NewTimer(delay)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}
		}
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

// Retry executes a function with retry logic.
func Retry(ctx context.Context, opts RetryOptions, fn func() error) error {
	// Fill in defaults
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.InitialDelay <= 0 {
		opts.InitialDelay = 100 * time.Millisecond
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 10 * time.Second
	}
	if opts.Multiplier <= 0 {
		opts.Multiplier = 2.0
	}

	var lastErr error

	for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}

		lastErr = err

		// Don't delay after the last attempt
		if attempt < opts.MaxAttempts-1 {
			delay := calculateDelay(&opts, attempt)

			if opts.OnRetry != nil {
				opts.OnRetry(attempt+1, err, delay)
			}

			// Wait before retrying
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return lastErr
}

// RetryWithResult executes a function with retry logic that returns a result.
func RetryWithResult[T any](ctx context.Context, opts RetryOptions, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	// Fill in defaults
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.InitialDelay <= 0 {
		opts.InitialDelay = 100 * time.Millisecond
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 10 * time.Second
	}
	if opts.Multiplier <= 0 {
		opts.Multiplier = 2.0
	}

	for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Check if error is retryable
		if !IsRetryable(lastErr) {
			return result, lastErr
		}

		// Don't delay after the last attempt
		if attempt < opts.MaxAttempts-1 {
			delay := calculateDelay(&opts, attempt)

			if opts.OnRetry != nil {
				opts.OnRetry(attempt+1, lastErr, delay)
			}

			// Wait before retrying
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return result, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return result, lastErr
}
