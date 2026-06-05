package console

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// output styles
var (
	styleInfo    = lipgloss.NewStyle()
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	styleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	styleComment = lipgloss.NewStyle().Faint(true)
	styleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62"))
)

// cobraContext implements Context, backed by a cobra command's parsed flags and
// a configurable IO pair (so commands are testable with injected stdin/stdout).
type cobraContext struct {
	cmd  *cobra.Command
	args []string
	in   io.Reader
	out  io.Writer
}

// --- Arguments ---

func (c *cobraContext) Argument(index int) string {
	if index >= 0 && index < len(c.args) {
		return c.args[index]
	}
	return ""
}

func (c *cobraContext) Arguments() []string { return c.args }

func (c *cobraContext) ArgumentInt(index int) int {
	v, _ := strconv.Atoi(c.Argument(index))
	return v
}

func (c *cobraContext) ArgumentInt64(index int) int64 {
	v, _ := strconv.ParseInt(c.Argument(index), 10, 64)
	return v
}

func (c *cobraContext) ArgumentFloat64(index int) float64 {
	v, _ := strconv.ParseFloat(c.Argument(index), 64)
	return v
}

func (c *cobraContext) ArgumentBool(index int) bool {
	v, _ := strconv.ParseBool(c.Argument(index))
	return v
}

// --- Options ---

func (c *cobraContext) Option(key string) string {
	v, _ := c.cmd.Flags().GetString(key)
	return v
}

func (c *cobraContext) OptionBool(key string) bool {
	v, _ := c.cmd.Flags().GetBool(key)
	return v
}

func (c *cobraContext) OptionInt(key string) int {
	v, _ := c.cmd.Flags().GetInt(key)
	return v
}

func (c *cobraContext) OptionInt64(key string) int64 {
	v, _ := c.cmd.Flags().GetInt64(key)
	return v
}

func (c *cobraContext) OptionFloat64(key string) float64 {
	v, _ := c.cmd.Flags().GetFloat64(key)
	return v
}

func (c *cobraContext) OptionSlice(key string) []string {
	v, _ := c.cmd.Flags().GetStringSlice(key)
	return v
}

func (c *cobraContext) OptionIntSlice(key string) []int {
	v, _ := c.cmd.Flags().GetIntSlice(key)
	return v
}

// --- Output ---

func (c *cobraContext) Info(message string) { _, _ = fmt.Fprintln(c.out, styleInfo.Render(message)) }
func (c *cobraContext) Success(message string) {
	_, _ = fmt.Fprintln(c.out, styleSuccess.Render(message))
}
func (c *cobraContext) Warning(message string) {
	_, _ = fmt.Fprintln(c.out, styleWarning.Render(message))
}
func (c *cobraContext) Error(message string) { _, _ = fmt.Fprintln(c.out, styleError.Render(message)) }
func (c *cobraContext) Line(message string)  { _, _ = fmt.Fprintln(c.out, message) }
func (c *cobraContext) Comment(message string) {
	_, _ = fmt.Fprintln(c.out, styleComment.Render(message))
}

func (c *cobraContext) NewLine(times ...int) {
	n := 1
	if len(times) > 0 && times[0] > 0 {
		n = times[0]
	}
	_, _ = fmt.Fprint(c.out, strings.Repeat("\n", n))
}

// --- Interactive prompts ---

func (c *cobraContext) readLine() (string, error) {
	reader := bufio.NewReader(c.in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (c *cobraContext) Ask(question string, options ...AskOption) (string, error) {
	def := ""
	if len(options) > 0 {
		def = options[0].Default
	}
	if def != "" {
		_, _ = fmt.Fprintf(c.out, "%s [%s]: ", question, def)
	} else {
		_, _ = fmt.Fprintf(c.out, "%s: ", question)
	}
	answer, err := c.readLine()
	if err != nil {
		return "", err
	}
	if answer == "" {
		return def, nil
	}
	return answer, nil
}

func (c *cobraContext) Secret(question string, options ...AskOption) (string, error) {
	// Use no-echo input only for a real terminal; otherwise read a line so the
	// method is testable and pipe-friendly.
	if f, ok := c.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		_, _ = fmt.Fprintf(c.out, "%s: ", question)
		b, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(c.out)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return c.Ask(question, options...)
}

func (c *cobraContext) Confirm(question string, options ...ConfirmOption) bool {
	def := false
	if len(options) > 0 {
		def = options[0].Default
	}
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	_, _ = fmt.Fprintf(c.out, "%s [%s]: ", question, hint)
	answer, err := c.readLine()
	if err != nil {
		return def
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "" {
		return def
	}
	return answer == "y" || answer == "yes"
}

func (c *cobraContext) Choice(question string, choices []Choice, options ...ChoiceOption) (string, error) {
	def := ""
	if len(options) > 0 {
		def = options[0].Default
	}
	_, _ = fmt.Fprintln(c.out, question)
	for i, ch := range choices {
		_, _ = fmt.Fprintf(c.out, "  [%d] %s\n", i+1, ch.Label)
	}
	_, _ = fmt.Fprint(c.out, "> ")
	answer, err := c.readLine()
	if err != nil {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return def, nil
	}
	// Accept either a 1-based index or a choice key/label.
	if n, convErr := strconv.Atoi(answer); convErr == nil && n >= 1 && n <= len(choices) {
		return choices[n-1].Key, nil
	}
	for _, ch := range choices {
		if ch.Key == answer || ch.Label == answer {
			return ch.Key, nil
		}
	}
	return "", fmt.Errorf("invalid choice: %s", answer)
}

func (c *cobraContext) MultiSelect(question string, choices []Choice) ([]string, error) {
	_, _ = fmt.Fprintln(c.out, question+" (comma-separated)")
	for i, ch := range choices {
		_, _ = fmt.Fprintf(c.out, "  [%d] %s\n", i+1, ch.Label)
	}
	_, _ = fmt.Fprint(c.out, "> ")
	answer, err := c.readLine()
	if err != nil {
		return nil, err
	}
	var selected []string
	for _, part := range strings.Split(answer, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, convErr := strconv.Atoi(part); convErr == nil && n >= 1 && n <= len(choices) {
			selected = append(selected, choices[n-1].Key)
			continue
		}
		for _, ch := range choices {
			if ch.Key == part || ch.Label == part {
				selected = append(selected, ch.Key)
			}
		}
	}
	return selected, nil
}

// --- Rich rendering ---

func (c *cobraContext) Table(headers []string, rows [][]string) {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return styleHeader.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)
	_, _ = fmt.Fprintln(c.out, t.Render())
}

func (c *cobraContext) Spinner(message string, fn func() error) error {
	// Simple, dependency-free spinner: announce, run, report. A live animation
	// is reserved for interactive terminals and intentionally omitted here to
	// keep output clean in pipes, CI, and tests.
	_, _ = fmt.Fprintf(c.out, "%s ... ", message)
	if err := fn(); err != nil {
		_, _ = fmt.Fprintln(c.out, styleError.Render("failed"))
		return err
	}
	_, _ = fmt.Fprintln(c.out, styleSuccess.Render("done"))
	return nil
}

func (c *cobraContext) CreateProgressBar(total int) Progress {
	return &textProgress{out: c.out, total: total}
}
