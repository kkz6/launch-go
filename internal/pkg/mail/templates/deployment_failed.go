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

var deploymentFailedContentTemplate = template.Must(template.New("deployment-failed-content").Parse(`
<table class="incident-hero" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; table-layout:fixed;">
<tr>
<td class="incident-pad" style="padding:40px 36px 30px;">
<p style="margin:0 0 10px; color:#b91c1c; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:12px; font-weight:700; letter-spacing:0.08em; line-height:18px; mso-line-height-rule:exactly;">&#10007; {{ .StatusLabel }}</p>
<h1 class="incident-title" style="margin:0; color:#18181b; font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif; font-size:32px; font-weight:700; letter-spacing:-0.04em; line-height:40px; mso-line-height-rule:exactly;">{{ .StatusHeading }}</h1>
<p class="incident-route" style="margin:12px 0 0; color:#18181b; font-size:17px; font-weight:600; line-height:25px; mso-line-height-rule:exactly; overflow-wrap:anywhere;">{{ .SiteAddress }}{{ if .HasServer }} <span style="color:#a1a1aa; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-weight:400;">&#8594;</span> {{ .ServerName }}{{ end }}</p>
<p style="margin:8px 0 0; color:#71717a; font-size:14px; line-height:22px; mso-line-height-rule:exactly;">The deployment stopped before it completed. Start with the last actionable error.</p>
</td>
</tr>
</table>

{{ if .FailureSummary }}
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; table-layout:fixed;">
<tr>
<td class="incident-pad" style="padding:0 36px 30px;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; border-top:2px solid #ef4444;">
<tr>
<td style="padding:14px 0 0;">
<p style="margin:0 0 5px; color:#71717a; font-size:12px; font-weight:700; line-height:18px; mso-line-height-rule:exactly;">Last actionable error</p>
<p style="margin:0; color:#991b1b; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:13px; font-weight:600; line-height:20px; mso-line-height-rule:exactly; overflow-wrap:anywhere; word-break:break-word;">{{ .FailureSummary }}</p>
</td>
</tr>
</table>
</td>
</tr>
</table>
{{ end }}

<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; table-layout:fixed;">
<tr>
<td class="incident-pad" style="padding:0 36px 34px;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; border-top:1px solid #18181b;">
<tr>
<td style="padding:18px 0 0;">
<p style="margin:0 0 10px; color:#27272a; font-size:13px; font-weight:700; line-height:19px; mso-line-height-rule:exactly;">Run context</p>
{{ if .HasCommit }}
<p style="margin:0; color:#27272a; font-size:15px; font-weight:600; line-height:23px; mso-line-height-rule:exactly; overflow-wrap:anywhere;">
{{ if .ShortGitHash }}<span style="color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:12px; font-weight:500;">{{ .ShortGitHash }}</span>{{ end }}{{ if and .ShortGitHash .CommitMessage }} <span style="color:#d4d4d8;">/</span> {{ end }}{{ .CommitMessage }}
</p>
{{ end }}
<p style="margin:{{ if .HasCommit }}9px{{ else }}0{{ end }} 0 0; color:#71717a; font-size:12px; line-height:20px; mso-line-height-rule:exactly;">
{{ range $index, $item := .RunMeta }}{{ if $index }} <span style="color:#d4d4d8;">&middot;</span> {{ end }}<span>{{ $item.Label }}</span> <strong style="color:#3f3f46; font-weight:600;">{{ $item.Value }}</strong>{{ end }}
</p>
{{ if .ActionURL }}
<table class="incident-action" width="100%" border="0" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#18181b" style="width:100%; margin:22px 0 0; background-color:#18181b;">
<tr>
<td align="left" bgcolor="#18181b" style="background-color:#18181b; mso-padding-alt:13px 16px;">
<a href="{{ .ActionURL }}" target="_blank" rel="noopener" style="display:block; border:13px solid #18181b; background-color:#18181b; color:#ffffff; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:13px; font-weight:600; line-height:18px; text-decoration:none; -webkit-text-size-adjust:none;">{{ .ActionText }} <span style="color:#a1a1aa;">&#8594;</span></a>
</td>
</tr>
</table>
{{ end }}
</td>
</tr>
</table>
</td>
</tr>
</table>

{{ if .OutputLines }}
<table class="trace-shell" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#fafafa" style="width:100%; table-layout:fixed; border-top:1px solid #18181b; background-color:#fafafa;">
<tr>
<td class="trace-heading-pad" bgcolor="#ffffff" style="padding:18px 36px 12px; background-color:#ffffff;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%;">
<tr>
<td align="left"><h2 style="margin:0; color:#27272a; font-size:13px; font-weight:700; line-height:19px; mso-line-height-rule:exactly;">Run output</h2></td>
<td align="right"><span class="output-label" style="color:#a1a1aa; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; line-height:16px; mso-line-height-rule:exactly;">{{ .OutputLabel }}</span></td>
</tr>
</table>
</td>
</tr>
<tr>
<td bgcolor="#fafafa" style="padding:0 0 18px; background-color:#fafafa;">
<table class="deployment-trace" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#fafafa" style="width:100%; table-layout:fixed; background-color:#fafafa;">
{{ range .OutputLines }}
<tr class="trace-line{{ if .IsFailure }} trace-error{{ else if .IsCleanup }} trace-cleanup{{ end }}">
<td width="24" valign="top" align="center" {{ if .IsFailure }}bgcolor="#fef2f2"{{ else }}bgcolor="#f4f4f5"{{ end }} style="width:24px; padding:3px 0; {{ if .IsFailure }}background-color:#fef2f2; color:#b91c1c;{{ else }}background-color:#f4f4f5; color:#d4d4d8;{{ end }} font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:11px; font-weight:700; line-height:18px; mso-line-height-rule:exactly;">{{ if .Marker }}{{ .Marker }}{{ else }}&nbsp;{{ end }}</td>
<td width="34" valign="top" align="right" {{ if .IsFailure }}bgcolor="#fef2f2"{{ else }}bgcolor="#fafafa"{{ end }} style="width:34px; padding:3px 8px 3px 0; {{ if .IsFailure }}background-color:#fef2f2; color:#b91c1c;{{ else }}background-color:#fafafa; color:#a1a1aa;{{ end }} font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; line-height:18px; mso-line-height-rule:exactly;">{{ if .Number }}{{ .Number }}{{ else }}&nbsp;{{ end }}</td>
<td class="trace-code" valign="top" {{ if .IsFailure }}bgcolor="#fef2f2"{{ else }}bgcolor="#fafafa"{{ end }} style="padding:3px 24px 3px 0; {{ if .IsFailure }}background-color:#fef2f2; color:#991b1b; font-weight:600;{{ else if .IsCleanup }}background-color:#fafafa; color:#a1a1aa;{{ else }}background-color:#fafafa; color:#3f3f46;{{ end }} font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:11px; line-height:18px; mso-line-height-rule:exactly; overflow-wrap:anywhere; word-break:break-all; white-space:pre-wrap;">{{ .Text }}</td>
</tr>
{{ end }}
</table>
</td>
</tr>
</table>
{{ end }}
`))

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
	if err := deploymentFailedContentTemplate.Execute(&content, view); err != nil {
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
