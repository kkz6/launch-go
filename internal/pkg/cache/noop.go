package cache

import (
	"context"
	"time"
)

// NoopCache implements Cache but does nothing. Useful for testing and route listing.
type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (c *NoopCache) Get(_ context.Context, _ string) (string, error) {
	return "", ErrCacheMiss
}

func (c *NoopCache) Set(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}

func (c *NoopCache) Delete(_ context.Context, _ string) error {
	return nil
}

func (c *NoopCache) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
