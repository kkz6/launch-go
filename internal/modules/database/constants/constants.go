package constants

// Database types
const (
	TypeMySQL      = "mysql"
	TypePostgreSQL = "postgresql"
	TypeMariaDB    = "mariadb"
)

// AllDatabaseTypes returns all supported database types
var AllDatabaseTypes = []string{TypeMySQL, TypePostgreSQL, TypeMariaDB}

// Database ports
const (
	PortMySQL      = 3306
	PortPostgreSQL = 5432
	PortMariaDB    = 3306
)

// Default settings
const (
	DefaultMaxConnections = 100
	DefaultCharset        = "utf8mb4"
	DefaultCollation      = "utf8mb4_unicode_ci"
)

// User privileges
const (
	PrivilegeAll       = "ALL PRIVILEGES"
	PrivilegeReadOnly  = "SELECT"
	PrivilegeReadWrite = "SELECT, INSERT, UPDATE, DELETE"
)

// Backup settings
const (
	BackupCompressionEnabled = true
	BackupRetentionDays      = 7
)
