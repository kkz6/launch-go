package models

// AllModels returns all models for migration
func AllModels() []interface{} {
	return []interface{}{
		&Server{},
		&InstalledService{},
		&FirewallRule{},
		&Cron{},
		&Daemon{},
		&SshKey{},
		&ServerSshKey{},
		&Task{},
		&Metric{},
	}
}
