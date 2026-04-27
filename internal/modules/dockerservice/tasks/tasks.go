package tasks

import (
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// LifecycleAction names a docker lifecycle verb supported by Lifecycle().
type LifecycleAction string

const (
	// LifecycleStart starts a stopped container.
	LifecycleStart LifecycleAction = "start"
	// LifecycleStop stops a running container.
	LifecycleStop LifecycleAction = "stop"
	// LifecycleRestart restarts a container.
	LifecycleRestart LifecycleAction = "restart"
)

// Label returns a verb suitable for task names ("Start", "Stop", "Restart").
func (a LifecycleAction) Label() string {
	switch a {
	case LifecycleStart:
		return "Start"
	case LifecycleStop:
		return "Stop"
	case LifecycleRestart:
		return "Restart"
	}
	return string(a)
}

// InstallOptions configures the install run script.
type InstallOptions struct {
	Kind         types.Kind
	Image        string
	Container    string
	Volume       string
	Username     string
	Password     string
	DatabaseName string
}

// Install returns a task that pulls the image, ensures the volume, drops
// any prior container, and starts a fresh one on launch-network.
func Install(opts InstallOptions) *taskrunner.BaseTask {
	data := struct {
		Image        string
		Container    string
		Volume       string
		Username     string
		Password     string
		DatabaseName string
	}{
		Image:        opts.Image,
		Container:    opts.Container,
		Volume:       opts.Volume,
		Username:     opts.Username,
		Password:     opts.Password,
		DatabaseName: opts.DatabaseName,
	}
	script := templates.MustRender("dockerservice", opts.Kind.RunTemplateName(), data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install "+opts.Kind.Label()),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(900),
	)
}

// UninstallOptions configures the uninstall run script.
type UninstallOptions struct {
	Kind       types.Kind
	Container  string
	Volume     string
	RemoveData bool
}

// Uninstall returns a task that removes the container and (optionally)
// the named volume holding its data.
func Uninstall(opts UninstallOptions) *taskrunner.BaseTask {
	data := struct {
		Container  string
		Volume     string
		RemoveData bool
	}{
		Container:  opts.Container,
		Volume:     opts.Volume,
		RemoveData: opts.RemoveData,
	}
	script := templates.MustRender("dockerservice", "dockerservice/uninstall.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Uninstall "+opts.Kind.Label()),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// LifecycleOptions configures a start/stop/restart task.
type LifecycleOptions struct {
	Kind      types.Kind
	Container string
	Action    LifecycleAction
}

// Lifecycle returns a task that runs a docker lifecycle verb against a
// docker service container.
func Lifecycle(opts LifecycleOptions) *taskrunner.BaseTask {
	data := struct {
		Action    string
		Container string
	}{
		Action:    string(opts.Action),
		Container: opts.Container,
	}
	script := templates.MustRender("dockerservice", "dockerservice/lifecycle.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName(opts.Action.Label()+" "+opts.Kind.Label()),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}

// LogsOptions configures a logs-tail task.
type LogsOptions struct {
	Kind       types.Kind
	Container  string
	Tail       int
	Timestamps bool
}

// Logs returns a task that prints the most recent N log lines for a
// docker service container. Tail defaults to 100 if unset.
func Logs(opts LogsOptions) *taskrunner.BaseTask {
	tail := opts.Tail
	if tail <= 0 {
		tail = 100
	}
	data := struct {
		Container  string
		Tail       int
		Timestamps bool
	}{
		Container:  opts.Container,
		Tail:       tail,
		Timestamps: opts.Timestamps,
	}
	script := templates.MustRender("dockerservice", "dockerservice/logs.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Tail "+opts.Kind.Label()+" logs"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
