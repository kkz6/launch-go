package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// GetFile retrieves file contents from a server
type GetFile struct {
	BaseServerTask
	path   string
	lines  *int
	isRoot bool
}

// NewGetFile creates a new GetFile task
func NewGetFile(server *models.Server, path string, lines *int, isRoot bool) *GetFile {
	task := &GetFile{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/files/get-file",
				TaskTimeout:  30 * time.Second,
			},
			server: server,
		},
		path:   path,
		lines:  lines,
		isRoot: isRoot,
	}

	return task
}

// Data returns the template data
func (t *GetFile) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":  t.server,
		"Path":    t.path,
		"Lines":   t.lines,
		"IsRoot":  t.isRoot,
		"Command": t.buildCommand(),
	}
}

// Path returns the file path to retrieve
func (t *GetFile) Path() string {
	return t.path
}

// Lines returns the number of lines to tail (nil means full file up to 1MB)
func (t *GetFile) Lines() *int {
	return t.lines
}

// IsRoot returns whether to run as root
func (t *GetFile) IsRoot() bool {
	return t.isRoot
}

// buildCommand constructs the shell command for retrieving the file
func (t *GetFile) buildCommand() string {
	prefix := ""
	if t.isRoot {
		prefix = "sudo "
	}

	if t.lines != nil && *t.lines != 0 {
		return fmt.Sprintf("%stail -n %d %s", prefix, *t.lines, t.path)
	}

	return fmt.Sprintf("%stail -c 1M %s", prefix, t.path)
}
