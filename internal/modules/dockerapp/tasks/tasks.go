package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// LifecycleAction names a docker lifecycle verb.
type LifecycleAction string

const (
	LifecycleStart   LifecycleAction = "start"
	LifecycleStop    LifecycleAction = "stop"
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

// EnvVar is a key/value pair rendered into the deploy script.
type EnvVar struct {
	Key   string
	Value string
}

// Port maps a host port to a container port.
type Port struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

// Volume describes a named volume mount on the host.
type Volume struct {
	HostName  string
	MountPath string
}

// DeployOptions configures the deploy script.
type DeployOptions struct {
	AppName       string
	Container     string
	ImageRef      string
	RestartPolicy string

	RegistryURL      string
	RegistryUsername string
	RegistryPassword string

	EnvVars []EnvVar
	Ports   []Port
	Volumes []Volume
	Labels  []string
}

// Deploy returns a task that pulls the image (with optional registry
// login), drops any prior container, ensures named volumes exist, and
// runs a fresh container on launch-network with the configured env /
// ports / volumes / Traefik labels.
func Deploy(opts DeployOptions) *taskrunner.BaseTask {
	data := struct {
		Container        string
		ImageRef         string
		RestartPolicy    string
		RegistryURL      string
		RegistryUsername string
		RegistryPassword string
		EnvVars          []EnvVar
		Ports            []Port
		Volumes          []Volume
		Labels           []string
	}{
		Container:        opts.Container,
		ImageRef:         opts.ImageRef,
		RestartPolicy:    opts.RestartPolicy,
		RegistryURL:      opts.RegistryURL,
		RegistryUsername: opts.RegistryUsername,
		RegistryPassword: opts.RegistryPassword,
		EnvVars:          opts.EnvVars,
		Ports:            opts.Ports,
		Volumes:          opts.Volumes,
		Labels:           opts.Labels,
	}
	script := templates.MustRender("dockerapp", "dockerapp/deploy.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deploy "+opts.AppName),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(900),
	)
}

// UninstallOptions configures the uninstall script.
type UninstallOptions struct {
	AppName    string
	Container  string
	Volumes    []Volume
	RemoveData bool
}

// Uninstall returns a task that removes the container and (optionally)
// the named volumes the app was using.
func Uninstall(opts UninstallOptions) *taskrunner.BaseTask {
	data := struct {
		Container  string
		Volumes    []Volume
		RemoveData bool
	}{
		Container:  opts.Container,
		Volumes:    opts.Volumes,
		RemoveData: opts.RemoveData,
	}
	script := templates.MustRender("dockerapp", "dockerapp/uninstall.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Uninstall "+opts.AppName),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// LifecycleOptions configures a start/stop/restart task.
type LifecycleOptions struct {
	AppName   string
	Container string
	Action    LifecycleAction
}

// Lifecycle returns a task that runs a docker lifecycle verb against an
// application container.
func Lifecycle(opts LifecycleOptions) *taskrunner.BaseTask {
	data := struct {
		Action    string
		Container string
	}{
		Action:    string(opts.Action),
		Container: opts.Container,
	}
	script := templates.MustRender("dockerapp", "dockerapp/lifecycle.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName(opts.Action.Label()+" "+opts.AppName),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}

// LogsOptions configures a logs-tail task.
type LogsOptions struct {
	AppName    string
	Container  string
	Tail       int
	Timestamps bool
}

// Logs returns a task that prints recent log lines.
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
	script := templates.MustRender("dockerapp", "dockerapp/logs.sh", data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Tail "+opts.AppName+" logs"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
