# Scripts Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement reusable Bash scripts that can be executed across multiple servers with variable interpolation and real-time output streaming.

**Architecture:** New `script` module following existing patterns. Scripts can be personal (user-only) or team-shared. Execution creates records per server, linked by batch_id. Jobs handle SSH execution with WebSocket streaming, saving output on completion.

**Tech Stack:** Go, GORM, Fiber, Asynq, WebSocket, SSH

---

## Task 1: Database Migrations

**Files:**
- Create: `internal/database/migrations/0003_01_19_000001_add_script_columns.go`
- Create: `internal/database/migrations/0003_01_19_000002_add_script_execution_columns.go`

**Step 1: Create scripts table migration to add new columns**

```go
// internal/database/migrations/0003_01_19_000001_add_script_columns.go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_01_19_000001_add_script_columns",
		Name:      "Add columns to scripts table",
		Timestamp: time.Date(2003, 1, 19, 0, 0, 1, 0, time.UTC),
		Up:        addScriptColumnsUp,
		Down:      addScriptColumnsDown,
	})
}

func addScriptColumnsUp(db *gorm.DB) error {
	// Add team_id column (nullable for personal scripts)
	if err := db.Exec("ALTER TABLE scripts ADD COLUMN team_id CHAR(26) NULL AFTER user_id").Error; err != nil {
		return err
	}

	// Add user column (unix user to run as)
	if err := db.Exec("ALTER TABLE scripts ADD COLUMN user VARCHAR(255) NOT NULL DEFAULT 'root' AFTER name").Error; err != nil {
		return err
	}

	// Add indexes
	if err := db.Exec("CREATE INDEX idx_scripts_team_id ON scripts(team_id)").Error; err != nil {
		return err
	}

	// Add foreign key for team_id
	if err := db.Exec("ALTER TABLE scripts ADD CONSTRAINT fk_scripts_team FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE").Error; err != nil {
		return err
	}

	return nil
}

func addScriptColumnsDown(db *gorm.DB) error {
	db.Exec("ALTER TABLE scripts DROP FOREIGN KEY fk_scripts_team")
	db.Exec("DROP INDEX idx_scripts_team_id ON scripts")
	db.Exec("ALTER TABLE scripts DROP COLUMN user")
	db.Exec("ALTER TABLE scripts DROP COLUMN team_id")
	return nil
}
```

**Step 2: Create script_executions migration to add new columns**

```go
// internal/database/migrations/0003_01_19_000002_add_script_execution_columns.go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_01_19_000002_add_script_execution_columns",
		Name:      "Add columns to script_executions table",
		Timestamp: time.Date(2003, 1, 19, 0, 0, 2, 0, time.UTC),
		Up:        addScriptExecutionColumnsUp,
		Down:      addScriptExecutionColumnsDown,
	})
}

func addScriptExecutionColumnsUp(db *gorm.DB) error {
	// Add batch_id column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN batch_id CHAR(26) NULL AFTER server_id").Error; err != nil {
		return err
	}

	// Add status column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'pending' AFTER user").Error; err != nil {
		return err
	}

	// Add exit_code column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN exit_code INT NULL AFTER status").Error; err != nil {
		return err
	}

	// Add output column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN output LONGTEXT NULL AFTER exit_code").Error; err != nil {
		return err
	}

	// Add started_at column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN started_at TIMESTAMP NULL AFTER output").Error; err != nil {
		return err
	}

	// Add index for batch_id
	if err := db.Exec("CREATE INDEX idx_script_executions_batch_id ON script_executions(batch_id)").Error; err != nil {
		return err
	}

	// Add index for status
	if err := db.Exec("CREATE INDEX idx_script_executions_status ON script_executions(status)").Error; err != nil {
		return err
	}

	return nil
}

func addScriptExecutionColumnsDown(db *gorm.DB) error {
	db.Exec("DROP INDEX idx_script_executions_status ON script_executions")
	db.Exec("DROP INDEX idx_script_executions_batch_id ON script_executions")
	db.Exec("ALTER TABLE script_executions DROP COLUMN started_at")
	db.Exec("ALTER TABLE script_executions DROP COLUMN output")
	db.Exec("ALTER TABLE script_executions DROP COLUMN exit_code")
	db.Exec("ALTER TABLE script_executions DROP COLUMN status")
	db.Exec("ALTER TABLE script_executions DROP COLUMN batch_id")
	return nil
}
```

**Step 3: Run migrations**

Run: `go run cmd/migrate/main.go up`

**Step 4: Commit**

```bash
git add internal/database/migrations/
git commit -m "Add migrations for scripts feature columns"
```

---

## Task 2: Models

**Files:**
- Create: `internal/modules/script/models/script.go`
- Create: `internal/modules/script/models/script_execution.go`

**Step 1: Create Script model**

