package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DeleteFile deletes a file from a server
type DeleteFile struct {
	BaseServerTask
	path   string
	isRoot bool
}

// NewDeleteFile creates a new DeleteFile task
func NewDeleteFile(server *models.Server, path string, isRoot bool) *DeleteFile {
	task := &DeleteFile{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/delete-file",
				TaskTimeout:  30 * time.Second,
			},
			server: server,
		},
		path:   path,
		isRoot: isRoot,
	}

	return task
}

// Data returns the template data
func (t *DeleteFile) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server": t.server,
		"Path":   t.path,
		"IsRoot": t.isRoot,
	}
}

// Path returns the file path to delete
func (t *DeleteFile) Path() string {
	return t.path
}

// IsRoot returns whether to run as root
func (t *DeleteFile) IsRoot() bool {
	return t.isRoot
}
