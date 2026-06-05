// Package console provides a structured, self-describing console command
// system — the launch-go equivalent of Laravel's Artisan / Goravel's console
// commands. Each command declares a signature, description, flags, and a
// Handle method that receives a rich Context (arguments, typed options,
// interactive prompts, tables, spinners, progress bars).
//
// The package is domain-agnostic: commands live in their owning modules and are
// collected by the application kernel via the CommandProvider interface, then
// dispatched by signature through a cobra-backed Application.
package console

// Command is a single console command. Implementations live in their module
// (e.g. internal/modules/server/commands) and are registered through the
// kernel's CommandProvider interface.
type Command interface {
	// Signature is the unique command name, "category:name" style
	// (e.g. "server:list", "jwt:secret").
	Signature() string
	// Description is the one-line help text.
	Description() string
	// Extend declares the command's category and flags.
	Extend() Extend
	// Handle executes the command against the provided Context.
	Handle(ctx Context) error
}

// Extend declares command metadata: its help category and flag set.
type Extend struct {
	Category string
	Flags    []Flag
}

// Flag is a command-line flag declaration. The concrete types below are the
// supported flag kinds; the sealed marker keeps the set closed so the
// application can exhaustively register them with the underlying parser.
type Flag interface{ isFlag() }

// StringFlag declares a string option.
type StringFlag struct {
	Name     string
	Aliases  []string
	Usage    string
	Value    string
	Required bool
}

// BoolFlag declares a boolean option.
type BoolFlag struct {
	Name    string
	Aliases []string
	Usage   string
	Value   bool
}

// IntFlag declares an integer option.
type IntFlag struct {
	Name     string
	Aliases  []string
	Usage    string
	Value    int
	Required bool
}

// Int64Flag declares a 64-bit integer option.
type Int64Flag struct {
	Name     string
	Aliases  []string
	Usage    string
	Value    int64
	Required bool
}

// Float64Flag declares a float option.
type Float64Flag struct {
	Name     string
	Aliases  []string
	Usage    string
	Value    float64
	Required bool
}

// StringSliceFlag declares a repeatable string option.
type StringSliceFlag struct {
	Name    string
	Aliases []string
	Usage   string
	Value   []string
}

// IntSliceFlag declares a repeatable integer option.
type IntSliceFlag struct {
	Name    string
	Aliases []string
	Usage   string
	Value   []int
}

func (StringFlag) isFlag()      {}
func (BoolFlag) isFlag()        {}
func (IntFlag) isFlag()         {}
func (Int64Flag) isFlag()       {}
func (Float64Flag) isFlag()     {}
func (StringSliceFlag) isFlag() {}
func (IntSliceFlag) isFlag()    {}

// Choice is a selectable option for Choice/MultiSelect prompts.
type Choice struct {
	// Key is the value returned when selected; Label is shown to the user.
	Key   string
	Label string
}