```go
// internal/modules/script/models/script.go
package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Script represents a reusable bash script
type Script struct {
	basemodels.BaseModel
	UserID  string  `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID  *string `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Name    string  `gorm:"type:varchar(255);not null" json:"name"`
	User    string  `gorm:"type:varchar(255);not null;default:root" json:"user"`
	Content string  `gorm:"type:longtext;not null" json:"content"`

	// Relations
	Executions []ScriptExecution `gorm:"foreignKey:ScriptID;references:ID" json:"executions,omitempty"`
}

func (Script) TableName() string {
	return "scripts"
}

// IsTeamShared returns true if the script is shared with a team
func (s *Script) IsTeamShared() bool {
	return s.TeamID != nil && *s.TeamID != ""
}
```

**Step 2: Create ScriptExecution model**

```go
// internal/modules/script/models/script_execution.go
package models

import (
	"time"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// ExecutionStatus represents the status of a script execution
type ExecutionStatus string

const (
	ExecutionStatusPending  ExecutionStatus = "pending"
	ExecutionStatusRunning  ExecutionStatus = "running"
	ExecutionStatusFinished ExecutionStatus = "finished"
	ExecutionStatusFailed   ExecutionStatus = "failed"
)

// ScriptExecution represents a single execution of a script on a server
type ScriptExecution struct {
	ID         uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID   string          `gorm:"column:script_id;type:char(26);not null;index" json:"script_id"`
	ServerID   string          `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	BatchID    *string         `gorm:"column:batch_id;type:char(26);index" json:"batch_id,omitempty"`
	User       *string         `gorm:"type:varchar(255)" json:"user,omitempty"`
	Status     ExecutionStatus `gorm:"type:varchar(50);not null;default:pending" json:"status"`
	ExitCode   *int            `gorm:"column:exit_code" json:"exit_code,omitempty"`
	Output     *string         `gorm:"type:longtext" json:"output,omitempty"`
	StartedAt  *time.Time      `gorm:"column:started_at;type:timestamp null" json:"started_at,omitempty"`
	FinishedAt *time.Time      `gorm:"column:finished_at;type:timestamp null" json:"finished_at,omitempty"`
	CreatedAt  *time.Time      `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt  *time.Time      `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Script *Script                  `gorm:"foreignKey:ScriptID;references:ID" json:"script,omitempty"`
	Server *servermodels.Server     `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (ScriptExecution) TableName() string {
	return "script_executions"
}

// IsComplete returns true if the execution has finished (success or failure)
func (e *ScriptExecution) IsComplete() bool {
	return e.Status == ExecutionStatusFinished || e.Status == ExecutionStatusFailed
}
```

**Step 3: Commit**

```bash
git add internal/modules/script/models/
git commit -m "Add Script and ScriptExecution models"
```

---

## Task 3: DTOs

**Files:**
- Create: `internal/modules/script/dto/requests.go`
- Create: `internal/modules/script/dto/responses.go`

**Step 1: Create request DTOs**

```go
// internal/modules/script/dto/requests.go
package dto

// CreateScriptRequest represents a request to create a script
type CreateScriptRequest struct {
	Name    string  `json:"name" validate:"required,max=255"`
	User    string  `json:"user" validate:"required,max=255"`
	Content string  `json:"content" validate:"required"`
	TeamID  *string `json:"team_id,omitempty"`
}

// UpdateScriptRequest represents a request to update a script
type UpdateScriptRequest struct {
	Name    *string `json:"name,omitempty" validate:"omitempty,max=255"`
	User    *string `json:"user,omitempty" validate:"omitempty,max=255"`
	Content *string `json:"content,omitempty"`
	TeamID  *string `json:"team_id,omitempty"`
}

// ExecuteScriptRequest represents a request to execute a script
type ExecuteScriptRequest struct {
	ServerIDs []string `json:"server_ids" validate:"required,min=1"`
	User      *string  `json:"user,omitempty"`
}
```

**Step 2: Create response DTOs**

```go
// internal/modules/script/dto/responses.go
package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/script/models"
)

// ScriptResponse represents a script in API responses
type ScriptResponse struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TeamID    *string    `json:"team_id,omitempty"`
	Name      string     `json:"name"`
	User      string     `json:"user"`
	Content   string     `json:"content"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ToScriptResponse converts a Script model to ScriptResponse
