package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Task types for database operations
const (
	TypeInstallDatabase      = "database:install"
	TypeUninstallDatabase    = "database:uninstall"
	TypeInstallDatabaseUser  = "database:user:install"
	TypeUpdateDatabaseUser   = "database:user:update"
	TypeUninstallDatabaseUser = "database:user:uninstall"
	TypeSyncDatabases        = "database:sync"
)

// Handler handles database job processing
type Handler struct {
	db     *gorm.DB
	ws     *websocket.Hub
	logger *zerolog.Logger
}

// NewHandler creates a new database job handler
func NewHandler(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		ws:     ws,
		logger: logger,
	}
}

// InstallDatabasePayload contains the data for installing a database
type InstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// NewInstallDatabaseTask creates a new install database task
func NewInstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDatabase, payload), nil
}

// HandleInstallDatabase handles the database installation job
func (h *Handler) HandleInstallDatabase(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("database_id", payload.DatabaseID).Msg("Installing database")

	// TODO: Implement actual database creation via SSH
	// 1. Get database and server info
	// 2. Connect via SSH
	// 3. Run CREATE DATABASE script
	// 4. Update database status to installed

	h.broadcastProgress(payload.DatabaseID, "installed", "Database created successfully")

	return nil
}

// UninstallDatabasePayload contains the data for uninstalling a database
type UninstallDatabasePayload struct {
	DatabaseID string  `json:"database_id"`
	UserID     *string `json:"user_id,omitempty"`
}

// NewUninstallDatabaseTask creates a new uninstall database task
func NewUninstallDatabaseTask(databaseID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallDatabasePayload{
		DatabaseID: databaseID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallDatabase, payload), nil
}

// HandleUninstallDatabase handles the database uninstallation job
func (h *Handler) HandleUninstallDatabase(ctx context.Context, t *asynq.Task) error {
	var payload UninstallDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("database_id", payload.DatabaseID).Msg("Uninstalling database")

	// TODO: Implement actual database deletion via SSH
	// 1. Get database and server info
	// 2. Connect via SSH
	// 3. Run DROP DATABASE script
	// 4. Delete database record from database

	h.broadcastProgress(payload.DatabaseID, "deleted", "Database deleted successfully")

	return nil
}

// InstallDatabaseUserPayload contains the data for installing a database user
type InstallDatabaseUserPayload struct {
	UserID   string  `json:"user_id"`
	Password string  `json:"password"`
	CallerID *string `json:"caller_id,omitempty"`
}

