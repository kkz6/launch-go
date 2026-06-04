package console_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/console"
)

// fakeCommand is a test command that records what its Handle received.
type fakeCommand struct {
	sig    string
	desc   string
	cat    string
	flags  []console.Flag
	handle func(console.Context) error
}

func (c *fakeCommand) Signature() string   { return c.sig }
func (c *fakeCommand) Description() string { return c.desc }
func (c *fakeCommand) Extend() console.Extend {
	return console.Extend{Category: c.cat, Flags: c.flags}
}
func (c *fakeCommand) Handle(ctx console.Context) error { return c.handle(ctx) }

func TestApplication_DispatchesArgumentsAndTypedFlags(t *testing.T) {
	var (
		arg0  string
		name  string
		force bool
		count int
	)
	cmd := &fakeCommand{
		sig: "demo:run",
		flags: []console.Flag{
			console.StringFlag{Name: "name", Value: "default"},
			console.BoolFlag{Name: "force"},
			console.IntFlag{Name: "count", Value: 1},
		},
		handle: func(ctx console.Context) error {
			arg0 = ctx.Argument(0)
			name = ctx.Option("name")
			force = ctx.OptionBool("force")
			count = ctx.OptionInt("count")
			ctx.Info("ran ok")
			return nil
		},
	}

	app := console.NewApplication("launch", "test")
	var out bytes.Buffer
	app.SetIO(strings.NewReader(""), &out)
	app.Register(cmd)

	err := app.Call("demo:run", []string{"hello", "--name", "alice", "--force", "--count", "5"})
	require.NoError(t, err)

	assert.Equal(t, "hello", arg0)
	assert.Equal(t, "alice", name)
	assert.True(t, force)
	assert.Equal(t, 5, count)
	assert.Contains(t, out.String(), "ran ok")
}

func TestApplication_FlagDefaults(t *testing.T) {
	var name string
	var count int
	cmd := &fakeCommand{
		sig: "demo:defaults",
		flags: []console.Flag{
			console.StringFlag{Name: "name", Value: "default"},
			console.IntFlag{Name: "count", Value: 7},
		},
		handle: func(ctx console.Context) error {
			name = ctx.Option("name")
			count = ctx.OptionInt("count")
			return nil
		},
	}
	app := console.NewApplication("launch", "test")
	app.SetIO(strings.NewReader(""), &bytes.Buffer{})
	app.Register(cmd)

	require.NoError(t, app.Call("demo:defaults", nil))
	assert.Equal(t, "default", name)
	assert.Equal(t, 7, count)
}

func TestContext_Confirm_ReadsStdin(t *testing.T) {
	cmd := &fakeCommand{
		sig: "demo:confirm",
		handle: func(ctx console.Context) error {
			if ctx.Confirm("proceed?") {
				ctx.Info("confirmed")
			} else {
				ctx.Info("declined")
			}
			return nil
		},
	}
	app := console.NewApplication("launch", "test")
	var out bytes.Buffer
	app.SetIO(strings.NewReader("y\n"), &out)
	app.Register(cmd)

	require.NoError(t, app.Call("demo:confirm", nil))
	assert.Contains(t, out.String(), "confirmed")
}

func TestContext_Ask_ReadsStdin(t *testing.T) {
	var answer string
	cmd := &fakeCommand{
		sig: "demo:ask",
		handle: func(ctx console.Context) error {
			a, err := ctx.Ask("name?")
			answer = a
			return err
		},
	}
	app := console.NewApplication("launch", "test")
	app.SetIO(strings.NewReader("Karthick\n"), &bytes.Buffer{})
	app.Register(cmd)

	require.NoError(t, app.Call("demo:ask", nil))
	assert.Equal(t, "Karthick", answer)
}

func TestApplication_UnknownCommandErrors(t *testing.T) {
	app := console.NewApplication("launch", "test")
	app.SetIO(strings.NewReader(""), &bytes.Buffer{})
	err := app.Call("does:not-exist", nil)
	assert.Error(t, err)
}
