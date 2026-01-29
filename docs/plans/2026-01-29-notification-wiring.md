# Notification Wiring + Resend Email Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the existing notification system into the task runner so callbacks can send notifications, and implement Resend + SMTP email senders so the email channel works.

**Architecture:** The notification module already has a full channel system (Email, Slack, Discord, Telegram) with a `Notifier` service, but it's never wired into `app.Deps` or passed to `TaskRunnerDeps`. The `taskrunner.NotifierService` interface uses a minimal `Notification` (just `RawText()`) while the notification module's `Notifier` expects a richer `models.Notification`. We'll create a thin adapter to bridge these, add `MailConfig` to the config system, implement `ResendSender` and `SMTPSender` as `channels.EmailSender` implementations, and wire everything together.

**Tech Stack:** Go, Resend API (HTTP), net/smtp (SMTP), existing notification module

---

## Task 1: Add MailConfig to the config system

**Files:**
- Modify: `internal/config/services.go`
- Modify: `internal/config/config.go`
- Modify: `.env.example`

**Step 1: Add MailConfig struct to services.go**

Add at the end of `internal/config/services.go`:

```go
// MailConfig holds email sending configuration
type MailConfig struct {
	Driver      string `env:"MAIL_DRIVER" default:"resend"`
	FromAddress string `env:"MAIL_FROM_ADDRESS" default:"noreply@example.com"`
	FromName    string `env:"MAIL_FROM_NAME" default:"Launch"`
	ResendKey   string `env:"RESEND_API_KEY" default:""`
	SMTPHost    string `env:"SMTP_HOST" default:""`
	SMTPPort    int    `env:"SMTP_PORT" default:"587"`
	SMTPUser    string `env:"SMTP_USERNAME" default:""`
	SMTPPass    string `env:"SMTP_PASSWORD" default:""`
}
```

**Step 2: Add Mail field to Config and load it**

In `internal/config/config.go`, add `Mail MailConfig` to the `Config` struct and `Mail: config.Load[MailConfig]()` in `Load()`.

**Step 3: Update .env.example**

Add under the `# Email` section:

```
MAIL_FROM_ADDRESS=noreply@example.com
MAIL_FROM_NAME=Launch
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
```

**Step 4: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`

**Step 5: Commit**

```bash
git add internal/config/services.go internal/config/config.go .env.example
git commit -m "feat: add mail configuration to config system"
```

---

## Task 2: Create mail sender implementations

**Files:**
- Create: `internal/pkg/mail/resend.go`
- Create: `internal/pkg/mail/smtp.go`
- Create: `internal/pkg/mail/mail.go`

**Step 1: Create the mail package with ResendSender**

Create `internal/pkg/mail/resend.go`:

```go
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ResendSender sends emails via the Resend API.
type ResendSender struct {
	apiKey      string
	fromAddress string
	fromName    string
	httpClient  *http.Client
}

// NewResendSender creates a new Resend email sender.
func NewResendSender(apiKey, fromAddress, fromName string) *ResendSender {
	return &ResendSender{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		fromName:    fromName,
		httpClient:  &http.Client{},
	}
}

type resendRequest struct {
	From    string `json:"from"`
	To      []string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Send sends an email via the Resend API.
func (s *ResendSender) Send(ctx context.Context, to, subject, body string, isHTML bool) error {
	from := s.fromAddress
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)
	}

	req := resendRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
	}
	if isHTML {
		req.HTML = body
	} else {
		req.Text = body
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal resend request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create resend request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("resend API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
```

**Step 2: Create SMTPSender**

Create `internal/pkg/mail/smtp.go`:

```go
package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSender sends emails via SMTP.
type SMTPSender struct {
	host        string
	port        int
	username    string
	password    string
	fromAddress string
	fromName    string
}

// NewSMTPSender creates a new SMTP email sender.
func NewSMTPSender(host string, port int, username, password, fromAddress, fromName string) *SMTPSender {
	return &SMTPSender{
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		fromAddress: fromAddress,
		fromName:    fromName,
	}
}

// Send sends an email via SMTP.
func (s *SMTPSender) Send(_ context.Context, to, subject, body string, isHTML bool) error {
	from := s.fromAddress

	var msg strings.Builder
	if s.fromName != "" {
		msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", s.fromName, from))
	} else {
		msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	}
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	if isHTML {
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg.String())); err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	return nil
}
```

**Step 3: Create factory function**

Create `internal/pkg/mail/mail.go`:

```go
package mail

import (
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
)

// NewEmailSender creates an EmailSender based on the mail configuration.
// Returns nil if the driver is not configured or the required credentials are missing.
func NewEmailSender(cfg config.MailConfig) channels.EmailSender {
	switch cfg.Driver {
	case "resend":
		if cfg.ResendKey == "" {
			return nil
		}
		return NewResendSender(cfg.ResendKey, cfg.FromAddress, cfg.FromName)
	case "smtp":
		if cfg.SMTPHost == "" {
			return nil
		}
		return NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.FromAddress, cfg.FromName)
	default:
		return nil
	}
}
```

**Step 4: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`

