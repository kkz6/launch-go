package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// BaseCache provides generic caching functionality with JSON serialization.
// It wraps the Cache interface and provides type-safe operations for any type T.
type BaseCache[T any] struct {
	cache     Cache
	keyPrefix string
	ttl       time.Duration
}

// NewBaseCache creates a new BaseCache instance.
// The keyPrefix is prepended to all keys to namespace the cache entries.
// The ttl sets the default time-to-live for cached values.
func NewBaseCache[T any](cache Cache, keyPrefix string, ttl time.Duration) *BaseCache[T] {
	return &BaseCache[T]{
		cache:     cache,
		keyPrefix: keyPrefix,
		ttl:       ttl,
	}
}

// Key builds a cache key from parts by joining them with colons
// and prepending the key prefix.
func (c *BaseCache[T]) Key(parts ...string) string {
	if len(parts) == 0 {
		return c.keyPrefix
	}

	return c.keyPrefix + strings.Join(parts, ":")
}

// Get retrieves and unmarshals a value from the cache.
// Returns ErrCacheMiss if the key does not exist.
func (c *BaseCache[T]) Get(ctx context.Context, key string) (*T, error) {
	data, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var value T
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return nil, err
	}

	return &value, nil
}

// Set marshals and stores a value in the cache with the default TTL.
func (c *BaseCache[T]) Set(ctx context.Context, key string, value *T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.cache.Set(ctx, key, string(data), c.ttl)
}

// SetWithTTL marshals and stores a value in the cache with a custom TTL.
func (c *BaseCache[T]) SetWithTTL(ctx context.Context, key string, value *T, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.cache.Set(ctx, key, string(data), ttl)
}

// GetOrSet gets a value from the cache, or if not found, calls fn to compute
// the value, caches it, and returns it. This provides cache-aside functionality.
func (c *BaseCache[T]) GetOrSet(ctx context.Context, key string, fn func() (*T, error)) (*T, error) {
	value, err := c.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	if !errors.Is(err, ErrCacheMiss) {
		return nil, err
	}

	value, err = fn()
	if err != nil {
		return nil, err
	}

	if err := c.Set(ctx, key, value); err != nil {
		return value, nil
	}

	return value, nil
}

// GetOrSetWithTTL is like GetOrSet but allows specifying a custom TTL.
func (c *BaseCache[T]) GetOrSetWithTTL(ctx context.Context, key string, ttl time.Duration, fn func() (*T, error)) (*T, error) {
	value, err := c.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	if !errors.Is(err, ErrCacheMiss) {
		return nil, err
	}

	value, err = fn()
	if err != nil {
		return nil, err
	}

	if err := c.SetWithTTL(ctx, key, value, ttl); err != nil {
		return value, nil
	}

	return value, nil
}

// Delete removes a cached value.
func (c *BaseCache[T]) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}

// Exists checks if a key exists in the cache.
func (c *BaseCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	return c.cache.Exists(ctx, key)
}

// TTL returns the default TTL for this cache.
func (c *BaseCache[T]) TTL() time.Duration {
	return c.ttl
}

// KeyPrefix returns the key prefix for this cache.
func (c *BaseCache[T]) KeyPrefix() string {
	return c.keyPrefix
}
