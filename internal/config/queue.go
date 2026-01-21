package config

// QueueConfig holds queue configuration
type QueueConfig struct {
	Concurrency int `env:"QUEUE_CONCURRENCY" default:"10"`
}