**Step 5: Commit**

```bash
git add internal/pkg/mail/
git commit -m "feat: add Resend and SMTP email sender implementations"
```

---

## Task 3: Create taskrunner NotifierService adapter

The `taskrunner.NotifierService` interface accepts `taskrunner.Notification` (just `RawText() string`), but `services.Notifier.SendToTeam()` accepts `models.Notification` (full interface with `Type()`, `ToEmail()`, `ToSlack()`, etc.). We need a thin adapter.

**Files:**
- Create: `internal/modules/notification/adapter.go`

**Step 1: Create the adapter**

Create `internal/modules/notification/adapter.go`:

```go
package notification

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Ensure NotifierAdapter satisfies the taskrunner interface at compile time
var _ taskrunner.NotifierService = (*NotifierAdapter)(nil)

// NotifierAdapter adapts the notification module's Notifier to the
// taskrunner.NotifierService interface. It wraps minimal taskrunner.Notification
// values into full models.Notification before forwarding to the real Notifier.
type NotifierAdapter struct {
	notifier *services.Notifier
}

// NewNotifierAdapter creates a new adapter around the notification module's Notifier.
func NewNotifierAdapter(notifier *services.Notifier) *NotifierAdapter {
	return &NotifierAdapter{notifier: notifier}
}

// SendToTeam sends a notification to all connected channels for a team.
func (a *NotifierAdapter) SendToTeam(ctx context.Context, teamID string, notification taskrunner.Notification) error {
	return a.notifier.SendToTeam(ctx, teamID, &taskrunnerNotification{notification: notification})
}

// SendToChannel sends a notification to a specific channel.
func (a *NotifierAdapter) SendToChannel(ctx context.Context, channelID string, notification taskrunner.Notification) error {
	return a.notifier.SendToChannel(ctx, channelID, &taskrunnerNotification{notification: notification})
}

// taskrunnerNotification wraps a taskrunner.Notification to satisfy models.Notification.
type taskrunnerNotification struct {
	notification taskrunner.Notification
}

var _ models.Notification = (*taskrunnerNotification)(nil)

func (n *taskrunnerNotification) Type() notificationtypes.NotificationType {
	return notificationtypes.NotificationType("task")
}

func (n *taskrunnerNotification) RawText() string {
	return n.notification.RawText()
}

func (n *taskrunnerNotification) ToEmail() *channels.EmailMessage {
	return &channels.EmailMessage{
		Subject: "Launch Notification",
		Body:    n.notification.RawText(),
		IsHTML:  false,
	}
}

func (n *taskrunnerNotification) ToSlack() string {
	return n.notification.RawText()
}

func (n *taskrunnerNotification) ToDiscord() string {
	return n.notification.RawText()
}

func (n *taskrunnerNotification) ToTelegram() string {
	return n.notification.RawText()
}
```

**Step 2: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`

**Step 3: Commit**

```bash
git add internal/modules/notification/adapter.go
git commit -m "feat: add NotifierAdapter to bridge taskrunner and notification interfaces"
```

---

## Task 4: Update notification module to accept EmailSender and expose Notifier

**Files:**
- Modify: `internal/modules/notification/module.go`

**Step 1: Update NewModule to accept an optional EmailSender and expose Notifier**

The module needs to:
1. Accept an `EmailSender` parameter (can be nil if no email is configured)
2. Use `NewFactoryWithEmail` when an email sender is available
3. Expose a `Notifier()` method that returns a `*NotifierAdapter`

Update `internal/modules/notification/module.go`:

```go
package notification

