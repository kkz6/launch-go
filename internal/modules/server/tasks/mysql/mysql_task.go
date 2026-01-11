package mysql

import (
	"errors"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// GetServerPublicIPv4 safely gets the server's public IPv4 address
func GetServerPublicIPv4(server *models.Server) string {
	if server.PublicIPv4 != nil {
		return *server.PublicIPv4
	}

	return ""
}

// GetServerDatabasePassword safely gets the server's database password
func GetServerDatabasePassword(server *models.Server) string {
	if server.DatabasePassword != nil {
		return *server.DatabasePassword
	}

	return ""
}

// GetHostsForServer returns the hosts list for MySQL user operations on a server
func GetHostsForServer(server *models.Server) []string {
	ip := GetServerPublicIPv4(server)
	if ip != "" {
		return []string{ip, "%"}
	}

	return []string{"%"}
}

// ValidateCredentials checks if credentials are set
func ValidateCredentials(user, password string) error {
	if user == "" || password == "" {
		return errors.New("forgot to set the user or password")
	}

	return nil
}

// WrapValue prepares a value for use in a SQL query by wrapping it in backticks
// and escaping any existing backticks
func WrapValue(value string) string {
	if value == "*" {
		return value
	}

	// Replace single quotes with backticks
	value = strings.ReplaceAll(value, "'", "`")

	// Replace backticks with double backticks and wrap in backticks
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

// BuildMySQLCommand builds the mysql command with credentials and SQL query
func BuildMySQLCommand(user, password, sql string) string {
	return "MYSQL_PWD=" + password + " mysql --user=" + user + " --execute='" + sql + "' --skip-column-names"
}
