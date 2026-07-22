// Package tasktemplates wires feature-owned shell templates into the shared
// taskrunner renderer. It belongs at the application boundary so the
// taskrunner package does not depend on individual feature modules.
package tasktemplates

import (
	dbtemplates "github.com/kkz6/launch-go/internal/modules/database/tasks/templates"
	servertemplates "github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	sitetemplates "github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// RegisterAll registers the shell templates supplied by every feature module.
// Call it once during process startup, before rendering or creating tasks.
func RegisterAll() error {
	if err := templates.Register("server", servertemplates.FS, nil); err != nil {
		return err
	}
	if err := templates.Register("site", sitetemplates.FS, &templates.RegisterOptions{UseLenientShellMode: true}); err != nil {
		return err
	}
	return templates.Register("database", dbtemplates.FS, nil)
}

// MustRegisterAll registers all feature templates or panics. It is suitable
// only for process startup and test setup, where template registration failure
// makes further execution invalid.
func MustRegisterAll() {
	if err := RegisterAll(); err != nil {
		panic("failed to register task templates: " + err.Error())
	}
}
