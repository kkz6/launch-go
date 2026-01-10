package taskrunner

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// TemplateEngine handles script template rendering
type TemplateEngine struct {
	templates *template.Template
}

// NewTemplateEngine creates a new template engine with all embedded templates
func NewTemplateEngine() (*TemplateEngine, error) {
	// Custom template functions
	funcMap := template.FuncMap{
		"join":     strings.Join,
		"contains": strings.Contains,
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"trim":     strings.TrimSpace,
		"replace":  strings.ReplaceAll,
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},
		"quote": func(s string) string {
			return fmt.Sprintf("%q", s)
		},
		"escape": func(s string) string {
			// Escape single quotes for bash
			return strings.ReplaceAll(s, "'", "'\"'\"'")
		},
	}

	tmpl := template.New("").Funcs(funcMap)

	// Parse all template files
	patterns := []string{
		"templates/common/*.sh",
		"templates/server/*.sh",
		"templates/site/*.sh",
	}

	for _, pattern := range patterns {
		files, err := templateFS.ReadDir(strings.TrimSuffix(pattern, "/*.sh"))
		if err != nil {
			continue // Directory might not exist
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			path := strings.TrimSuffix(pattern, "*.sh") + file.Name()
			content, err := templateFS.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read template %s: %w", path, err)
			}

			_, err = tmpl.Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse template %s: %w", path, err)
			}
		}
	}

	return &TemplateEngine{templates: tmpl}, nil
}

// Render renders a template with the given data
func (e *TemplateEngine) Render(name string, data interface{}) (string, error) {
	var buf bytes.Buffer

	err := e.templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", fmt.Errorf("failed to render template %s: %w", name, err)
	}

	return buf.String(), nil
}

// RenderString renders a template string (not from embedded files)
func (e *TemplateEngine) RenderString(templateStr string, data interface{}) (string, error) {
	tmpl, err := e.templates.Clone()
	if err != nil {
		return "", err
	}

	tmpl, err = tmpl.Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template string: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

// Available template names
const (
	// Common templates
	TemplateHelpers         = "helpers"
	TemplateCallbackWrapper = "callback_wrapper"

	// Server templates
	TemplateServerProvision         = "server_provision"
	TemplateServerInstallPHP        = "server_install_php"
	TemplateServerConfigureFirewall = "server_configure_firewall"

	// Site templates
	TemplateSiteDeploy     = "site_deploy"
	TemplateSiteRollback   = "site_rollback"
	TemplateSiteInstallSSL = "site_install_ssl"
)

// Data structures for templates

// ServerProvisionData contains data for server provisioning
type ServerProvisionData struct {
	ServerName           string
	Provider             string
	Timezone             string
	SwapSize             string
	SSHPort              int
	DisablePasswordAuth  bool
	CreateUser           bool
	Username             string
	PublicKey            string
	PHPVersion           string
	NodeVersion          string
	InstallCaddy         bool
	DatabaseType         string
	DatabaseRootPassword string
	InstallRedis         bool
	InstallSupervisor    bool
}

// InstallPHPData contains data for PHP installation
type InstallPHPData struct {
	PHPVersion              string
	UploadMaxFilesize       string
	PostMaxSize             string
	MemoryLimit             string
	MaxExecutionTime        int
	OpcacheEnabled          bool
	OpcacheMemory           int
	OpcacheValidateTimestamps bool
	SetAsDefault            bool
}

// DeployData contains data for site deployment
type DeployData struct {
	SiteName        string
	Domain          string
	SitePath        string
	Release         string
	RepoURL         string
	Branch          string
	DeployKey       bool
	DeployKeyPath   string
	SharedDirs      []string
	HasComposer     bool
	HasNpm          bool
	UseNpmCi        bool
	BuildAssets     bool
	BuildCommand    string
	IsLaravel       bool
	RunMigrations   bool
	RunSeeders      bool
	CustomScript    string
	PHPVersion      string
	RestartQueue    bool
	RestartScheduler bool
	UseSupervisor   bool
	QueueWorkerName string
	ReleasesToKeep  int
	HealthCheckURL  string
}

// RollbackData contains data for rollback
type RollbackData struct {
	SiteName          string
	SitePath          string
	ReleaseID         string
	ReleasePath       string
	IsLaravel         bool
	PHPVersion        string
	RestartQueue      bool
	UseSupervisor     bool
	QueueWorkerName   string
	PreRollbackScript string
	PostRollbackScript string
	HealthCheckURL    string
}

// FirewallRule represents a firewall rule
type FirewallRule struct {
	Name     string
	Port     int
	Protocol string
	FromIP   string
}

// ConfigureFirewallData contains data for firewall configuration
type ConfigureFirewallData struct {
	ServerName string
	SSHPort    int
	AllowHTTP  bool
	AllowHTTPS bool
	Rules      []FirewallRule
}

// InstallSSLData contains data for SSL installation
type InstallSSLData struct {
	Domain         string
	Aliases        []string
	Email          string
	UseCaddy       bool
	PHPVersion     string
	HealthCheckURL string
}

// CallbackWrapperData contains data for the callback wrapper
type CallbackWrapperData struct {
	Script      string
	Timeout     int
	FinishedURL string
	FailedURL   string
	TimeoutURL  string
}
