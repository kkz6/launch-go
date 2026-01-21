package models

// AllModels returns all models for migration
func AllModels() []interface{} {
	return []interface{}{
		&Server{},
		&InstalledService{},
		&FirewallRule{},
		&Cron{},
		&Daemon{},
		&SSHKey{},
		&ServerSSHKey{},
		&Task{},
		&Metric{},
		&Script{},
		&ScriptExecution{},
		&Monitor{},
	}
}
