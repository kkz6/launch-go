package queue

import (
	"errors"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/config"
)

type Client struct {
	*asynq.Client
	inspector *asynq.Inspector
}

func NewClient(cfg config.RedisConfig) *Client {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	return &Client{
		Client:    asynq.NewClient(redisOpt),
		inspector: asynq.NewInspector(redisOpt),
	}
}

func (c *Client) Inspector() *asynq.Inspector {
	return c.inspector
}

// Close releases both Redis connections owned by the queue client. The
// embedded asynq.Client only closes its own connection; the inspector has a
// separate pool and must be closed explicitly during application shutdown.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	var errs []error
	if c.Client != nil {
		errs = append(errs, c.Client.Close())
	}
	if c.inspector != nil {
		errs = append(errs, c.inspector.Close())
	}
	return errors.Join(errs...)
}

// Queue names
const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

// Enqueue helpers
func (c *Client) EnqueueCritical(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.Queue(QueueCritical))
	return c.Enqueue(task, opts...)
}

func (c *Client) EnqueueDefault(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.Queue(QueueDefault))
	return c.Enqueue(task, opts...)
}

func (c *Client) EnqueueLow(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.Queue(QueueLow))
	return c.Enqueue(task, opts...)
}
