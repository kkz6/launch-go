// Command console is the launch-go console runner (Artisan-style). It builds a
// console.Application, registers built-in commands, and dispatches by signature.
//
// Module commands are collected via the kernel's CommandProvider interface; as
// modules add their own commands packages, wire them here through
// kernel.BootCommands once the modules are constructed.
//
// Usage:
//
//	go run ./cmd/console about
//	go run ./cmd/console <category:name> [args] [--flags]
package main

import (
	"fmt"
	"os"

	"github.com/kkz6/launch-go/internal/pkg/console"
)

// Version is set via ldflags during build.
var Version = "development"

func main() {
	cliApp := console.NewApplication("launch", Version)

	cliApp.Register(
		&aboutCommand{},
		&workerCommand{},
		&scriptsRenderCommand{},
		&imagesValidateCommand{},
		&polarSetupCommand{},
	)

	// Database migration commands.
	cliApp.Register(migrateCommands()...)

	if err := cliApp.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// aboutCommand prints application info — a self-contained example that
// exercises the Context's table rendering.
type aboutCommand struct{}

func (aboutCommand) Signature() string   { return "about" }
func (aboutCommand) Description() string { return "Display information about the application" }
func (aboutCommand) Extend() console.Extend {
	return console.Extend{Category: "system"}
}

func (aboutCommand) Handle(ctx console.Context) error {
	ctx.Info("Launch")
	ctx.NewLine()
	ctx.Table(
		[]string{"Key", "Value"},
		[][]string{
			{"Application", "Launch"},
			{"Version", Version},
			{"Runtime", "Go"},
		},
	)
	return nil
}
