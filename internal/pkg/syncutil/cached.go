package syncutil

import "sync"

// Cached provides thread-safe lazy initialization with double-check locking.
// It stores a value that is computed once and cached for subsequent reads.
type Cached[T any] struct {
	mu    sync.RWMutex
	value *T
}

// GetOrCompute returns the cached value or computes it using the provided function.
// This uses the double-check locking pattern for efficient thread-safe access.
func (c *Cached[T]) GetOrCompute(compute func() T) T {
	c.mu.RLock()
	if c.value != nil {
		defer c.mu.RUnlock()
		return *c.value
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.value != nil {
		return *c.value
	}

	result := compute()
	c.value = &result
	return result
}

// Get returns the cached value and a boolean indicating if it was set.
// This does not compute the value if not cached.
func (c *Cached[T]) Get() (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.value == nil {
		var zero T
		return zero, false
	}
	return *c.value, true
}

// Invalidate clears the cached value, forcing recomputation on next access.
func (c *Cached[T]) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = nil
}

// Set explicitly sets the cached value.
func (c *Cached[T]) Set(value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = &value
}

// IsSet returns true if the cached value has been set.
func (c *Cached[T]) IsSet() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value != nil
}
