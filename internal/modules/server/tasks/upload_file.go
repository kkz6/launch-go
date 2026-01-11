package tasks

import (
	"path/filepath"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// UploadFile uploads file contents to a server
type UploadFile struct {
	BaseServerTask
	path     string
	contents string
	isRoot   bool
}

// NewUploadFile creates a new UploadFile task
func NewUploadFile(server *models.Server, path string, contents string, isRoot bool) *UploadFile {
	task := &UploadFile{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/upload-file",
				TaskTimeout:  20 * time.Second,
			},
			server: server,
		},
		path:     path,
		contents: contents,
		isRoot:   isRoot,
	}

	return task
}

// Data returns the template data
func (t *UploadFile) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":    t.server,
		"Path":      t.path,
		"Contents":  t.contents,
		"IsRoot":    t.isRoot,
		"Directory": t.Directory(),
	}
}

// Path returns the file path to upload to
func (t *UploadFile) Path() string {
	return t.path
}

// Contents returns the file contents to upload
func (t *UploadFile) Contents() string {
	return t.contents
}

// IsRoot returns whether to run as root
func (t *UploadFile) IsRoot() bool {
	return t.isRoot
}

// Directory returns the parent directory of the file path
func (t *UploadFile) Directory() string {
	return filepath.Dir(t.path)
}
