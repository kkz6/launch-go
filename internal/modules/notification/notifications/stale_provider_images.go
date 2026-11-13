package notifications

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// StaleProviderImageFinding is one row of the cron's output: a single
// configured (provider, os, image) tuple that could not be verified.
type StaleProviderImageFinding struct {
	Provider string
	OS       string
	Image    string
	Reason   string
}

// StaleProviderImagesNotification is sent to team owners when the daily
// image-validation cron cannot verify configured images. The point of
// the notification is to surface this in
// the dashboard / email of the people who can act on it BEFORE an actual
// server-create attempt blows up with the unhelpful 422.
//
// Why team owners and not all team members: the affected resource is a
// server_providers row, which is connected at the team level. Owners
// have the permission to swap credentials or rotate image selections.
type StaleProviderImagesNotification struct {
	*models.BaseNotification
	Findings []StaleProviderImageFinding
}

// NewStaleProviderImagesNotification creates a notification summarising
// the cron's findings. The raw-text representation goes into the
// in-app notifications panel; the channel-specific renderers below
// expand into email/slack/discord/telegram.
func NewStaleProviderImagesNotification(findings []StaleProviderImageFinding) *StaleProviderImagesNotification {
	rawText := fmt.Sprintf("Could not verify %d configured cloud-provider image(s). Review the reported errors before provisioning new servers.", len(findings))
	return &StaleProviderImagesNotification{
		BaseNotification: models.NewBaseNotification(notificationtypes.NotificationTypeStaleProviderImages, rawText),
		Findings:         findings,
	}
}

// findingsTable renders one bullet per finding in markdown form. Used
// by ToEmail's WithPanel and by the plain-text fallback.
func (n *StaleProviderImagesNotification) findingsTable() string {
	var b strings.Builder
	for _, f := range n.Findings {
		b.WriteString(fmt.Sprintf("- **%s** · `%s` — image `%s` — %s\n", f.Provider, f.OS, f.Image, f.Reason))
	}
	return strings.TrimRight(b.String(), "\n")
}

// ToEmail returns the email message content
func (n *StaleProviderImagesNotification) ToEmail() *channels.EmailMessage {
	builder := templates.NewEmail().
		WithContext("lctl / provider").
		WithState("VALIDATION FAILED", "warning").
		WithGreeting("Cloud-provider image configuration needs attention").
		WithIntro(fmt.Sprintf("Launch's daily check could not verify **%d** configured cloud-provider image(s).", len(n.Findings))).
		WithPanel(n.findingsTable()).
		WithIntro("Review the error for each image. Check the cloud connection or contact the platform team to update an unavailable image.")

	html, err := builder.Build()
	if err != nil {
		return n.plainTextEmail()
	}
	return &channels.EmailMessage{
		Subject: "Cloud-provider image configuration needs attention",
		Body:    html,
		IsHTML:  true,
	}
}

func (n *StaleProviderImagesNotification) plainTextEmail() *channels.EmailMessage {
	body := fmt.Sprintf(`Launch's daily check could not verify %d configured cloud-provider image(s).

Affected images:
%s

Review the errors, check the cloud connection, or contact the platform team to update an unavailable image.`,
		len(n.Findings),
		n.findingsTable(),
	)
	return &channels.EmailMessage{
		Subject: "Cloud-provider image configuration needs attention",
		Body:    body,
		IsHTML:  false,
	}
}

// ToSlack returns the Slack message content
func (n *StaleProviderImagesNotification) ToSlack() string {
	return fmt.Sprintf(`*⚠️ Cloud-provider image configuration needs attention*

Could not verify %d configured image(s). Review the errors before provisioning new servers.

%s`, len(n.Findings), n.findingsTable())
}

// ToDiscord mirrors Slack's markdown rendering.
func (n *StaleProviderImagesNotification) ToDiscord() string {
	return n.ToSlack()
}

// ToTelegram returns the Telegram message content (HTML-style)
func (n *StaleProviderImagesNotification) ToTelegram() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>⚠️ Cloud-provider image configuration needs attention</b>\n\nCould not verify %d configured image(s). Review the errors before provisioning new servers.\n\n", len(n.Findings)))
	for _, f := range n.Findings {
		b.WriteString(fmt.Sprintf("• <b>%s</b> · <code>%s</code> — image <code>%s</code> — %s\n", f.Provider, f.OS, f.Image, f.Reason))
	}
	return b.String()
}
