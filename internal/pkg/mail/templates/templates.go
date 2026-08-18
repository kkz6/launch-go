package templates

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// EmailBuilder builds HTML emails using a fluent interface
type EmailBuilder struct {
	appName         string
	appURL          string
	preheader       string
	greeting        string
	context         string
	stateLabel      string
	stateTone       string
	blocks          []emailBlock
	renderedContent template.HTML
	layoutVariant   string
	subcopy         string
	footerText      string
}

// emailBlock preserves the sequence in which content is added to an email.
// Transactional messages are event traces, so order is part of their meaning.
type emailBlock struct {
	Kind    string
	Content template.HTML
	Plain   string
	Action  Action
	Table   Table
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
		appName:    appName,
		appURL:     appURL,
		greeting:   "Hello",
		context:    "lctl / notification",
		stateLabel: "EVENT",
		stateTone:  "neutral",
	}
}

// WithGreeting sets the greeting (e.g., "Hello", "Hi John")
func (b *EmailBuilder) WithGreeting(greeting string) *EmailBuilder {
	b.greeting = greeting
	return b
}

// WithPreheader sets the hidden inbox preview text shown by supporting clients.
func (b *EmailBuilder) WithPreheader(preheader string) *EmailBuilder {
	b.preheader = preheader
	return b
}

// WithContext sets the compact product route shown above the event title.
func (b *EmailBuilder) WithContext(context string) *EmailBuilder {
	b.context = context
	return b
}

// WithState sets the event state and semantic tone. Supported tones are
// neutral, success, warning, and error.
func (b *EmailBuilder) WithState(label, tone string) *EmailBuilder {
	b.stateLabel = label
	switch tone {
	case "success", "warning", "error":
		b.stateTone = tone
	default:
		b.stateTone = "neutral"
	}
	return b
}

// WithIntro adds an intro paragraph
func (b *EmailBuilder) WithIntro(text string) *EmailBuilder {
	b.blocks = append(b.blocks, markdownBlock("intro", text))
	return b
}

// WithMarkdownContent sets markdown content that will be rendered as HTML
func (b *EmailBuilder) WithMarkdownContent(markdown string) *EmailBuilder {
	b.renderedContent = ""
	b.blocks = append(b.blocks, markdownBlock("content", markdown))
	return b
}

// withRenderedContent sets content that was already rendered by html/template.
// It stays private so callers cannot bypass contextual escaping with arbitrary HTML.
func (b *EmailBuilder) withRenderedContent(content template.HTML) *EmailBuilder {
	b.renderedContent = content
	b.blocks = nil
	return b
}

// withLayoutVariant selects a private shared-shell treatment for a template.
// Keeping this private prevents callers from coupling arbitrary emails to
// internal layout details.
func (b *EmailBuilder) withLayoutVariant(variant string) *EmailBuilder {
	b.layoutVariant = variant
	return b
}

// WithAction adds a call-to-action button
func (b *EmailBuilder) WithAction(text, url, color string) *EmailBuilder {
	if color == "" {
		color = "primary"
	}
	b.blocks = append(b.blocks, emailBlock{
		Kind:   "action",
		Plain:  text + ": " + url,
		Action: Action{Text: text, URL: url, Color: color},
	})
	return b
}

// WithPanel adds an info panel
func (b *EmailBuilder) WithPanel(content string) *EmailBuilder {
	b.blocks = append(b.blocks, markdownBlock("panel", content))
	return b
}

// WithTable adds a data table
func (b *EmailBuilder) WithTable(headers []string, rows [][]string) *EmailBuilder {
	b.blocks = append(b.blocks, emailBlock{
		Kind:  "table",
		Table: Table{Headers: headers, Rows: rows},
	})
	return b
}

// WithOutro adds an outro paragraph
func (b *EmailBuilder) WithOutro(text string) *EmailBuilder {
	b.blocks = append(b.blocks, markdownBlock("outro", text))
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
		AppName:           b.appName,
		BrandName:         emailBrandName(b.appName),
		AppURL:            b.appURL,
		LayoutVariant:     b.layoutVariant,
		Preheader:         b.preheader,
		Greeting:          b.greeting,
		Context:           b.context,
		StateLabel:        b.stateLabel,
		StateTone:         b.stateTone,
		Blocks:            b.blocks,
		Content:           b.renderedContent,
		Subcopy:           b.subcopy,
		FooterText:        b.footerText,
		DirectionContract: template.HTML(directionContract),
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

	for _, block := range b.blocks {
		switch block.Kind {
		case "panel":
			_, _ = buf.WriteString("---\n" + block.Plain + "\n---\n\n")
		case "table":
			writePlainTextTable(&buf, block.Table)
		default:
			if block.Plain != "" {
				_, _ = buf.WriteString(block.Plain + "\n\n")
			}
		}
	}

	if b.subcopy != "" {
		_, _ = buf.WriteString("\n" + b.subcopy + "\n")
	}

	return buf.String()
}

func markdownBlock(kind, content string) emailBlock {
	return emailBlock{
		Kind:    kind,
		Content: template.HTML(renderMarkdown(content)),
		Plain:   content,
	}
}

func writePlainTextTable(buf *bytes.Buffer, table Table) {
	if len(table.Headers) > 0 {
		_, _ = buf.WriteString(strings.Join(table.Headers, " | ") + "\n")
	}
	for _, row := range table.Rows {
		_, _ = buf.WriteString(strings.Join(row, " | ") + "\n")
	}
	_, _ = buf.WriteString("\n")
}

type templateData struct {
	AppName           string
	BrandName         string
	AppURL            string
	LayoutVariant     string
	Preheader         string
	Greeting          string
	Context           string
	StateLabel        string
	StateTone         string
	Blocks            []emailBlock
	Content           template.HTML
	Subcopy           string
	FooterText        string
	DirectionContract template.HTML
}

func emailBrandName(appName string) string {
	name := strings.TrimSpace(appName)
	if name == "" || strings.EqualFold(name, "launch") {
		return "launchctl"
	}

	return name
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithXHTML()),
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