// NewInstallDatabaseUserTask creates a new install database user task
func NewInstallDatabaseUserTask(userID, password string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(InstallDatabaseUserPayload{
		UserID:   userID,
		Password: password,
		CallerID: callerID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeInstallDatabaseUser, payload), nil
}

// HandleInstallDatabaseUser handles the database user installation job
func (h *Handler) HandleInstallDatabaseUser(ctx context.Context, t *asynq.Task) error {
	var payload InstallDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("user_id", payload.UserID).Msg("Installing database user")

	// TODO: Implement actual database user creation via SSH
	// 1. Get user, databases, and server info
	// 2. Connect via SSH
	// 3. Run CREATE USER script with GRANT statements
	// 4. Update user status to installed

	h.broadcastUserProgress(payload.UserID, "installed", "Database user created successfully")

	return nil
}

// UpdateDatabaseUserPayload contains the data for updating a database user
type UpdateDatabaseUserPayload struct {
	UserID   string  `json:"user_id"`
	Password *string `json:"password,omitempty"`
	CallerID *string `json:"caller_id,omitempty"`
}

// NewUpdateDatabaseUserTask creates a new update database user task
func NewUpdateDatabaseUserTask(userID string, password *string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UpdateDatabaseUserPayload{
		UserID:   userID,
		Password: password,
		CallerID: callerID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUpdateDatabaseUser, payload), nil
}

// HandleUpdateDatabaseUser handles the database user update job
func (h *Handler) HandleUpdateDatabaseUser(ctx context.Context, t *asynq.Task) error {
	var payload UpdateDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("user_id", payload.UserID).Msg("Updating database user")

	// TODO: Implement actual database user update via SSH
	// 1. Get user, databases, and server info
	// 2. Connect via SSH
	// 3. Run ALTER USER and GRANT/REVOKE scripts
	// 4. Update user record

	h.broadcastUserProgress(payload.UserID, "updated", "Database user updated successfully")

	return nil
}

// UninstallDatabaseUserPayload contains the data for uninstalling a database user
type UninstallDatabaseUserPayload struct {
	UserID   string  `json:"user_id"`
	CallerID *string `json:"caller_id,omitempty"`
}

// NewUninstallDatabaseUserTask creates a new uninstall database user task
func NewUninstallDatabaseUserTask(userID string, callerID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(UninstallDatabaseUserPayload{
		UserID:   userID,
		CallerID: callerID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeUninstallDatabaseUser, payload), nil
}

// HandleUninstallDatabaseUser handles the database user uninstallation job
func (h *Handler) HandleUninstallDatabaseUser(ctx context.Context, t *asynq.Task) error {
	var payload UninstallDatabaseUserPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("user_id", payload.UserID).Msg("Uninstalling database user")

	// TODO: Implement actual database user deletion via SSH
	// 1. Get user and server info
	// 2. Connect via SSH
	// 3. Run DROP USER script
	// 4. Delete user record from database

	h.broadcastUserProgress(payload.UserID, "deleted", "Database user deleted successfully")

	return nil
}

// SyncDatabasesPayload contains the data for syncing databases from server
type SyncDatabasesPayload struct {
	ServerID string  `json:"server_id"`
	UserID   *string `json:"user_id,omitempty"`
}

// NewSyncDatabasesTask creates a new sync databases task
func NewSyncDatabasesTask(serverID string, userID *string) (*asynq.Task, error) {
	payload, err := json.Marshal(SyncDatabasesPayload{
		ServerID: serverID,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeSyncDatabases, payload), nil
}

// HandleSyncDatabases handles the database sync job
func (h *Handler) HandleSyncDatabases(ctx context.Context, t *asynq.Task) error {
	var payload SyncDatabasesPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().Str("server_id", payload.ServerID).Msg("Syncing databases from server")

	// TODO: Implement actual database sync via SSH
	// 1. Get server info
	// 2. Connect via SSH
	// 3. Run SHOW DATABASES script
	// 4. Compare with existing records
	// 5. Create missing database records

	return nil
}

// Helper methods

func (h *Handler) broadcastProgress(databaseID, status, message string) {
	h.ws.Broadcast("database", "database.status", map[string]interface{}{
		"database_id": databaseID,
		"status":      status,
		"message":     message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) broadcastUserProgress(userID, status, message string) {
	h.ws.Broadcast("database", "database_user.status", map[string]interface{}{
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetDatabaseScriptTemplate returns the script template for creating a database
func GetDatabaseScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "CREATE DATABASE IF NOT EXISTS \` + "`{{.Name}}`" + ` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
echo "Database {{.Name}} created successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "CREATE DATABASE \"{{.Name}}\";"
echo "Database {{.Name}} created successfully"
`
	default:
		return ""
	}
}

// GetDropDatabaseScriptTemplate returns the script template for dropping a database
func GetDropDatabaseScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "DROP DATABASE IF EXISTS \` + "`{{.Name}}`" + `;"
echo "Database {{.Name}} dropped successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "DROP DATABASE IF EXISTS \"{{.Name}}\";"
echo "Database {{.Name}} dropped successfully"
`
	default:
		return ""
	}
}

// GetCreateUserScriptTemplate returns the script template for creating a database user
func GetCreateUserScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "CREATE USER IF NOT EXISTS '{{.Name}}'@'{{.Host}}' IDENTIFIED BY '{{.Password}}';"
{{range .Databases}}
mysql -u root -e "GRANT ALL PRIVILEGES ON \` + "`{{.}}`" + `.* TO '{{$.Name}}'@'{{$.Host}}';"
{{end}}
mysql -u root -e "FLUSH PRIVILEGES;"
echo "User {{.Name}} created successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "CREATE USER \"{{.Name}}\" WITH PASSWORD '{{.Password}}';"
{{range .Databases}}
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE \"{{.}}\" TO \"{{$.Name}}\";"
{{end}}
echo "User {{.Name}} created successfully"
`
	default:
		return ""
	}
}

// GetDropUserScriptTemplate returns the script template for dropping a database user
func GetDropUserScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "DROP USER IF EXISTS '{{.Name}}'@'{{.Host}}';"
mysql -u root -e "FLUSH PRIVILEGES;"
echo "User {{.Name}} dropped successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "DROP USER IF EXISTS \"{{.Name}}\";"
echo "User {{.Name}} dropped successfully"
`
	default:
		return ""
	}
}
