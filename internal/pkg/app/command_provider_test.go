package app_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/console"
)

// pingCommand is a trivial console command used to prove the registrar path.
type pingCommand struct{}

func (pingCommand) Signature() string      { return "demo:ping" }
func (pingCommand) Description() string    { return "ping" }
func (pingCommand) Extend() console.Extend { return console.Extend{} }
func (pingCommand) Handle(ctx console.Context) error {
	ctx.Info("pong")
	return nil
}

// providerModule is a module that exposes console commands.
type providerModule struct{ app.Base }

func (providerModule) Name() string { return "provider" }
func (providerModule) Commands() []console.Command {
	return []console.Command{pingCommand{}}
}

// plainModule implements only Module (no commands) to confirm it's skipped.
type plainModule struct{ app.Base }

func (plainModule) Name() string { return "plain" }

func TestKernel_BootCommands_RegistersModuleCommands(t *testing.T) {
	kernel := app.NewKernel(nil)
	kernel.Register(plainModule{})
	kernel.Register(providerModule{})

	cliApp := console.NewApplication("launch", "test")
	var out bytes.Buffer
	cliApp.SetIO(strings.NewReader(""), &out)

	kernel.BootCommands(cliApp)

	// The module's command should now be dispatchable.
	err := cliApp.Call("demo:ping", nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "pong")
}
