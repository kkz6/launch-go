package models

// AllModels returns all models for migration
func AllModels() []interface{} {
	return []interface{}{
		&Database{},
		&DatabaseUser{},
		&DatabaseDatabaseUser{},
	}
}
