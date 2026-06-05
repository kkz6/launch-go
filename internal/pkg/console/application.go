package console

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Application registers console commands and dispatches them by signature,
// backed by cobra for parsing and help. Build one at bootstrap, Register module
// commands, then Run with os.Args (or Call programmatically / in tests).
type Application struct {
	root   *cobra.Command
	in     io.Reader
	out    io.Writer
	groups map[string]bool
}

// NewApplication creates a console application named name at the given version.
func NewApplication(name, version string) *Application {
	root := &cobra.Command{
		Use:           name,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return &Application{root: root, in: os.Stdin, out: os.Stdout, groups: make(map[string]bool)}
}

// SetIO overrides the input/output streams (used by tests and embedding hosts).
func (a *Application) SetIO(in io.Reader, out io.Writer) {
	a.in = in
	a.out = out
	a.root.SetOut(out)
	a.root.SetErr(out)
}

// Register adds commands to the application.
func (a *Application) Register(commands ...Command) {
	for _, cmd := range commands {
		// cobra requires a group to be registered on the root before any
		// subcommand references it via GroupID.
		if cat := cmd.Extend().Category; cat != "" && !a.groups[cat] {
			a.root.AddGroup(&cobra.Group{ID: cat, Title: cat + ":"})
			a.groups[cat] = true
		}
		a.root.AddCommand(a.build(cmd))
	}
}

// build turns a Command into a cobra command, wiring flags and the Handle call.
func (a *Application) build(cmd Command) *cobra.Command {
	ext := cmd.Extend()
	cc := &cobra.Command{
		Use:           cmd.Signature(),
		Short:         cmd.Description(),
		GroupID:       ext.Category,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cc *cobra.Command, args []string) error {
			ctx := &cobraContext{cmd: cc, args: args, in: a.in, out: a.out}
			return cmd.Handle(ctx)
		},
	}
	registerFlags(cc, ext.Flags)
	return cc
}

// registerFlags declares each Flag on the cobra command's flag set.
func registerFlags(cc *cobra.Command, flags []Flag) {
	fs := cc.Flags()
	for _, f := range flags {
		switch v := f.(type) {
		case StringFlag:
			fs.StringP(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
			markRequired(cc, v.Name, v.Required)
		case BoolFlag:
			fs.BoolP(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
		case IntFlag:
			fs.IntP(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
			markRequired(cc, v.Name, v.Required)
		case Int64Flag:
			fs.Int64P(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
			markRequired(cc, v.Name, v.Required)
		case Float64Flag:
			fs.Float64P(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
			markRequired(cc, v.Name, v.Required)
		case StringSliceFlag:
			fs.StringSliceP(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
		case IntSliceFlag:
			fs.IntSliceP(v.Name, shorthand(v.Aliases), v.Value, v.Usage)
		}
	}
}

// shorthand returns a single-character alias for cobra's -x shorthand, or "".
func shorthand(aliases []string) string {
	for _, a := range aliases {
		if len(a) == 1 {
			return a
		}
	}
	return ""
}

func markRequired(cc *cobra.Command, name string, required bool) {
	if required {
		_ = cc.MarkFlagRequired(name)
	}
}

// Run parses args (typically os.Args[1:]) and executes the matched command.
func (a *Application) Run(args []string) error {
	a.root.SetArgs(args)
	return a.root.Execute()
}

// Call dispatches a single command by signature with the given args. Convenience
// for programmatic invocation and tests.
func (a *Application) Call(signature string, args []string) error {
	if a.find(signature) == nil {
		return fmt.Errorf("command not found: %s", signature)
	}
	return a.Run(append([]string{signature}, args...))
}

func (a *Application) find(signature string) *cobra.Command {
	for _, c := range a.root.Commands() {
		if c.Use == signature || c.Name() == signature {
			return c
		}
	}
	return nil
}
