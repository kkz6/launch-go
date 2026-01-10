package taskrunner

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "pending"
	TaskStatusRunning      TaskStatus = "running"
	TaskStatusFinished     TaskStatus = "finished"
	TaskStatusFailed       TaskStatus = "failed"
	TaskStatusTimeout      TaskStatus = "timeout"
	TaskStatusUploadFailed TaskStatus = "upload_failed"
)

// TaskModel represents a task stored in database
type TaskModel struct {
	ID         string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID   string         `gorm:"size:26;not null;index" json:"server_id"`
	UserID     *string        `gorm:"size:26;index" json:"user_id,omitempty"`
	Name       string         `gorm:"size:255;not null" json:"name"`
	User       string         `gorm:"size:100;default:'root'" json:"user"` // SSH user
	Type       string         `gorm:"size:500" json:"type"`                // Task class/type name
	Script     string         `gorm:"type:text" json:"-"`                  // Bash script (encrypted in Laravel)
	Timeout    int            `gorm:"default:600" json:"timeout"`          // Timeout in seconds
	Status     TaskStatus     `gorm:"size:50;default:'pending'" json:"status"`
	Output     string         `gorm:"type:text" json:"output,omitempty"` // Task output (encrypted in Laravel)
	ExitCode   *int           `json:"exit_code,omitempty"`
	PID        *string        `gorm:"size:20" json:"pid,omitempty"` // Process ID for background tasks
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Callback data - serialized task instance for callbacks
	CallbackData string `gorm:"type:text" json:"-"`
}

func (t *TaskModel) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}
	return nil
}

func (t *TaskModel) TableName() string {
	return "tasks"
}

// OutputLogPath returns the remote path where output is logged
func (t *TaskModel) OutputLogPath(scriptPath string) string {
	return scriptPath + "/task-" + t.ID + ".log"
}

// ScriptPath returns the remote path for the script
func (t *TaskModel) ScriptPath(basePath string) string {
	return basePath + "/task-" + t.ID + ".sh"
}

// IsFinished returns true if task has completed (success or failure)
func (t *TaskModel) IsFinished() bool {
	return t.Status == TaskStatusFinished ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusTimeout
}

// IsRunning returns true if task is currently executing
func (t *TaskModel) IsRunning() bool {
	return t.Status == TaskStatusRunning
}

// Duration returns the task duration if finished
func (t *TaskModel) Duration() *time.Duration {
	if t.StartedAt == nil || t.FinishedAt == nil {
		return nil
	}
	d := t.FinishedAt.Sub(*t.StartedAt)
	return &d
}

// TaskRepository handles task persistence
type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(task *TaskModel) error {
	return r.db.Create(task).Error
}

func (r *TaskRepository) FindByID(id string) (*TaskModel, error) {
	var task TaskModel
	err := r.db.First(&task, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *TaskRepository) FindByServerID(serverID string) ([]TaskModel, error) {
	var tasks []TaskModel
	err := r.db.Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

func (r *TaskRepository) Update(task *TaskModel) error {
	return r.db.Save(task).Error
}

func (r *TaskRepository) UpdateStatus(id string, status TaskStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if status == TaskStatusRunning {
		now := time.Now()
		updates["started_at"] = now
	}
	if status == TaskStatusFinished || status == TaskStatusFailed || status == TaskStatusTimeout {
		now := time.Now()
		updates["finished_at"] = now
	}
	return r.db.Model(&TaskModel{}).Where("id = ?", id).Updates(updates).Error
}

func (r *TaskRepository) UpdateOutput(id string, output string) error {
	return r.db.Model(&TaskModel{}).
		Where("id = ?", id).
		Update("output", output).Error
}

func (r *TaskRepository) UpdateResult(id string, status TaskStatus, exitCode int, output string) error {
	now := time.Now()
	return r.db.Model(&TaskModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"exit_code":   exitCode,
			"output":      output,
			"finished_at": now,
		}).Error
}

func (r *TaskRepository) FindPendingByServer(serverID string) ([]TaskModel, error) {
	var tasks []TaskModel
	err := r.db.Where("server_id = ? AND status IN ?", serverID,
		[]TaskStatus{TaskStatusPending, TaskStatusRunning}).
		Find(&tasks).Error
	return tasks, err
}
