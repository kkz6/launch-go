// scripts:render — render SSH task templates to disk so ShellCheck (or a human)
// can lint them. Faithful port of the former cmd/shellcheck entrypoint.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/console"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

type scriptsRenderCommand struct{}

func (scriptsRenderCommand) Signature() string { return "scripts:render" }
func (scriptsRenderCommand) Description() string {
	return "Render SSH task templates to disk for ShellCheck validation"
}
func (scriptsRenderCommand) Extend() console.Extend {
	return console.Extend{
		Category: "scripts",
		Flags: []console.Flag{
			console.StringFlag{Name: "output", Value: "storage/shellcheck", Usage: "Output directory for rendered scripts"},
			console.BoolFlag{Name: "dry-run", Usage: "Show what would be rendered without writing files"},
		},
	}
}

func (scriptsRenderCommand) Handle(ctx console.Context) error {
	outputDir := ctx.Option("output")
	dryRun := ctx.OptionBool("dry-run")

	// Register all module templates.
	templates.MustRegisterAll()

	if !dryRun {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			ctx.Error(fmt.Sprintf("failed to create output directory: %v", err))
			return err
		}
		entries, _ := os.ReadDir(outputDir)
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(outputDir, entry.Name()))
		}
	}

	ctx.Info("Rendering shell scripts from task templates...")
	ctx.NewLine()

	successCount, errorCount := 0, 0
	for name, data := range scriptsTasksToRender() {
		script, err := templates.Render(data.module, data.template, data.data)
		filename := scriptsSlugify(name) + ".sh"

		if err != nil {
			ctx.Error(fmt.Sprintf("  ✗ %s: %v", name, err))
			errorCount++
			continue
		}

		if dryRun {
			ctx.Line(fmt.Sprintf("  ✓ %s -> %s", name, filename))
			successCount++
			continue
		}

		if err := os.WriteFile(filepath.Join(outputDir, filename), []byte(script), 0o755); err != nil {
			ctx.Error(fmt.Sprintf("  ✗ %s: %v", name, err))
			errorCount++
			continue
		}

		ctx.Line(fmt.Sprintf("  ✓ %s", name))
		successCount++
	}

	ctx.NewLine()
	ctx.Success(fmt.Sprintf("Rendered %d scripts successfully.", successCount))

	if errorCount > 0 {
		ctx.Error(fmt.Sprintf("Failed to render %d scripts.", errorCount))
		return fmt.Errorf("some scripts failed to render")
	}

	if !dryRun {
		ctx.Comment("Scripts saved to: " + outputDir)
	}
	return nil
}

type scriptsTaskData struct {
	module   string
	template string
	data     any
}

type scriptsInstallPHPData struct {
	PHPVersion                string
	UploadMaxFilesize         string
	PostMaxSize               string
	MemoryLimit               string
	MaxExecutionTime          int
	OpcacheEnabled            bool
	OpcacheMemory             int
	OpcacheValidateTimestamps bool
	SetAsDefault              bool
}

type scriptsFirewallRule struct {
	Name     string
	Port     int
	Protocol string
	FromIP   string
}

type scriptsConfigureFirewallData struct {
	ServerName string
	SSHPort    int
	AllowHTTP  bool
	AllowHTTPS bool
	Rules      []scriptsFirewallRule
}

type scriptsRollbackData struct {
	SiteName           string
	SitePath           string
	ReleaseID          string
	ReleasePath        string
	IsLaravel          bool
	PHPVersion         string
	RestartQueue       bool
	UseSupervisor      bool
	QueueWorkerName    string
	PreRollbackScript  string
	PostRollbackScript string
	HealthCheckURL     string
}

func scriptsTasksToRender() map[string]scriptsTaskData {
	return map[string]scriptsTaskData{
		"server-install-php": {
			module:   "server",
			template: "software/install_php.sh",
			data: scriptsInstallPHPData{
				PHPVersion:                "8.3",
				UploadMaxFilesize:         "100M",
				PostMaxSize:               "100M",
				MemoryLimit:               "512M",
				MaxExecutionTime:          300,
				OpcacheEnabled:            true,
				OpcacheMemory:             256,
				OpcacheValidateTimestamps: false,
				SetAsDefault:              true,
			},
		},
		"server-configure-firewall": {
			module:   "server",
			template: "provision/configure_firewall.sh",
			data: scriptsConfigureFirewallData{
				ServerName: "test-server",
				SSHPort:    22,
				AllowHTTP:  true,
				AllowHTTPS: true,
				Rules: []scriptsFirewallRule{
					{Name: "MySQL", Port: 3306, Protocol: "tcp", FromIP: "10.0.0.0/8"},
					{Name: "Redis", Port: 6379, Protocol: "tcp", FromIP: "10.0.0.0/8"},
				},
			},
		},
		"site-rollback": {
			module:   "site",
			template: "rollback_deployment.sh",
			data: scriptsRollbackData{
				SiteName:        "example-site",
				SitePath:        "/home/launch/example.com",
				ReleaseID:       "20240114120000",
				ReleasePath:     "/home/launch/example.com/releases/20240114120000",
				IsLaravel:       true,
				PHPVersion:      "8.3",
				RestartQueue:    true,
				UseSupervisor:   true,
				QueueWorkerName: "example-site-worker",
				HealthCheckURL:  "https://example.com/health",
			},
		},
	}
}

func scriptsSlugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