func ToScriptResponse(s *models.Script) *ScriptResponse {
	return &ScriptResponse{
		ID:        s.ID,
		UserID:    s.UserID,
		TeamID:    s.TeamID,
		Name:      s.Name,
		User:      s.User,
		Content:   s.Content,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// ScriptExecutionResponse represents an execution in API responses
type ScriptExecutionResponse struct {
	ID         uint64     `json:"id"`
	ScriptID   string     `json:"script_id"`
	ServerID   string     `json:"server_id"`
	BatchID    *string    `json:"batch_id,omitempty"`
	User       *string    `json:"user,omitempty"`
	Status     string     `json:"status"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Output     *string    `json:"output,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

// ToExecutionResponse converts a ScriptExecution to response
func ToExecutionResponse(e *models.ScriptExecution) *ScriptExecutionResponse {
	return &ScriptExecutionResponse{
		ID:         e.ID,
		ScriptID:   e.ScriptID,
		ServerID:   e.ServerID,
		BatchID:    e.BatchID,
		User:       e.User,
		Status:     string(e.Status),
		ExitCode:   e.ExitCode,
		Output:     e.Output,
		StartedAt:  e.StartedAt,
		FinishedAt: e.FinishedAt,
		CreatedAt:  e.CreatedAt,
	}
}

// ExecuteScriptResponse represents the response after triggering execution
type ExecuteScriptResponse struct {
	BatchID    string                     `json:"batch_id"`
	Executions []*ScriptExecutionResponse `json:"executions"`
}
```

**Step 3: Commit**

```bash
git add internal/modules/script/dto/
git commit -m "Add Script DTOs for requests and responses"
```

---

## Task 4: Repositories

**Files:**
- Create: `internal/modules/script/repositories/script_repository.go`
- Create: `internal/modules/script/repositories/script_execution_repository.go`
- Create: `internal/modules/script/repositories/registry.go`
- Create: `internal/modules/script/repositories/errors.go`

**Step 1: Create errors**

```go
// internal/modules/script/repositories/errors.go
package repositories

import "errors"

var (
	ErrScriptNotFound    = errors.New("script not found")
	ErrExecutionNotFound = errors.New("script execution not found")
)
```

**Step 2: Create ScriptRepository**

```go
// internal/modules/script/repositories/script_repository.go
package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ScriptRepository handles database operations for scripts
type ScriptRepository struct {
	repository.Base[models.Script]
}

// NewScriptRepository creates a new script repository
func NewScriptRepository(db *gorm.DB) *ScriptRepository {
	return &ScriptRepository{
		Base: repository.NewBase[models.Script](db),
	}
}

// FindByID finds a script by ID
func (r *ScriptRepository) FindByID(ctx context.Context, id string) (*models.Script, error) {
	script, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrScriptNotFound
		}
		return nil, err
	}
	return script, nil
}

// FindByUserOrTeam finds scripts accessible to a user (personal + team shared)
func (r *ScriptRepository) FindByUserOrTeam(ctx context.Context, userID, teamID string) ([]models.Script, error) {
	var scripts []models.Script
	err := r.DB.WithContext(ctx).
		Where("user_id = ? OR team_id = ?", userID, teamID).
		Order("name ASC").
		Find(&scripts).Error
	return scripts, err
}

// FindByTeam finds all team-shared scripts
func (r *ScriptRepository) FindByTeam(ctx context.Context, teamID string) ([]models.Script, error) {
	var scripts []models.Script
	err := r.DB.WithContext(ctx).
		Where("team_id = ?", teamID).
		Order("name ASC").
		Find(&scripts).Error
	return scripts, err
}

// CanAccess checks if a user can access a script
func (r *ScriptRepository) CanAccess(ctx context.Context, scriptID, userID, teamID string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Script{}).
		Where("id = ? AND (user_id = ? OR team_id = ?)", scriptID, userID, teamID).
		Count(&count).Error
	return count > 0, err
}
```

**Step 3: Create ScriptExecutionRepository**

```go
// internal/modules/script/repositories/script_execution_repository.go
package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/script/models"
)

// ScriptExecutionRepository handles database operations for script executions
type ScriptExecutionRepository struct {
	DB *gorm.DB
}

// NewScriptExecutionRepository creates a new execution repository
func NewScriptExecutionRepository(db *gorm.DB) *ScriptExecutionRepository {
	return &ScriptExecutionRepository{DB: db}
}

// Create creates a new execution record
func (r *ScriptExecutionRepository) Create(ctx context.Context, execution *models.ScriptExecution) error {
	return r.DB.WithContext(ctx).Create(execution).Error
}

// FindByID finds an execution by ID
func (r *ScriptExecutionRepository) FindByID(ctx context.Context, id uint64) (*models.ScriptExecution, error) {
	var execution models.ScriptExecution
	err := r.DB.WithContext(ctx).
		Preload("Server").
		First(&execution, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrExecutionNotFound
		}
		return nil, err
	}
	return &execution, nil
}

// FindByScript finds all executions for a script
func (r *ScriptExecutionRepository) FindByScript(ctx context.Context, scriptID string) ([]models.ScriptExecution, error) {
	var executions []models.ScriptExecution
	err := r.DB.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Preload("Server").
		Order("created_at DESC").
		Find(&executions).Error
	return executions, err
}

// FindByBatch finds all executions in a batch
func (r *ScriptExecutionRepository) FindByBatch(ctx context.Context, batchID string) ([]models.ScriptExecution, error) {
	var executions []models.ScriptExecution
	err := r.DB.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Preload("Server").
		Order("created_at ASC").
		Find(&executions).Error
	return executions, err
}

// UpdateFields updates specific fields of an execution
func (r *ScriptExecutionRepository) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	return r.DB.WithContext(ctx).
		Model(&models.ScriptExecution{}).
		Where("id = ?", id).
		Updates(fields).Error
}
```

**Step 4: Create Registry**

```go
// internal/modules/script/repositories/registry.go
package repositories

import "gorm.io/gorm"

