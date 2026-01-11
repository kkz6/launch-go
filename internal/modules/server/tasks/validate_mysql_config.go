package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

// ValidateMySqlConfig validates a MySQL configuration file
type ValidateMySqlConfig struct {
	taskrunner.BaseTask
	mysqlConfig string
	path        string
}

// NewValidateMySqlConfig creates a new ValidateMySqlConfig task
func NewValidateMySqlConfig(mysqlConfig string) *ValidateMySqlConfig {
	task := &ValidateMySqlConfig{
		BaseTask: taskrunner.BaseTask{
			TaskName:     "validate-mysql-config",
			TemplateName: "server/validate-mysql-config",
			TaskTimeout:  2 * time.Minute,
		},
		mysqlConfig: mysqlConfig,
		path:        generateTempMySqlConfigPath(),
	}

	return task
}

// Data returns the template data
func (t *ValidateMySqlConfig) Data() map[string]interface{} {
	return map[string]interface{}{
		"MySqlConfig": t.mysqlConfig,
		"Path":        t.path,
	}
}

// MySqlConfig returns the MySQL configuration content being validated
func (t *ValidateMySqlConfig) MySqlConfig() string {
	return t.mysqlConfig
}

// Path returns the temporary file path for validation
func (t *ValidateMySqlConfig) Path() string {
	return t.path
}

// generateTempMySqlConfigPath generates a unique temporary file path
func generateTempMySqlConfigPath() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "/tmp/mysql-default.cnf"
	}

	return fmt.Sprintf("/tmp/mysql-%s.cnf", hex.EncodeToString(bytes))
}
