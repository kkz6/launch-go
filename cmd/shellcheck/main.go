package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kkz6/launch-go/internal/taskrunner"
)

func main() {
	outputDir := flag.String("output", "storage/shellcheck", "Output directory for rendered scripts")
	dryRun := flag.Bool("dry-run", false, "Show what would be rendered without writing files")
	flag.Parse()

	if err := run(*outputDir, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(outputDir string, dryRun bool) error {
	engine, err := taskrunner.NewTemplateEngine()
	if err != nil {
		return fmt.Errorf("failed to create template engine: %w", err)
	}

	if !dryRun {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		// Clean directory
		entries, _ := os.ReadDir(outputDir)
		for _, entry := range entries {
			os.RemoveAll(filepath.Join(outputDir, entry.Name()))
		}
	}

	fmt.Println("Rendering shell scripts from task templates...")
	fmt.Println()

	tasks := getTasksToRender()
	successCount := 0
	errorCount := 0

	for name, data := range tasks {
		templateName := data.template
		script, err := engine.Render(templateName, data.data)

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
	template string
	data     interface{}
}

func getTasksToRender() map[string]taskData {
	// Sample data for rendering templates
	return map[string]taskData{
		"server-provision": {
			template: taskrunner.TemplateServerProvision,
			data: taskrunner.ServerProvisionData{
				ServerName:           "test-server",
				Provider:             "digitalocean",
				Timezone:             "UTC",
				SwapSize:             "1G",
				SSHPort:              22,
				DisablePasswordAuth:  true,
				CreateUser:           true,
				Username:             "launch",
				PublicKey:            "ssh-ed25519 AAAA... test@example.com",
				PHPVersion:           "8.3",
				NodeVersion:          "20",
				InstallCaddy:         true,
				DatabaseType:         "mysql",
				DatabaseRootPassword: "secret-password",
				InstallRedis:         true,
				InstallSupervisor:    true,
			},
		},
		"server-install-php": {
			template: taskrunner.TemplateServerInstallPHP,
			data: taskrunner.InstallPHPData{
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
			template: taskrunner.TemplateServerConfigureFirewall,
			data: taskrunner.ConfigureFirewallData{
				ServerName: "test-server",
				SSHPort:    22,
				AllowHTTP:  true,
				AllowHTTPS: true,
				Rules: []taskrunner.FirewallRule{
					{Name: "MySQL", Port: 3306, Protocol: "tcp", FromIP: "10.0.0.0/8"},
					{Name: "Redis", Port: 6379, Protocol: "tcp", FromIP: "10.0.0.0/8"},
				},
			},
		},
		"site-deploy": {
			template: taskrunner.TemplateSiteDeploy,
			data: taskrunner.DeployData{
				SiteName:         "example-site",
				Domain:           "example.com",
				SitePath:         "/home/launch/example.com",
				Release:          "20240115120000",
				RepoURL:          "git@github.com:example/repo.git",
				Branch:           "main",
				DeployKey:        true,
				DeployKeyPath:    "/home/launch/.ssh/deploy_key",
				SharedDirs:       []string{"storage", "bootstrap/cache"},
				HasComposer:      true,
				HasNpm:           true,
				UseNpmCi:         true,
				BuildAssets:      true,
				BuildCommand:     "npm run build",
				IsLaravel:        true,
				RunMigrations:    true,
				RunSeeders:       false,
				CustomScript:     "",
				PHPVersion:       "8.3",
				RestartQueue:     true,
				RestartScheduler: true,
				UseSupervisor:    true,
				QueueWorkerName:  "example-site-worker",
				ReleasesToKeep:   5,
				HealthCheckURL:   "https://example.com/health",
			},
		},
		"site-rollback": {
			template: taskrunner.TemplateSiteRollback,
			data: taskrunner.RollbackData{
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
		"site-install-ssl": {
			template: taskrunner.TemplateSiteInstallSSL,
			data: taskrunner.InstallSSLData{
				Domain:         "example.com",
				Aliases:        []string{"www.example.com"},
				Email:          "admin@example.com",
				UseCaddy:       true,
				PHPVersion:     "8.3",
				HealthCheckURL: "https://example.com",
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
