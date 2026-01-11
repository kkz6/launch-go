package migrations

// registeredMigrations holds all registered migrations
var registeredMigrations []Migration

// Register adds a migration to the registry
func Register(migration Migration) {
	registeredMigrations = append(registeredMigrations, migration)
}
