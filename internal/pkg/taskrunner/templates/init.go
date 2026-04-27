package templates

import (
	dbtemplates "github.com/kkz6/launch-go/internal/modules/database/tasks/templates"
	mstemplates "github.com/kkz6/launch-go/internal/modules/managedservice/tasks/templates"
	servertemplates "github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	sitetemplates "github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
)

// RegisterAll registers all module templates with the registry.
// Call this once at application startup.
func RegisterAll() error {
	if err := Register("server", servertemplates.FS, nil); err != nil {
		return err
	}
	if err := Register("site", sitetemplates.FS, &RegisterOptions{UseLenientShellMode: true}); err != nil {
		return err
	}
	if err := Register("database", dbtemplates.FS, nil); err != nil {
		return err
	}
	return Register("managedservice", mstemplates.FS, nil)
}

// MustRegisterAll registers all templates or panics.
// Use only at application startup where panic is acceptable.
func MustRegisterAll() {
	if err := RegisterAll(); err != nil {
		panic("failed to register templates: " + err.Error())
	}
}
