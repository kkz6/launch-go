package cache

import (
	"context"
	"time"
)

// Cache defines the interface for caching operations
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// ErrCacheMiss is returned when a key is not found in the cache
var ErrCacheMiss = &CacheMissError{}

type CacheMissError struct{}

func (e *CacheMissError) Error() string {
	return "cache miss"
}