import (
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/notification/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

const ModuleName = "notification"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the notification module
type Module struct {
	app.Base

	// Repository registry
	repos *repositories.Registry

	// Channel factory (needed for creating notification channels)
	channelFactory *channels.Factory

	// Cached service registry (created lazily)
	serviceRegistry *services.ServiceRegistry
}

// NewModule creates a new notification module.
// emailSender may be nil if no email driver is configured.
func NewModule(b *app.Builder, emailSender channels.EmailSender) *Module {
	deps := b.Deps()
	httpClient := channels.NewDefaultHTTPClient()

	var channelFactory *channels.Factory
	if emailSender != nil {
		channelFactory = channels.NewFactoryWithEmail(httpClient, emailSender)
	} else {
		channelFactory = channels.NewFactory(httpClient)
	}

	return &Module{
		Base:           app.NewBase(ModuleName, b),
		repos:          repositories.NewRegistry(deps.DB),
		channelFactory: channelFactory,
	}
}

// createServices creates all services needed for route handlers
func (m *Module) createServices() *services.ServiceRegistry {
	if m.serviceRegistry != nil {
		return m.serviceRegistry
	}

	deps := m.Deps()

	// Get admin webhook URL from config
	adminWebhookURL := ""
	if deps.Config != nil {
		adminWebhookURL = deps.Config.Slack.AdminWebhookURL
	}

	// Create shared service dependencies using embedded service.ModuleDeps
	svcDeps := &services.ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: deps.ServiceDeps(),
			Repos:        m.repos,
		},
		ChannelFactory:  m.channelFactory,
		AdminWebhookURL: adminWebhookURL,
	}

	// Create service registry - handles all service creation and wiring
	m.serviceRegistry = services.NewServiceRegistry(svcDeps)
	return m.serviceRegistry
}

// Notifier returns a NotifierAdapter that satisfies taskrunner.NotifierService.
// This creates the service registry if it hasn't been created yet.
func (m *Module) Notifier() *NotifierAdapter {
	svc := m.createServices()
	return NewNotifierAdapter(svc.Notifier())
}

// Repos returns the repository registry
func (m *Module) Repos() *repositories.Registry {
	return m.repos
}
```

**Step 2: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`
Expected: Compilation errors in `cmd/api/main.go` and `cmd/worker/main.go` because `NewModule` signature changed. That's fine — we'll fix them in Task 6.

**Step 3: Commit**

```bash
git add internal/modules/notification/module.go
git commit -m "feat: update notification module to accept EmailSender and expose Notifier"
```

---

## Task 5: Add Notifier to app.Deps

**Files:**
- Modify: `internal/pkg/app/builder.go`
- Modify: `internal/pkg/app/context.go`

**Step 1: Add Notifier field to Deps**

In `internal/pkg/app/builder.go`, add to the `Deps` struct:

```go
Notifier taskrunner.NotifierService
```

**Step 2: Add Notifier parameter to NewContext**

In `internal/pkg/app/context.go`, add `notifier taskrunner.NotifierService` parameter and set `Notifier: notifier` in the Deps struct.

**Step 3: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`
Expected: Compilation errors in callers of `NewContext` — that's expected, fixed in Task 6.

**Step 4: Commit**

```bash
git add internal/pkg/app/builder.go internal/pkg/app/context.go
git commit -m "feat: add Notifier field to app.Deps"
```

---

## Task 6: Wire everything in cmd/api/main.go and cmd/worker/main.go

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `cmd/worker/main.go`

**Step 1: Update cmd/api/main.go**

In `registerModules()`:

1. Before creating the notification module, create the email sender:
```go
// Create email sender based on config
emailSender := mail.NewEmailSender(a.config.Mail)
```

2. Update the notification module creation:
```go
notificationModule := notification.NewModule(builder, emailSender)
```

3. After registering all modules and calling `BootTaskCallbacks()`, get the notifier and set it on the context's Deps:
```go
// Wire notifier into shared deps for modules that create TaskRunnerDeps
ctx.Notifier = notificationModule.Notifier()
```

4. Update `NewContext` call to pass `nil` initially (notifier is set after module creation):
```go
ctx := app.NewContext(
    a.config,
    a.db,
    a.logger,
    a.queueClient,
    a.wsHub,
    a.dispatcher,
    a.membershipCache,
    nil, // Notifier - set after module creation
)
```

5. Add import for `"github.com/kkz6/launch-go/internal/pkg/mail"`

**Step 2: Update cmd/worker/main.go**

1. Create email sender and update notification module creation (if notification module is used in worker):

Since the worker doesn't currently create a notification module, we need to:
- Create an email sender
- Create the notification module
- Get the notifier adapter
- Pass it to `NewContext` and to the `TaskRecoverer`

```go
// Create email sender based on config
emailSender := mail.NewEmailSender(cfg.Mail)

// Create notification module for notifier access
notifBuilder := app.NewBuilder(app.Deps{
    DB:     db,
    Logger: appLogger,
    Config: cfg,
})
notificationModule := notification.NewModule(notifBuilder, emailSender)
notifier := notificationModule.Notifier()
```

2. Update `NewContext` call to pass the notifier.

3. Pass `notifier` to `NewTaskRecoverer` instead of `nil`.

4. Add imports for `"github.com/kkz6/launch-go/internal/pkg/mail"` and `"github.com/kkz6/launch-go/internal/modules/notification"`.

**Step 3: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`

