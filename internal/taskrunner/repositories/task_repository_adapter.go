package repositories

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
	"github.com/kkz6/launch-go/internal/taskrunner/services"
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

// Task represents a task stored in database
type Task struct {
	ID         string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID   string         `gorm:"size:26;not null;index" json:"server_id"`
	UserID     *string        `gorm:"size:26;index" json:"user_id,omitempty"`
	Name       string         `gorm:"size:255;not null" json:"name"`
	User       string         `gorm:"size:100;default:'root'" json:"user"`
	Type       string         `gorm:"size:500" json:"type"`
	Script     string         `gorm:"type:text" json:"-"`
	Timeout    int            `gorm:"default:600" json:"timeout"`
	Status     TaskStatus     `gorm:"size:50;default:'pending'" json:"status"`
	Output     string         `gorm:"type:text" json:"output,omitempty"`
	ExitCode   *int           `json:"exit_code,omitempty"`
	PID        *string        `gorm:"size:20" json:"pid,omitempty"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	CallbackData string `gorm:"type:text" json:"-"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}
	return nil
}

func (t *Task) TableName() string {
	return "tasks"
}

// IsFinished returns true if task has completed
func (t *Task) IsFinished() bool {
	return t.Status == TaskStatusFinished ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusTimeout
}

// IsRunning returns true if task is currently executing
func (t *Task) IsRunning() bool {
	return t.Status == TaskStatusRunning
}

// TaskRepository handles task persistence
type TaskRepository struct {
	db *gorm.DB
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create creates a new task
func (r *TaskRepository) Create(task *Task) error {
	return r.db.Create(task).Error
}

// FindByID finds a task by ID
func (r *TaskRepository) FindByID(id string) (services.TaskModel, error) {
	var task Task
	err := r.db.First(&task, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// Update updates a task
func (r *TaskRepository) Update(task *Task) error {
	return r.db.Save(task).Error
}

// UpdateStatus updates task status with appropriate timestamps
func (r *TaskRepository) UpdateStatus(id string, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == string(TaskStatusRunning) {
		now := time.Now()
		updates["started_at"] = now
	}

	if status == string(TaskStatusFinished) || status == string(TaskStatusFailed) || status == string(TaskStatusTimeout) {
		now := time.Now()
		updates["finished_at"] = now
	}

	return r.db.Model(&Task{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateOutput updates task output
func (r *TaskRepository) UpdateOutput(id string, output string) error {
	return r.db.Model(&Task{}).
		Where("id = ?", id).
		Update("output", output).Error
}

// UpdateResult updates task result with status, exit code, and output
func (r *TaskRepository) UpdateResult(id string, status string, exitCode int, output string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":      status,
		"exit_code":   exitCode,
		"finished_at": now,
	}

	if output != "" {
		updates["output"] = output
	}

	return r.db.Model(&Task{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// FindByServerID finds all tasks for a server
func (r *TaskRepository) FindByServerID(serverID string) ([]Task, error) {
	var tasks []Task
	err := r.db.Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

// FindPendingByServer finds pending or running tasks for a server
func (r *TaskRepository) FindPendingByServer(serverID string) ([]Task, error) {
	var tasks []Task
	err := r.db.Where("server_id = ? AND status IN ?", serverID,
		[]TaskStatus{TaskStatusPending, TaskStatusRunning}).
		Find(&tasks).Error
	return tasks, err
}

// Ensure TaskRepository implements services.TaskRepository
var _ services.TaskRepository = (*TaskRepository)(nil)
