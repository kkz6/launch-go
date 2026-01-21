package queue

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/config"
)

// Scheduler wraps asynq.Scheduler for periodic task scheduling
type Scheduler struct {
	*asynq.Scheduler
}

// NewScheduler creates a new scheduler instance
func NewScheduler(cfg config.RedisConfig) *Scheduler {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	return &Scheduler{
		Scheduler: asynq.NewScheduler(redisOpt, nil),
	}
}

// ScheduledTask represents a task to be scheduled
type ScheduledTask struct {
	CronSpec string
	Task     *asynq.Task
	Opts     []asynq.Option
}

// RegisterTasks registers multiple scheduled tasks
func (s *Scheduler) RegisterTasks(tasks []ScheduledTask) error {
	for _, t := range tasks {
		_, err := s.Register(t.CronSpec, t.Task, t.Opts...)
		if err != nil {
			return err
		}
	}
	return nil
}
