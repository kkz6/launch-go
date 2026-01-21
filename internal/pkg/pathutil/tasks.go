package pathutil

import (
	"fmt"
	"path/filepath"
)

// TaskPaths holds all file paths for a task execution
type TaskPaths struct {
	Script   string // task-{id}.sh
	Output   string // task-{id}.log
	ExitCode string // task-{id}.exit
}

// GetTaskPaths returns all file paths for a task given the base directory and task ID
func GetTaskPaths(baseDir, taskID string) TaskPaths {
	return TaskPaths{
		Script:   filepath.Join(baseDir, fmt.Sprintf("task-%s.sh", taskID)),
		Output:   filepath.Join(baseDir, fmt.Sprintf("task-%s.log", taskID)),
		ExitCode: filepath.Join(baseDir, fmt.Sprintf("task-%s.exit", taskID)),
	}
}

// GetTaskDir returns the .launch-tasks directory for a user
func GetTaskDir(user string) string {
	return JoinHome(user, ".launch-tasks")
}
