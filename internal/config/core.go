package config

import pkgconfig "github.com/kkz6/launch-go/internal/pkg/config"

// CoreConfig holds core application configuration
type CoreConfig struct {
	LogsDisk             string   `env:"SERVER_LOGS_DISK" default:"server-logs"`
	KeyPairsDisk         string   `env:"KEY_PAIRS_DISK" default:"key-pairs"`
	SubscriptionsEnabled bool     `env:"SUBSCRIPTIONS_ENABLED" default:"true"`
	WebhookURL           string   `env:"WEBHOOK_URL" default:""`
	SSHProxies           []string `env:"SSH_PROXIES" default:""`
	PrintShellCommands   bool     `env:"PRINT_SHELL_COMMANDS" default:"false"`
	FormatServerContent  bool     `env:"FORMAT_SERVER_CONTENT" default:"false"`
	TailOutputDefault    int      `env:"TAIL_OUTPUT_DEFAULT" default:"500"`
	AgentConfigPath      string   `env:"LAUNCH_AGENT_CONFIG_PATH" default:"/etc/launch-agent/launch.yml"`
	ServerDefaults       ServerDefaultsConfig
}

// ServerDefaultsConfig holds default values for server provisioning
type ServerDefaultsConfig struct {
	WorkingDirectory string `env:"SERVER_WORKING_DIR" default:".launch"`
	Username         string `env:"SSH_USER" default:"launcher"`
	SSHPort          int    `env:"SSH_PORT" default:"22"`
	SSHComment       string `env:"SSH_COMMENT" default:"launch@gigcodes.com"`
	DatabaseName     string `env:"DEFAULT_DATABASE_NAME" default:"launch"`
}

// RestrictedIPAddresses returns the list of restricted IP addresses
func (c CoreConfig) RestrictedIPAddresses() []string {
	return []string{"127.0.0.1", "localhost", "0.0.0.0"}
}

// AvailableDatabases returns all available database options
func (c CoreConfig) AvailableDatabases() []string {
	return []string{
		"none",
		"mysql57",
		"mysql80",
		"mariadb103",
		"mariadb104",
		"postgresql12",
		"postgresql13",
		"postgresql14",
		"postgresql15",
		"postgresql16",
	}
}

// DatabaseTypes returns available database types
func (c CoreConfig) DatabaseTypes() []string {
	return []string{
		"mysql",
		"postgresql",
	}
}

// Default core configuration instance (initialized on first use)
var coreConfig *CoreConfig

// GetCoreConfig returns the core configuration singleton
func GetCoreConfig() *CoreConfig {
	if coreConfig == nil {
		defaults := pkgconfig.Load[ServerDefaultsConfig]()
		coreConfig = &CoreConfig{
			LogsDisk:             "server-logs",
			KeyPairsDisk:         "key-pairs",
			SubscriptionsEnabled: true,
			PrintShellCommands:   false,
			FormatServerContent:  false,
			TailOutputDefault:    500,
			AgentConfigPath:      "/etc/launch-agent/launch.yml",
			ServerDefaults:       defaults,
		}
	}
	return coreConfig
}

// ServerDefaults returns the server defaults configuration
func ServerDefaults() ServerDefaultsConfig {
	return GetCoreConfig().ServerDefaults
}
