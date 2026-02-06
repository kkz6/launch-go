package templates

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// EmailBuilder builds HTML emails using a fluent interface
type EmailBuilder struct {
	appName    string
	appURL     string
	greeting   string
	intros     []string
	content    string
	actions    []Action
	panels     []Panel
	tables     []Table
	outros     []string
	subcopy    string
	footerText string
}

// Action represents a call-to-action button
type Action struct {
	Text  string
	URL   string
	Color string // "primary", "success", "error"
}

// Panel represents an info panel
type Panel struct {
	Content string
}

// Table represents a data table
type Table struct {
	Headers []string
	Rows    [][]string
}

// NewEmailBuilder creates a new email builder
func NewEmailBuilder(appName, appURL string) *EmailBuilder {
	return &EmailBuilder{
		appName:  appName,
		appURL:   appURL,
		greeting: "Hello",
	}
}

// WithGreeting sets the greeting (e.g., "Hello", "Hi John")
func (b *EmailBuilder) WithGreeting(greeting string) *EmailBuilder {
	b.greeting = greeting
	return b
}

// WithIntro adds an intro paragraph
func (b *EmailBuilder) WithIntro(text string) *EmailBuilder {
	b.intros = append(b.intros, text)
	return b
}

// WithMarkdownContent sets markdown content that will be rendered as HTML
func (b *EmailBuilder) WithMarkdownContent(markdown string) *EmailBuilder {
	b.content = markdown
	return b
}

// WithAction adds a call-to-action button
func (b *EmailBuilder) WithAction(text, url, color string) *EmailBuilder {
	if color == "" {
		color = "primary"
	}
	b.actions = append(b.actions, Action{Text: text, URL: url, Color: color})
	return b
}

// WithPanel adds an info panel
func (b *EmailBuilder) WithPanel(content string) *EmailBuilder {
	b.panels = append(b.panels, Panel{Content: content})
	return b
}

// WithTable adds a data table
func (b *EmailBuilder) WithTable(headers []string, rows [][]string) *EmailBuilder {
	b.tables = append(b.tables, Table{Headers: headers, Rows: rows})
	return b
}

// WithOutro adds an outro paragraph
func (b *EmailBuilder) WithOutro(text string) *EmailBuilder {
	b.outros = append(b.outros, text)
	return b
}

// WithSubcopy adds subcopy text (small text at the bottom, typically for URL fallbacks)
func (b *EmailBuilder) WithSubcopy(text string) *EmailBuilder {
	b.subcopy = text
	return b
}

// WithFooter sets custom footer text
func (b *EmailBuilder) WithFooter(text string) *EmailBuilder {
	b.footerText = text
	return b
}

// Build generates the HTML email
func (b *EmailBuilder) Build() (string, error) {
	data := templateData{
		AppName:    b.appName,
		AppURL:     b.appURL,
		Greeting:   b.greeting,
		Intros:     b.intros,
		Content:    template.HTML(renderMarkdown(b.content)),
		Actions:    b.actions,
		Panels:     b.buildPanels(),
		Tables:     b.tables,
		Outros:     b.outros,
		Subcopy:    b.subcopy,
		FooterText: b.footerText,
	}

	tmpl, err := template.New("email").Parse(baseLayout)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// BuildPlainText generates a plain text version of the email
func (b *EmailBuilder) BuildPlainText() string {
	var buf bytes.Buffer

	if b.greeting != "" {
		_, _ = buf.WriteString(b.greeting + "\n\n")
	}

	for _, intro := range b.intros {
		_, _ = buf.WriteString(intro + "\n\n")
	}

	if b.content != "" {
		_, _ = buf.WriteString(b.content + "\n\n")
	}

	for _, action := range b.actions {
		_, _ = buf.WriteString(action.Text + ": " + action.URL + "\n\n")
	}

	for _, panel := range b.panels {
		_, _ = buf.WriteString("---\n" + panel.Content + "\n---\n\n")
	}

	for _, outro := range b.outros {
		_, _ = buf.WriteString(outro + "\n\n")
	}

	if b.subcopy != "" {
		_, _ = buf.WriteString("\n" + b.subcopy + "\n")
	}

	return buf.String()
}

func (b *EmailBuilder) buildPanels() []template.HTML {
	panels := make([]template.HTML, len(b.panels))
	for i, p := range b.panels {
		panels[i] = template.HTML(renderMarkdown(p.Content))
	}
	return panels
}

type templateData struct {
	AppName    string
	AppURL     string
	Greeting   string
	Intros     []string
	Content    template.HTML
	Actions    []Action
	Panels     []template.HTML
	Tables     []Table
	Outros     []string
	Subcopy    string
	FooterText string
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

func renderMarkdown(content string) string {
	if content == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return content
	}
	return buf.String()
}
