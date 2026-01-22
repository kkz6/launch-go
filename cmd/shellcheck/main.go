package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

func main() {
	outputDir := flag.String("output", "storage/shellcheck", "Output directory for rendered scripts")
	dryRun := flag.Bool("dry-run", false, "Show what would be rendered without writing files")
	flag.Parse()

	if err := run(*outputDir, *dryRun); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(outputDir string, dryRun bool) error {
	// Register all module templates
	templates.MustRegisterAll()

	if !dryRun {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		// Clean directory
		entries, _ := os.ReadDir(outputDir)
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(outputDir, entry.Name()))
		}
	}

	fmt.Println("Rendering shell scripts from task templates...")
	fmt.Println()

	tasks := getTasksToRender()
	successCount := 0
	errorCount := 0

	for name, data := range tasks {
		script, err := templates.Render(data.module, data.template, data.data)

		filename := slugify(name) + ".sh"

		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", name, err)
			errorCount++
			continue
		}

		if dryRun {
			fmt.Printf("  ✓ %s -> %s\n", name, filename)
			successCount++
			continue
		}

		outputPath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(outputPath, []byte(script), 0755); err != nil {
			fmt.Printf("  ✗ %s: %v\n", name, err)
			errorCount++
			continue
		}

		fmt.Printf("  ✓ %s\n", name)
		successCount++
	}

	fmt.Println()
	fmt.Printf("Rendered %d scripts successfully.\n", successCount)

	if errorCount > 0 {
		fmt.Printf("Failed to render %d scripts.\n", errorCount)
		return fmt.Errorf("some scripts failed to render")
	}

	if !dryRun {
		fmt.Printf("Scripts saved to: %s\n", outputDir)
	}

	return nil
}

type taskData struct {
	module   string
	template string
	data     interface{}
}

// Data structures for template rendering

type InstallPHPData struct {
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

type FirewallRule struct {
	Name     string
	Port     int
	Protocol string
	FromIP   string
}

type ConfigureFirewallData struct {
	ServerName string
	SSHPort    int
	AllowHTTP  bool
	AllowHTTPS bool
	Rules      []FirewallRule
}

type RollbackData struct {
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

func getTasksToRender() map[string]taskData {
	// Sample data for rendering templates
	// Template names match the actual file paths under each module's templates directory
	return map[string]taskData{
		"server-install-php": {
			module:   "server",
			template: "software/install_php.sh",
			data: InstallPHPData{
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
			data: ConfigureFirewallData{
				ServerName: "test-server",
				SSHPort:    22,
				AllowHTTP:  true,
				AllowHTTPS: true,
				Rules: []FirewallRule{
					{Name: "MySQL", Port: 3306, Protocol: "tcp", FromIP: "10.0.0.0/8"},
					{Name: "Redis", Port: 6379, Protocol: "tcp", FromIP: "10.0.0.0/8"},
				},
			},
		},
		"site-rollback": {
			module:   "site",
			template: "rollback_deployment.sh",
			data: RollbackData{
				SiteName:           "example-site",
				SitePath:           "/home/launch/example.com",
				ReleaseID:          "20240114120000",
				ReleasePath:        "/home/launch/example.com/releases/20240114120000",
				IsLaravel:          true,
				PHPVersion:         "8.3",
				RestartQueue:       true,
				UseSupervisor:      true,
				QueueWorkerName:    "example-site-worker",
				PreRollbackScript:  "",
				PostRollbackScript: "",
				HealthCheckURL:     "https://example.com/health",
			},
		},
	}
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
