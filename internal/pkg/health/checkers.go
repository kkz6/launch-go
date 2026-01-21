package health

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// DatabaseChecker checks the health of a database connection
type DatabaseChecker struct {
	DB *gorm.DB
}

// Name returns the name of the database health check
func (c *DatabaseChecker) Name() string {
	return "database"
}

// Check performs the database health check by pinging the database
func (c *DatabaseChecker) Check(ctx context.Context) error {
	if c.DB == nil {
		return fmt.Errorf("database connection is nil")
	}

	sqlDB, err := c.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// RedisChecker checks the health of a Redis connection
type RedisChecker struct {
	Client redis.UniversalClient
}

// Name returns the name of the Redis health check
func (c *RedisChecker) Name() string {
	return "redis"
}

// Check performs the Redis health check by pinging the Redis server
func (c *RedisChecker) Check(ctx context.Context) error {
	if c.Client == nil {
		return fmt.Errorf("redis client is nil")
	}

	if err := c.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}
