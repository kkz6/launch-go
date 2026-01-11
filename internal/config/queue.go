package config

import "github.com/spf13/viper"

// QueueConfig holds queue configuration
type QueueConfig struct {
	Concurrency int
}

func loadQueueConfig() QueueConfig {
	return QueueConfig{
		Concurrency: viper.GetInt("QUEUE_CONCURRENCY"),
	}
}

func setQueueDefaults() {
	viper.SetDefault("QUEUE_CONCURRENCY", 10)
}
