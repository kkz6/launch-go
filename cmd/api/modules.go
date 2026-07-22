package main

import "github.com/kkz6/launch-go/internal/pkg/app"

// registerModules assembles the module graph, then boots it in dependency order.
func (a *Application) registerModules() {
	modules := a.newModules()
	a.kernel = app.NewKernel(a.logger)

	modules.wire(a)
	modules.register(a.kernel)

	a.kernel.BootTaskCallbacks()
	if err := a.kernel.Boot(); err != nil {
		a.logger.Fatal().Err(err).Msg("Failed to boot modules")
	}

	a.registerRoutes(modules)
}
