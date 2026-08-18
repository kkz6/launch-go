package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type deploymentRunMeta struct {
	Label string
	Value string
}

type deploymentOutputLine struct {
	Marker       string
	Number       string
	Text         string
	IsFailure    bool
	IsCleanup    bool
	Continuation bool
}

type deploymentFailedEmailView struct {
	StatusHeading  string
	StatusLabel    string
	StatusLower    string
	SiteAddress    string
	ServerName     string
	HasServer      bool
	ShortGitHash   string
	CommitMessage  string
	HasCommit      bool
	RunMeta        []deploymentRunMeta
	DeploymentTime string
	FailureSummary string
	Output         string
	OutputLines    []deploymentOutputLine
	OutputLabel    string
	ActionURL      string
	ActionText     string
}

var (
	ansiEscapePattern       = regexp.MustCompile(`(?:\x1b\][^\x07]*(?:\x07|\x1b\\))|\x1b\[[0-?]*[ -/]*[@-~]`)
	failureLinePattern      = regexp.MustCompile(`(?i)(command not found|fatal:|error:|exception|failed|failure|permission denied|no such file|timed? out|cannot |could not |not found)`)
	shellFailureLinePattern = regexp.MustCompile(`(?i)^.*:\s*line\s+\d+:\s*(.+)$`)
)

// DeploymentFailedEmail creates a branded, structured email for deployment failures.
func DeploymentFailedEmail(siteAddress, serverName, statusLabel, gitHash, commitMessage, commitAuthor, triggeredBy string, deploymentTime time.Time, output string, siteURL string) (htmlContent string, plainText string, err error) {
	view := newDeploymentFailedEmailView(
		siteAddress,
		serverName,
		statusLabel,
		gitHash,
		commitMessage,
		commitAuthor,
		triggeredBy,
		deploymentTime,
		output,
		siteURL,
	)

	var content bytes.Buffer
	if err := deploymentTimelineContentTemplate.Execute(&content, view); err != nil {
		return "", "", err
	}

	preheader := fmt.Sprintf("%s %s on %s", view.SiteAddress, view.StatusLower, displayServerName(view.ServerName))
	if view.FailureSummary != "" {
		preheader += ": " + view.FailureSummary
	}

	builder := NewEmail().
		WithGreeting("").
		WithPreheader(preheader).
		WithFooter("Deployment alerts are enabled for your team.").
		withRenderedContent(template.HTML(content.String())).
		withLayoutVariant("incident")

	htmlContent, err = builder.Build()
	if err != nil {
		return "", "", err
	}

	return htmlContent, buildDeploymentFailedPlainText(view), nil
}

func newDeploymentFailedEmailView(siteAddress, serverName, statusLabel, gitHash, commitMessage, commitAuthor, triggeredBy string, deploymentTime time.Time, output string, siteURL string) deploymentFailedEmailView {
	cleanOutput := cleanDeploymentOutput(output)
	statusLower := strings.ToLower(strings.TrimSpace(statusLabel))
	statusHeading := "Deployment failed"
	statusDisplay := "FAILED"
	if strings.Contains(statusLower, "timed") || strings.Contains(statusLower, "timeout") {
		statusHeading = "Deployment timed out"
		statusDisplay = "TIMED OUT"
	}

	shortHash := strings.TrimSpace(gitHash)
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	message := firstTextLine(commitMessage)
	actionURL := strings.TrimSpace(siteURL)
	actionText := "Open deployment"
	if actionURL == "" {
		actionURL = strings.TrimSpace(GetConfig().AppURL)
		actionText = "Open Launch"
	}

	formattedTime := deploymentTime.UTC().Format("02 Jan 2006, 15:04 UTC")
	runMeta := make([]deploymentRunMeta, 0, 3)
	if author := strings.TrimSpace(commitAuthor); author != "" {
		runMeta = append(runMeta, deploymentRunMeta{Label: "Author", Value: author})
	}
	if actor := strings.TrimSpace(triggeredBy); actor != "" {
		runMeta = append(runMeta, deploymentRunMeta{Label: "Triggered by", Value: actor})
	}
	runMeta = append(runMeta, deploymentRunMeta{Label: "Started", Value: formattedTime})

	summary := compactDeploymentFailureSummary(deploymentFailureSummary(cleanOutput))

	return deploymentFailedEmailView{
		StatusHeading:  statusHeading,
		StatusLabel:    statusDisplay,
		StatusLower:    statusLower,
		SiteAddress:    strings.TrimSpace(siteAddress),
		ServerName:     strings.TrimSpace(serverName),
		HasServer:      strings.TrimSpace(serverName) != "",
		ShortGitHash:   shortHash,
		CommitMessage:  message,
		HasCommit:      shortHash != "" || message != "",
		RunMeta:        runMeta,
		DeploymentTime: formattedTime,
		FailureSummary: summary,
		Output:         cleanOutput,
		OutputLines:    deploymentOutputLines(cleanOutput),
		OutputLabel:    outputLineLabel(cleanOutput),
		ActionURL:      actionURL,
		ActionText:     actionText,
	}
}