**Step 4: Commit**

```bash
git add cmd/api/main.go cmd/worker/main.go
git commit -m "feat: wire notification system into API and worker"
```

---

## Task 7: Wire Notifier into all TaskRunnerDeps creation sites

Every place that creates `TaskRunnerDeps` needs to include the `Notifier` from `app.Deps`.

**Files:**
- Modify: `internal/modules/server/routes.go` (line 15-21)
- Modify: `internal/modules/server/jobs/deps.go` (lines 39-45, 69-75)
- Modify: `internal/modules/site/routes.go` (lines 17-22, 55-61)
- Modify: `internal/modules/site/jobs/deps.go` (lines 50-56, 84-90)
- Modify: `internal/modules/database/jobs/deps.go` (lines 41-47, 69-75)
- Modify: `internal/modules/script/jobs/deps.go` (lines 42-48, 72-78)

**Step 1: Add `Notifier: deps.Notifier` to each TaskRunnerDeps literal**

For every `TaskRunnerDeps{}` struct literal that uses `appDeps` (the `app.Deps` struct), add:
```go
Notifier: appDeps.Notifier,
```

For those that use individual variables (the test helper variants), add:
```go
Notifier: notifier,
```
...where `notifier` is a new parameter to those test helper functions. However, the test helpers (second function in each deps.go) accept individual params and are only used in tests — for those, we can pass `nil` since tests don't need real notifications.

**Step 2: Update each file**

For `server/routes.go`:
```go
taskRunnerDeps := &tasks.TaskRunnerDeps{
    DB:          deps.DB,
    Queue:       deps.Queue,
    Dispatcher:  deps.Dispatcher,
    Logger:      deps.Logger,
    Broadcaster: deps.WebSocket,
    Notifier:    deps.Notifier,
}
```

Repeat for all other `appDeps`-based sites. For the test helper functions that take individual parameters, the `Notifier` field is already zero-valued (nil) which is acceptable.

**Step 3: Verify build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`

**Step 4: Commit**

```bash
git add internal/modules/server/routes.go internal/modules/server/jobs/deps.go \
       internal/modules/site/routes.go internal/modules/site/jobs/deps.go \
       internal/modules/database/jobs/deps.go internal/modules/script/jobs/deps.go
git commit -m "feat: wire Notifier into all TaskRunnerDeps creation sites"
```

---

## Task 8: Final build, verify, and test

**Step 1: Full build**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go build ./...`

**Step 2: Run tests**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go test ./... -count=1 -short`

**Step 3: Run vet**

Run: `cd /Users/karthickk/GolandProjects/launch-go && go vet ./...`

**Step 4: Verify the warning is gone**

Start the worker and verify the `Notifier not available, cannot send notification` warning no longer appears when tasks complete.

---

## Summary of Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/config/services.go` | Modify | Add `MailConfig` struct |
| `internal/config/config.go` | Modify | Add `Mail` field, load it |
| `.env.example` | Modify | Add mail env vars |
| `internal/pkg/mail/resend.go` | Create | Resend API email sender |
| `internal/pkg/mail/smtp.go` | Create | SMTP email sender |
| `internal/pkg/mail/mail.go` | Create | Factory function |
| `internal/modules/notification/adapter.go` | Create | NotifierAdapter bridging taskrunner <-> notification |
| `internal/modules/notification/module.go` | Modify | Accept EmailSender, expose Notifier |
| `internal/pkg/app/builder.go` | Modify | Add `Notifier` to `Deps` |
| `internal/pkg/app/context.go` | Modify | Add `notifier` param to `NewContext` |
| `cmd/api/main.go` | Modify | Wire email sender + notifier |
| `cmd/worker/main.go` | Modify | Wire email sender + notifier + recovery |
| `internal/modules/server/routes.go` | Modify | Add Notifier to TaskRunnerDeps |
| `internal/modules/server/jobs/deps.go` | Modify | Add Notifier to TaskRunnerDeps |
| `internal/modules/site/routes.go` | Modify | Add Notifier to TaskRunnerDeps |
| `internal/modules/site/jobs/deps.go` | Modify | Add Notifier to TaskRunnerDeps |
| `internal/modules/database/jobs/deps.go` | Modify | Add Notifier to TaskRunnerDeps |
| `internal/modules/script/jobs/deps.go` | Modify | Add Notifier to TaskRunnerDeps |
