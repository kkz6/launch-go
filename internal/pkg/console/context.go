package console

// Context is the runtime handed to Command.Handle. It exposes parsed arguments
// and typed options, console output helpers, interactive prompts, and rich
// rendering (tables, spinners, progress bars). It mirrors the capability
// surface of Goravel's console.Context, backed by cobra + a TTY-aware IO layer.
type Context interface {
	// --- Arguments (positional) ---
	Argument(index int) string
	Arguments() []string
	ArgumentInt(index int) int
	ArgumentInt64(index int) int64
	ArgumentFloat64(index int) float64
	ArgumentBool(index int) bool

	// --- Options (flags) ---
	Option(key string) string
	OptionBool(key string) bool
	OptionInt(key string) int
	OptionInt64(key string) int64
	OptionFloat64(key string) float64
	OptionSlice(key string) []string
	OptionIntSlice(key string) []int

	// --- Output ---
	Info(message string)
	Success(message string)
	Warning(message string)
	Error(message string)
	Line(message string)
	Comment(message string)
	NewLine(times ...int)

	// --- Interactive prompts ---
	Ask(question string, options ...AskOption) (string, error)
	Secret(question string, options ...AskOption) (string, error)
	Confirm(question string, options ...ConfirmOption) bool
	Choice(question string, choices []Choice, options ...ChoiceOption) (string, error)
	MultiSelect(question string, choices []Choice) ([]string, error)

	// --- Rich rendering ---
	Table(headers []string, rows [][]string)
	Spinner(message string, fn func() error) error
	CreateProgressBar(total int) Progress
}

// AskOption configures Ask/Secret prompts.
type AskOption struct {
	// Default is returned when the user submits an empty line.
	Default string
}

// ConfirmOption configures Confirm prompts.
type ConfirmOption struct {
	// Default is the answer when the user submits an empty line.
	Default bool
}

// ChoiceOption configures Choice prompts.
type ChoiceOption struct {
	// Default is the choice key preselected on an empty line.
	Default string
}

// Progress is a console progress bar.
type Progress interface {
	// Advance moves the bar forward by step (default 1).
	Advance(step ...int)
	// Finish completes and clears the bar.
	Finish()
}
