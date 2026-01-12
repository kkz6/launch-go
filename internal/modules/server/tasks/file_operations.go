package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// GetFileTask retrieves file contents from the server
type GetFileTask struct {
	*ServerTask
	Path   string
	Lines  *int
	IsRoot bool
}

// NewGetFileTask creates a new GetFileTask
func NewGetFileTask(path string, lines *int, isRoot bool) *GetFileTask {
	task := &GetFileTask{
		Path:   path,
		Lines:  lines,
		IsRoot: isRoot,
	}

	var script string
	sudo := ""
	if isRoot {
		sudo = "sudo "
	}

	if lines != nil && *lines > 0 {
		script = fmt.Sprintf("%stail -n %d %s", sudo, *lines, path)
	} else {
		script = fmt.Sprintf("%stail -c 1M %s", sudo, path)
	}

	task.ServerTask = NewServerTaskWithName("Get File", script, 30)
	return task
}

// GetFile creates a task to get file contents
func GetFile(path string, lines *int, isRoot bool) *taskrunner.BaseTask {
	sudo := ""
	if isRoot {
		sudo = "sudo "
	}

	var script string
	if lines != nil && *lines > 0 {
		script = fmt.Sprintf("%stail -n %d %s", sudo, *lines, path)
	} else {
		script = fmt.Sprintf("%stail -c 1M %s", sudo, path)
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(30),
	)
}

// DeleteFileTask deletes a file from the server
type DeleteFileTask struct {
	*ServerTask
	Path   string
	IsRoot bool
}

// NewDeleteFileTask creates a new DeleteFileTask
func NewDeleteFileTask(path string, isRoot bool) *DeleteFileTask {
	sudo := ""
	if isRoot {
		sudo = "sudo "
	}

	script := fmt.Sprintf("%srm -f %s", sudo, path)

	return &DeleteFileTask{
		ServerTask: NewServerTaskWithName("Delete File", script, 30),
		Path:       path,
		IsRoot:     isRoot,
	}
}

// DeleteFile creates a task to delete a file
func DeleteFile(path string, isRoot bool) *taskrunner.BaseTask {
	sudo := ""
	if isRoot {
		sudo = "sudo "
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete File"),
		taskrunner.WithScript(fmt.Sprintf("%srm -f %s", sudo, path)),
		taskrunner.WithTimeout(30),
	)
}

// UploadFileTask uploads content to a file on the server
type UploadFileTask struct {
	*ServerTask
	Path     string
	Content  string
	IsRoot   bool
	FileMode string
}

// NewUploadFileTask creates a new UploadFileTask
func NewUploadFileTask(path string, content string, isRoot bool, fileMode string) *UploadFileTask {
	sudo := ""
	if isRoot {
		sudo = "sudo "
	}

	if fileMode == "" {
		fileMode = "644"
	}

	// Use heredoc to write content to file
	script := fmt.Sprintf(`%scat > %s << 'UPLOAD_EOF'
%s
UPLOAD_EOF
%schmod %s %s`, sudo, path, content, sudo, fileMode, path)

	return &UploadFileTask{
		ServerTask: NewServerTaskWithName("Upload File", script, 60),
		Path:       path,
		Content:    content,
		IsRoot:     isRoot,
		FileMode:   fileMode,
	}
}

// UploadFile creates a task to upload content to a file
func UploadFile(path string, content string, isRoot bool, fileMode string) *taskrunner.BaseTask {
	sudo := ""
	if isRoot {
		sudo = "sudo "
	}

	if fileMode == "" {
		fileMode = "644"
	}

	script := fmt.Sprintf(`%scat > %s << 'UPLOAD_EOF'
%s
UPLOAD_EOF
%schmod %s %s`, sudo, path, content, sudo, fileMode, path)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Upload File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(60),
	)
}