func cleanDeploymentOutput(output string) string {
	cleaned := ansiEscapePattern.ReplaceAllString(output, "")
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	return strings.TrimSpace(cleaned)
}

func deploymentFailureSummary(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || isCleanupLine(line) {
			continue
		}
		if failureLinePattern.MatchString(line) {
			return truncateRunes(line, 240)
		}
	}

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !isCleanupLine(line) {
			return truncateRunes(line, 240)
		}
	}

	return ""
}

func compactDeploymentFailureSummary(summary string) string {
	matches := shellFailureLinePattern.FindStringSubmatch(strings.TrimSpace(summary))
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}

	return strings.TrimSpace(summary)
}

func deploymentOutputLines(output string) []deploymentOutputLine {
	if output == "" {
		return nil
	}

	logicalLines := strings.Split(output, "\n")
	lines := make([]deploymentOutputLine, 0, len(logicalLines))
	for index, line := range logicalLines {
		isCleanup := isCleanupLine(line)
		isFailure := !isCleanup && failureLinePattern.MatchString(line)
		segments := wrapDeploymentOutputLine(line, 68)
		for segmentIndex, segment := range segments {
			outputLine := deploymentOutputLine{
				Text:         segment,
				IsFailure:    isFailure,
				IsCleanup:    isCleanup,
				Continuation: segmentIndex > 0,
			}
			if segmentIndex == 0 {
				outputLine.Number = fmt.Sprintf("%02d", index+1)
				if isFailure {
					outputLine.Marker = "✗"
				}
			}
			lines = append(lines, outputLine)
		}
	}

	return lines
}

func wrapDeploymentOutputLine(line string, maxRunes int) []string {
	runes := []rune(line)
	if len(runes) == 0 {
		return []string{""}
	}

	segments := make([]string, 0, (len(runes)/maxRunes)+1)
	for len(runes) > maxRunes {
		splitAt := maxRunes
		for i := maxRunes; i > maxRunes-16; i-- {
			if unicode.IsSpace(runes[i-1]) {
				splitAt = i - 1
				break
			}
		}
		segments = append(segments, string(runes[:splitAt]))
		runes = runes[splitAt:]
		for len(runes) > 0 && unicode.IsSpace(runes[0]) {
			runes = runes[1:]
		}
	}
	segments = append(segments, string(runes))
	return segments
}

func isCleanupLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "cleaned up deployment credentials") ||
		strings.Contains(lower, "cleanup complete")
}

func firstTextLine(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}

	return ""
}

func outputLineLabel(output string) string {
	if output == "" {
		return ""
	}

	count := strings.Count(output, "\n") + 1
	if count == 1 {
		return "last line"
	}

	return fmt.Sprintf("last %d lines", count)
}

func displayServerName(serverName string) string {
	if strings.TrimSpace(serverName) == "" {
		return "your server"
	}

	return strings.TrimSpace(serverName)
}

func truncateRunes(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}

	return string(runes[:maxRunes-1]) + "…"
}

func buildDeploymentFailedPlainText(view deploymentFailedEmailView) string {
	var body strings.Builder
	_, _ = fmt.Fprintf(&body, "%s\n\n", view.StatusHeading)
	_, _ = fmt.Fprintf(&body, "Site: %s\n", view.SiteAddress)
	if view.HasServer {
		_, _ = fmt.Fprintf(&body, "Server: %s\n", view.ServerName)
	}
	if view.ShortGitHash != "" {
		_, _ = fmt.Fprintf(&body, "Commit: %s", view.ShortGitHash)
		if view.CommitMessage != "" {
			_, _ = fmt.Fprintf(&body, " / %s", view.CommitMessage)
		}
		_, _ = body.WriteString("\n")
	}
	for _, item := range view.RunMeta {
		_, _ = fmt.Fprintf(&body, "%s: %s\n", item.Label, item.Value)
	}
	if view.FailureSummary != "" {
		_, _ = fmt.Fprintf(&body, "\nLast actionable error:\n%s\n", view.FailureSummary)
	}
	if view.Output != "" {
		_, _ = fmt.Fprintf(&body, "\nRun output (%s):\n%s\n", view.OutputLabel, view.Output)
	}
	if view.ActionURL != "" {
		_, _ = fmt.Fprintf(&body, "\n%s: %s\n", view.ActionText, view.ActionURL)
	}

	return body.String()
}