// Registry holds all script module repositories
type Registry struct {
	script    *ScriptRepository
	execution *ScriptExecutionRepository
}

// NewRegistry creates a new repository registry
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		script:    NewScriptRepository(db),
		execution: NewScriptExecutionRepository(db),
	}
}

// Script returns the script repository
func (r *Registry) Script() *ScriptRepository {
	return r.script
}

// Execution returns the execution repository
func (r *Registry) Execution() *ScriptExecutionRepository {
	return r.execution
}
```

**Step 5: Commit**

```bash
git add internal/modules/script/repositories/
git commit -m "Add Script repositories"
```

---

## Task 5: Variable Interpolation

**Files:**
- Create: `internal/modules/script/support/interpolation.go`

**Step 1: Create interpolation helper**

```go
// internal/modules/script/support/interpolation.go
package support

import (
	"strings"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// InterpolateVariables replaces template variables in script content with server values
func InterpolateVariables(content string, server *servermodels.Server) string {
	replacements := map[string]string{
		"{{server_id}}":         server.ID,
		"{{server_name}}":       server.Name,
		"{{ip_address}}":        getStringOrEmpty(server.PublicIPv4),
		"{{private_ip_address}}": getStringOrEmpty(server.PrivateIPv4),
		"{{username}}":          getStringOrEmpty(server.Username),
		"{{db_password}}":       string(server.DatabasePassword),
		"{{server_type}}":       getStringOrEmpty(server.Type),
	}

	result := content
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

func getStringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

**Step 2: Commit**

```bash
git add internal/modules/script/support/
git commit -m "Add variable interpolation for scripts"
```

---

## Task 6: Service

**Files:**
- Create: `internal/modules/script/services/script_service.go`

**Step 1: Create ScriptService**

```go
// internal/modules/script/services/script_service.go
package services

import (
	"context"
	"fmt"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/jobs"
	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/queue"
)

// ScriptService handles business logic for scripts
type ScriptService struct {
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
	queue       *queue.Client
	logger      *zerolog.Logger
}

// NewScriptService creates a new script service
func NewScriptService(
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	queue *queue.Client,
	logger *zerolog.Logger,
) *ScriptService {
	return &ScriptService{
		repos:       repos,
		serverRepos: serverRepos,
		queue:       queue,
		logger:      logger,
	}
}

// List returns all scripts accessible to the user
func (s *ScriptService) List(ctx context.Context, userID, teamID string) ([]models.Script, error) {
	return s.repos.Script().FindByUserOrTeam(ctx, userID, teamID)
}

// Get returns a script by ID if user has access
func (s *ScriptService) Get(ctx context.Context, scriptID, userID, teamID string) (*models.Script, error) {
	script, err := s.repos.Script().FindByID(ctx, scriptID)
	if err != nil {
		return nil, err
	}

	// Check access
	if script.UserID != userID && (script.TeamID == nil || *script.TeamID != teamID) {
		return nil, repositories.ErrScriptNotFound
	}

	return script, nil
}

// Create creates a new script
func (s *ScriptService) Create(ctx context.Context, userID string, req *dto.CreateScriptRequest) (*models.Script, error) {
	script := &models.Script{
		UserID:  userID,
		TeamID:  req.TeamID,
		Name:    req.Name,
		User:    req.User,
		Content: req.Content,
	}

	if err := s.repos.Script().Create(ctx, script); err != nil {
		return nil, err
	}

	return script, nil
}

// Update updates a script
func (s *ScriptService) Update(ctx context.Context, scriptID, userID, teamID string, req *dto.UpdateScriptRequest) (*models.Script, error) {
	script, err := s.Get(ctx, scriptID, userID, teamID)
	if err != nil {
		return nil, err
	}

	// Only owner can update
	if script.UserID != userID {
		return nil, fmt.Errorf("only the script owner can update it")
	}

	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.User != nil {
		updates["user"] = *req.User
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.TeamID != nil {
		updates["team_id"] = req.TeamID
	}

	if len(updates) > 0 {
		if err := s.repos.Script().UpdateFields(ctx, scriptID, updates); err != nil {
			return nil, err
		}
	}

	return s.repos.Script().FindByID(ctx, scriptID)
}

// Delete deletes a script
func (s *ScriptService) Delete(ctx context.Context, scriptID, userID, teamID string) error {
	script, err := s.Get(ctx, scriptID, userID, teamID)
	if err != nil {
		return err
	}

	// Only owner can delete
	if script.UserID != userID {
		return fmt.Errorf("only the script owner can delete it")
	}

	return s.repos.Script().Delete(ctx, scriptID)
}

// Execute executes a script on multiple servers
func (s *ScriptService) Execute(ctx context.Context, scriptID, userID, teamID string, req *dto.ExecuteScriptRequest) (*dto.ExecuteScriptResponse, error) {
	script, err := s.Get(ctx, scriptID, userID, teamID)
	if err != nil {
		return nil, err
	}

	// Validate servers exist and user has access
	for _, serverID := range req.ServerIDs {
		server, err := s.serverRepos.Server().FindByID(ctx, serverID)
		if err != nil {
			return nil, fmt.Errorf("server %s not found", serverID)
		}
		if server.TeamID != teamID {
			return nil, fmt.Errorf("server %s not accessible", serverID)
		}
	}

	// Generate batch ID
	batchID := ulid.Make().String()

	// Determine user to run as
	runAsUser := script.User
	if req.User != nil && *req.User != "" {
		runAsUser = *req.User
	}

	// Create execution records and dispatch jobs
	var executions []*dto.ScriptExecutionResponse
	for _, serverID := range req.ServerIDs {
		execution := &models.ScriptExecution{
			ScriptID: scriptID,
			ServerID: serverID,
			BatchID:  &batchID,
			User:     &runAsUser,
			Status:   models.ExecutionStatusPending,
		}

		if err := s.repos.Execution().Create(ctx, execution); err != nil {
			return nil, fmt.Errorf("failed to create execution record: %w", err)
		}

		// Dispatch job
		task, err := jobs.NewExecuteScriptTask(execution.ID, scriptID, serverID, teamID)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to create execute script task")
			continue
		}

		if s.queue != nil {
			if _, err := s.queue.Enqueue(task); err != nil {
				s.logger.Error().Err(err).Msg("Failed to enqueue execute script job")
			}
		}

		executions = append(executions, dto.ToExecutionResponse(execution))
	}

	return &dto.ExecuteScriptResponse{
		BatchID:    batchID,
		Executions: executions,
	}, nil
}

// GetExecution returns a single execution
func (s *ScriptService) GetExecution(ctx context.Context, executionID uint64) (*models.ScriptExecution, error) {
	return s.repos.Execution().FindByID(ctx, executionID)
}

// ListExecutions returns all executions for a script
func (s *ScriptService) ListExecutions(ctx context.Context, scriptID, userID, teamID string) ([]models.ScriptExecution, error) {
	// Verify access to script
	if _, err := s.Get(ctx, scriptID, userID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Execution().FindByScript(ctx, scriptID)
}
```

**Step 2: Commit**

```bash
git add internal/modules/script/services/
git commit -m "Add ScriptService with CRUD and execution"
```

---

## Task 7: Job and Task

**Files:**
- Create: `internal/modules/script/jobs/execute_script.go`
- Create: `internal/modules/script/jobs/context.go`
- Create: `internal/modules/script/jobs/register.go`
- Create: `internal/modules/script/tasks/run_script.go`

**Step 1: Create job context**

```go
// internal/modules/script/jobs/context.go
package jobs

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// JobContext holds dependencies for script job execution
type JobContext struct {
	DB             *gorm.DB
	Logger         *zerolog.Logger
	WS             broadcast.TeamBroadcaster
	Dispatcher     taskrunner.TaskDispatcher
	Queue          *queue.Client
	Repos          *repositories.Registry
	ServerRepos    *serverrepos.Registry
	TaskRunnerDeps *servertasks.TaskRunnerDeps
}

// NewJobContext creates a new script job context
func NewJobContext(
	db *gorm.DB,
	logger *zerolog.Logger,
	ws broadcast.TeamBroadcaster,
	dispatcher taskrunner.TaskDispatcher,
	queueClient *queue.Client,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
) *JobContext {
	return &JobContext{
		DB:          db,
		Logger:      logger,
		WS:          ws,
		Dispatcher:  dispatcher,
		Queue:       queueClient,
		Repos:       repos,
		ServerRepos: serverRepos,
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:          db,
			Queue:       queueClient,
			Dispatcher:  dispatcher,
			Logger:      logger,
			Broadcaster: ws,
		},
	}
}

// RunTaskOnServer creates a TaskRunner for executing a task on a server
func (c *JobContext) RunTaskOnServer(server *servermodels.Server, task taskrunner.Task) *servertasks.TaskRunner {
	return c.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastToTeam sends a websocket event to a team channel
func (c *JobContext) BroadcastToTeam(teamID, event string, data any) {
	if c.WS != nil {
		c.WS.BroadcastToTeam(teamID, event, data)
	}
}

// LogInfo logs an info message
func (c *JobContext) LogInfo(msg string, fields ...any) {
	if c.Logger == nil {
		return
	}
	event := c.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error message
func (c *JobContext) LogError(err error, msg string, fields ...any) {
	if c.Logger == nil {
		return
	}
	event := c.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
```

**Step 2: Create run script task**

```go
// internal/modules/script/tasks/run_script.go
package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunScriptConfig holds configuration for running a script
type RunScriptConfig struct {
	Content string // The interpolated script content
}

// RunScript creates a task to run a script on a server
func RunScript(cfg RunScriptConfig) taskrunner.Task {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Run Script"),
		taskrunner.WithScript(cfg.Content),
		taskrunner.WithTimeoutSeconds(3600), // 1 hour max
	)
}
```

**Step 3: Create execute script job**

```go
// internal/modules/script/jobs/execute_script.go
package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/support"
	"github.com/kkz6/launch-go/internal/modules/script/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeExecuteScript = "script:execute"

// ExecuteScriptPayload holds data for executing a script
type ExecuteScriptPayload struct {
	ExecutionID uint64 `json:"execution_id"`
	ScriptID    string `json:"script_id"`
	ServerID    string `json:"server_id"`
	TeamID      string `json:"team_id"`
}

// ExecuteScriptJob handles script execution on a server
type ExecuteScriptJob struct {
	ctx     *JobContext
	Payload ExecuteScriptPayload
}

// NewExecuteScriptJob creates a new ExecuteScriptJob
func NewExecuteScriptJob(ctx *JobContext, payload ExecuteScriptPayload) *ExecuteScriptJob {
	return &ExecuteScriptJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the script on the server
func (j *ExecuteScriptJob) Handle(ctx context.Context) error {
	j.ctx.LogInfo("ExecuteScript job started",
		"execution_id", j.Payload.ExecutionID,
		"script_id", j.Payload.ScriptID,
		"server_id", j.Payload.ServerID,
	)

	// Get execution record
	execution, err := j.ctx.Repos.Execution().FindByID(ctx, j.Payload.ExecutionID)
	if err != nil {
		return fmt.Errorf("failed to find execution: %w", err)
	}

	// Get script
	script, err := j.ctx.Repos.Script().FindByID(ctx, j.Payload.ScriptID)
	if err != nil {
		return fmt.Errorf("failed to find script: %w", err)
	}

	// Get server
	server, err := j.ctx.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Update status to running
	now := time.Now()
	if err := j.ctx.Repos.Execution().UpdateFields(ctx, execution.ID, map[string]any{
		"status":     models.ExecutionStatusRunning,
		"started_at": now,
	}); err != nil {
		j.ctx.LogError(err, "Failed to update execution status")
	}

	// Broadcast started event
	j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.execution.started", map[string]any{
		"execution_id": execution.ID,
		"batch_id":     execution.BatchID,
		"server_id":    server.ID,
		"script_id":    script.ID,
	})

	// Interpolate variables
	interpolatedContent := support.InterpolateVariables(script.Content, server)

	// Create the task
	task := tasks.RunScript(tasks.RunScriptConfig{
		Content: interpolatedContent,
	})

	// Set up output streaming
	var outputBuffer strings.Builder
	task.(*taskrunner.BaseTask).OutputCallback = func(output string) {
		outputBuffer.Reset()
		outputBuffer.WriteString(output)

		// Broadcast output chunk
		j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.output", map[string]any{
			"execution_id": execution.ID,
			"batch_id":     execution.BatchID,
			"server_id":    server.ID,
			"output":       output,
		})
	}

	// Determine user to run as
	runAsUser := "root"
	if execution.User != nil && *execution.User != "" {
		runAsUser = *execution.User
	}

	// Execute the task
	result, err := j.ctx.RunTaskOnServer(server, task).
		AsUser(runAsUser).
		Dispatch(ctx)

	// Prepare final status
	finalStatus := models.ExecutionStatusFinished
	var exitCode *int
	var output *string

	if result != nil {
		ec := result.GetExitCode()
		exitCode = &ec
		out := result.GetOutput()
		output = &out

		if !result.IsSuccessful() {
			finalStatus = models.ExecutionStatusFailed
		}
	} else if err != nil {
		finalStatus = models.ExecutionStatusFailed
		errMsg := err.Error()
		output = &errMsg
	}

	// Update execution record
	finishedAt := time.Now()
	if updateErr := j.ctx.Repos.Execution().UpdateFields(ctx, execution.ID, map[string]any{
		"status":      finalStatus,
		"exit_code":   exitCode,
		"output":      output,
		"finished_at": finishedAt,
	}); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update execution result")
	}

	// Broadcast completion
	j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.execution.completed", map[string]any{
		"execution_id": execution.ID,
		"batch_id":     execution.BatchID,
		"server_id":    server.ID,
		"script_id":    script.ID,
		"status":       string(finalStatus),
		"exit_code":    exitCode,
	})

	j.ctx.LogInfo("ExecuteScript job completed",
		"execution_id", execution.ID,
		"status", finalStatus,
	)

	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

// Failed handles job failure
func (j *ExecuteScriptJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "ExecuteScript job failed",
		"execution_id", j.Payload.ExecutionID,
		"script_id", j.Payload.ScriptID,
		"server_id", j.Payload.ServerID,
	)

	errMsg := err.Error()

	// Update execution status to failed
	if updateErr := j.ctx.Repos.Execution().UpdateFields(ctx, j.Payload.ExecutionID, map[string]any{
		"status":      models.ExecutionStatusFailed,
		"output":      errMsg,
		"finished_at": time.Now(),
	}); updateErr != nil {
		j.ctx.LogError(updateErr, "Failed to update execution status on failure")
	}

	// Broadcast failure
	j.ctx.BroadcastToTeam(j.Payload.TeamID, "script.execution.completed", map[string]any{
		"execution_id": j.Payload.ExecutionID,
		"server_id":    j.Payload.ServerID,
		"script_id":    j.Payload.ScriptID,
		"status":       string(models.ExecutionStatusFailed),
		"error":        errMsg,
	})
}

// NewExecuteScriptTask creates an execute script job
func NewExecuteScriptTask(executionID uint64, scriptID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeExecuteScript, ExecuteScriptPayload{
		ExecutionID: executionID,
		ScriptID:    scriptID,
		ServerID:    serverID,
		TeamID:      teamID,
	})
}
```

**Step 4: Create job registration**

```go
// internal/modules/script/jobs/register.go
package jobs

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// RegisterHandlers registers all script job handlers
func RegisterHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterHandler(mux, TypeExecuteScript, jobContext, NewExecuteScriptJob)
}
```

**Step 5: Commit**

```bash
git add internal/modules/script/jobs/ internal/modules/script/tasks/
git commit -m "Add script execution job with streaming output"
```

---

## Task 8: Handler

**Files:**
- Create: `internal/modules/script/handlers/script_handler.go`

**Step 1: Create ScriptHandler**

```go
// internal/modules/script/handlers/script_handler.go
package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// ScriptHandler handles HTTP requests for scripts
type ScriptHandler struct {
	service *services.ScriptService
}

// NewScriptHandler creates a new script handler
func NewScriptHandler(service *services.ScriptService) *ScriptHandler {
	return &ScriptHandler{service: service}
}

// List returns all scripts accessible to the user
func (h *ScriptHandler) List(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)

	scripts, err := h.service.List(c.Context(), userID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	results := make([]*dto.ScriptResponse, len(scripts))
	for i, s := range scripts {
		results[i] = dto.ToScriptResponse(&s)
	}

	return response.Success(c, results)
}

// Show returns a single script
func (h *ScriptHandler) Show(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	script, err := h.service.Get(c.Context(), scriptID, userID, teamID)
	if err != nil {
		if err == repositories.ErrScriptNotFound {
			return response.Error(c, fiber.StatusNotFound, "Script not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.Success(c, dto.ToScriptResponse(script))
}

// Create creates a new script
func (h *ScriptHandler) Create(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req dto.CreateScriptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := validator.Validate(&req); err != nil {
		return response.ValidationError(c, err)
	}

	script, err := h.service.Create(c.Context(), userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.Success(c, dto.ToScriptResponse(script), fiber.StatusCreated)
}

// Update updates a script
func (h *ScriptHandler) Update(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	var req dto.UpdateScriptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := validator.Validate(&req); err != nil {
		return response.ValidationError(c, err)
	}

	script, err := h.service.Update(c.Context(), scriptID, userID, teamID, &req)
	if err != nil {
		if err == repositories.ErrScriptNotFound {
			return response.Error(c, fiber.StatusNotFound, "Script not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.Success(c, dto.ToScriptResponse(script))
}

// Delete deletes a script
func (h *ScriptHandler) Delete(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	if err := h.service.Delete(c.Context(), scriptID, userID, teamID); err != nil {
		if err == repositories.ErrScriptNotFound {
			return response.Error(c, fiber.StatusNotFound, "Script not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.Success(c, nil, fiber.StatusNoContent)
}

// Execute executes a script on servers
func (h *ScriptHandler) Execute(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	var req dto.ExecuteScriptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := validator.Validate(&req); err != nil {
		return response.ValidationError(c, err)
	}

	result, err := h.service.Execute(c.Context(), scriptID, userID, teamID, &req)
	if err != nil {
		if err == repositories.ErrScriptNotFound {
			return response.Error(c, fiber.StatusNotFound, "Script not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.Success(c, result)
}

// ListExecutions returns executions for a script
func (h *ScriptHandler) ListExecutions(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Locals("teamID").(string)
	scriptID := c.Params("id")

	executions, err := h.service.ListExecutions(c.Context(), scriptID, userID, teamID)
	if err != nil {
		if err == repositories.ErrScriptNotFound {
			return response.Error(c, fiber.StatusNotFound, "Script not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	results := make([]*dto.ScriptExecutionResponse, len(executions))
	for i, e := range executions {
		results[i] = dto.ToExecutionResponse(&e)
	}

	return response.Success(c, results)
}

// GetExecution returns a single execution
func (h *ScriptHandler) GetExecution(c *fiber.Ctx) error {
	executionIDStr := c.Params("id")
	executionID, err := strconv.ParseUint(executionIDStr, 10, 64)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid execution ID")
	}

	execution, err := h.service.GetExecution(c.Context(), executionID)
	if err != nil {
		if err == repositories.ErrExecutionNotFound {
			return response.Error(c, fiber.StatusNotFound, "Execution not found")
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error())
	}

	return response.Success(c, dto.ToExecutionResponse(execution))
}
```

**Step 2: Commit**

```bash
git add internal/modules/script/handlers/
git commit -m "Add ScriptHandler for HTTP endpoints"
```

---

## Task 9: Module and Routes

**Files:**
- Create: `internal/modules/script/module.go`
- Create: `internal/modules/script/routes.go`

**Step 1: Create module**

```go
// internal/modules/script/module.go
package script

import (
	"github.com/hibiken/asynq"

	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/jobs"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	"github.com/kkz6/launch-go/internal/modules/script/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
)

const ModuleName = "script"

// Ensure Module implements required interfaces
var (
	_ app.Module       = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module represents the script module
type Module struct {
	module.Base
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
}

// NewModule creates a new script module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()

	return &Module{
		Base:        module.NewBase(ModuleName, b),
		repos:       repositories.NewRegistry(deps.DB),
		serverRepos: serverrepos.NewRegistry(deps.DB),
	}
}

// RegisterJobs registers background job handlers
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	deps := m.Deps()

	jobContext := jobs.NewJobContext(
		deps.DB,
		deps.Logger,
		deps.WebSocket,
		deps.Dispatcher,
		deps.Queue,
		m.repos,
		m.serverRepos,
	)
	jobs.SetJobContext(jobContext)

	jobs.RegisterHandlers(mux)
}

// createService creates the script service
func (m *Module) createService() *services.ScriptService {
	deps := m.Deps()

	return services.NewScriptService(
		m.repos,
		m.serverRepos,
		deps.Queue,
		deps.Logger,
	)
}
```

**Step 2: Create routes**

```go
// internal/modules/script/routes.go
package script

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/script/handlers"
)

// RegisterRoutes registers all script routes
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	service := m.createService()
	handler := handlers.NewScriptHandler(service)

	// Script routes (authenticated + team scoped)
	scripts := router.Group("/scripts", authMiddleware, middleware.TeamScope())
	{
		scripts.Get("/", handler.List)
		scripts.Post("/", handler.Create)
		scripts.Get("/:id", handler.Show)
		scripts.Put("/:id", handler.Update)
		scripts.Delete("/:id", handler.Delete)

		// Execution
		scripts.Post("/:id/execute", handler.Execute)
		scripts.Get("/:id/executions", handler.ListExecutions)
	}

	// Execution routes (for fetching individual executions)
	executions := router.Group("/script-executions", authMiddleware, middleware.TeamScope())
	{
		executions.Get("/:id", handler.GetExecution)
	}
}
```

**Step 3: Commit**

```bash
git add internal/modules/script/module.go internal/modules/script/routes.go
git commit -m "Add Script module and routes"
```

---

## Task 10: Wire Module to API and Worker

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `cmd/worker/main.go`

**Step 1: Update cmd/api/main.go**

Add import:
```go
"github.com/kkz6/launch-go/internal/modules/script"
```

In `registerModules()`, after other module creation:
```go
scriptModule := script.NewModule(builder)
```

In the kernel registration chain, add:
```go
Register(scriptModule).
```

**Step 2: Update cmd/worker/main.go**

Add import:
```go
"github.com/kkz6/launch-go/internal/modules/script"
```

After other module creation:
```go
scriptModule := script.NewModule(builder)
```

In the kernel registration chain, add:
```go
Register(scriptModule).
```

**Step 3: Verify build**

Run: `go build ./...`

**Step 4: Commit**

```bash
git add cmd/api/main.go cmd/worker/main.go
git commit -m "Wire script module to API and worker"
```

---

## Task 11: Test the Feature

**Step 1: Run migrations**

```bash
go run cmd/migrate/main.go up
```

**Step 2: Start the API and worker**

```bash
# Terminal 1
go run cmd/api/main.go

# Terminal 2
go run cmd/worker/main.go
```

**Step 3: Test API endpoints**

```bash
# Create a script
curl -X POST http://localhost:3000/api/scripts \
  -H "Authorization: Bearer <token>" \
  -H "X-Team-ID: <team_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Script",
    "user": "root",
    "content": "echo \"Server: {{server_name}}\"\necho \"IP: {{ip_address}}\"\nuptime"
  }'

# List scripts
curl http://localhost:3000/api/scripts \
  -H "Authorization: Bearer <token>" \
  -H "X-Team-ID: <team_id>"

# Execute script
curl -X POST http://localhost:3000/api/scripts/<script_id>/execute \
  -H "Authorization: Bearer <token>" \
  -H "X-Team-ID: <team_id>" \
  -H "Content-Type: application/json" \
  -d '{"server_ids": ["<server_id>"]}'

# Get execution status
curl http://localhost:3000/api/script-executions/<execution_id> \
  -H "Authorization: Bearer <token>" \
  -H "X-Team-ID: <team_id>"
```

**Step 4: Final commit**

```bash
git add -A
git commit -m "Complete scripts feature implementation"
```

---

## Summary

This plan implements:

1. **Database migrations** - Add columns to scripts and script_executions tables
2. **Models** - Script and ScriptExecution with proper relationships
3. **DTOs** - Request/response structures for API
4. **Repositories** - Data access layer with access control
5. **Variable interpolation** - Replace {{placeholders}} with server data
6. **Service** - Business logic for CRUD and execution
7. **Job** - Async execution with WebSocket streaming
8. **Handler** - HTTP endpoints
9. **Module** - Wire everything together
10. **Integration** - Connect to API and worker
