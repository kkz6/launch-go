package server

import (
	"testing"
)

func TestServerStatus_String(t *testing.T) {
	tests := []struct {
		status   ServerStatus
		expected string
	}{
		{ServerStatusNew, "new"},
		{ServerStatusStarting, "starting"},
		{ServerStatusProvisioning, "provisioning"},
		{ServerStatusRunning, "running"},
		{ServerStatusPaused, "paused"},
		{ServerStatusStopped, "stopped"},
		{ServerStatusDeleting, "deleting"},
		{ServerStatusArchived, "archived"},
		{ServerStatusUnknown, "unknown"},
		{ServerStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("ServerStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerStatus_Label(t *testing.T) {
	tests := []struct {
		status   ServerStatus
		expected string
	}{
		{ServerStatusNew, "Connecting"},
		{ServerStatusStarting, "Starting"},
		{ServerStatusProvisioning, "Provisioning"},
		{ServerStatusRunning, "Running"},
		{ServerStatusPaused, "Paused"},
		{ServerStatusStopped, "Stopped"},
		{ServerStatusDeleting, "Deleting"},
		{ServerStatusArchived, "Archived"},
		{ServerStatusUnknown, "Unknown"},
		{ServerStatusFailed, "Failed"},
		{ServerStatus("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Label(); got != tt.expected {
				t.Errorf("ServerStatus.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerStatus_IsValid(t *testing.T) {
	tests := []struct {
		status   ServerStatus
		expected bool
	}{
		{ServerStatusNew, true},
		{ServerStatusStarting, true},
		{ServerStatusProvisioning, true},
		{ServerStatusRunning, true},
		{ServerStatusPaused, true},
		{ServerStatusStopped, true},
		{ServerStatusDeleting, true},
		{ServerStatusArchived, true},
		{ServerStatusUnknown, true},
		{ServerStatusFailed, true},
		{ServerStatus("invalid"), false},
		{ServerStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.expected {
				t.Errorf("ServerStatus.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   ServerStatus
		expected bool
	}{
		{ServerStatusRunning, true},
		{ServerStatusProvisioning, true},
		{ServerStatusStarting, true},
		{ServerStatusNew, false},
		{ServerStatusPaused, false},
		{ServerStatusStopped, false},
		{ServerStatusArchived, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsActive(); got != tt.expected {
				t.Errorf("ServerStatus.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   ServerStatus
		expected bool
	}{
		{ServerStatusFailed, true},
		{ServerStatusArchived, true},
		{ServerStatusDeleting, true},
		{ServerStatusRunning, false},
		{ServerStatusNew, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.expected {
				t.Errorf("ServerStatus.IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerStatus_Scan(t *testing.T) {
	var ss ServerStatus

	if err := ss.Scan("running"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ss != ServerStatusRunning {
		t.Errorf("Expected ServerStatusRunning, got %v", ss)
	}

	if err := ss.Scan([]byte("provisioning")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ss != ServerStatusProvisioning {
		t.Errorf("Expected ServerStatusProvisioning, got %v", ss)
	}

	if err := ss.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}
	if ss != ServerStatusUnknown {
		t.Errorf("Expected ServerStatusUnknown for nil, got %v", ss)
	}

	if err := ss.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestServerStatus_Value(t *testing.T) {
	ss := ServerStatusRunning
	v, err := ss.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "running" {
		t.Errorf("Expected 'running', got %v", v)
	}
}

func TestParseServerStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected ServerStatus
		hasError bool
	}{
		{"running", ServerStatusRunning, false},
		{"new", ServerStatusNew, false},
		{"failed", ServerStatusFailed, false},
		{"invalid", ServerStatusUnknown, true},
		{"", ServerStatusUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseServerStatus(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseServerStatus() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseServerStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllServerStatuses(t *testing.T) {
	statuses := AllServerStatuses()
	if len(statuses) != 10 {
		t.Errorf("Expected 10 server statuses, got %d", len(statuses))
	}
}

func TestServerType_String(t *testing.T) {
	tests := []struct {
		serverType ServerType
		expected   string
	}{
		{ServerTypePhp, "php"},
		{ServerTypeDatabase, "database"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.serverType.String(); got != tt.expected {
				t.Errorf("ServerType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerType_Label(t *testing.T) {
	tests := []struct {
		serverType ServerType
		expected   string
	}{
		{ServerTypePhp, "PHP Application Server"},
		{ServerTypeDatabase, "Database Server"},
		{ServerType("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.serverType), func(t *testing.T) {
			if got := tt.serverType.Label(); got != tt.expected {
				t.Errorf("ServerType.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerType_IsValid(t *testing.T) {
	tests := []struct {
		serverType ServerType
		expected   bool
	}{
		{ServerTypePhp, true},
		{ServerTypeDatabase, true},
		{ServerType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.serverType), func(t *testing.T) {
			if got := tt.serverType.IsValid(); got != tt.expected {
				t.Errorf("ServerType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerType_GetFeatures(t *testing.T) {
	phpFeatures := ServerTypePhp.GetFeatures()
	if len(phpFeatures) == 0 {
		t.Error("Expected PHP server to have features")
	}

	hasPhpManagement := false
	for _, f := range phpFeatures {
		if f == ServerFeaturePhpManagement {
			hasPhpManagement = true
			break
		}
	}
	if !hasPhpManagement {
		t.Error("Expected PHP server to have PHP management feature")
	}

	dbFeatures := ServerTypeDatabase.GetFeatures()
	if len(dbFeatures) == 0 {
		t.Error("Expected Database server to have features")
	}
}

func TestServerType_HasFeature(t *testing.T) {
	if !ServerTypePhp.HasFeature(ServerFeaturePhpManagement) {
		t.Error("Expected PHP server to have PHP management feature")
	}
	if !ServerTypePhp.HasFeature(ServerFeatureSites) {
		t.Error("Expected PHP server to have sites feature")
	}
	if ServerTypeDatabase.HasFeature(ServerFeaturePhpManagement) {
		t.Error("Expected Database server to not have PHP management feature")
	}
}

func TestServerType_GetProcessManager(t *testing.T) {
	if pm := ServerTypePhp.GetProcessManager(); pm != ProcessManagerSupervisor {
		t.Errorf("Expected PHP server to use Supervisor, got %v", pm)
	}
	if pm := ServerTypeDatabase.GetProcessManager(); pm != ProcessManagerNone {
		t.Errorf("Expected Database server to have no process manager, got %v", pm)
	}
}

func TestServerType_Scan(t *testing.T) {
	var st ServerType

	if err := st.Scan("php"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if st != ServerTypePhp {
		t.Errorf("Expected ServerTypePhp, got %v", st)
	}

	if err := st.Scan([]byte("database")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if st != ServerTypeDatabase {
		t.Errorf("Expected ServerTypeDatabase, got %v", st)
	}

	if err := st.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := st.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestServerType_Value(t *testing.T) {
	st := ServerTypePhp
	v, err := st.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "php" {
		t.Errorf("Expected 'php', got %v", v)
	}
}

func TestParseServerType(t *testing.T) {
	tests := []struct {
		input    string
		expected ServerType
		hasError bool
	}{
		{"php", ServerTypePhp, false},
		{"database", ServerTypeDatabase, false},
		{"invalid", ServerTypePhp, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseServerType(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseServerType() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseServerType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllServerTypes(t *testing.T) {
	types := AllServerTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 server types, got %d", len(types))
	}
}

func TestServerProvider_String(t *testing.T) {
	tests := []struct {
		provider ServerProvider
		expected string
	}{
		{ProviderDigitalOcean, "digitalocean"},
		{ProviderHetzner, "hetzner"},
		{ProviderLinode, "linode"},
		{ProviderVultr, "vultr"},
		{ProviderAWS, "aws"},
		{ProviderCustom, "custom_server"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.provider.String(); got != tt.expected {
				t.Errorf("ServerProvider.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerProvider_Label(t *testing.T) {
	tests := []struct {
		provider ServerProvider
		expected string
	}{
		{ProviderDigitalOcean, "DigitalOcean"},
		{ProviderHetzner, "Hetzner Cloud"},
		{ProviderLinode, "Linode"},
		{ProviderVultr, "Vultr"},
		{ProviderAWS, "AWS"},
		{ProviderCustom, "Custom"},
		{ServerProvider("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if got := tt.provider.Label(); got != tt.expected {
				t.Errorf("ServerProvider.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerProvider_IsValid(t *testing.T) {
	tests := []struct {
		provider ServerProvider
		expected bool
	}{
		{ProviderDigitalOcean, true},
		{ProviderHetzner, true},
		{ProviderLinode, true},
		{ProviderVultr, true},
		{ProviderAWS, true},
		{ProviderCustom, true},
		{ServerProvider("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if got := tt.provider.IsValid(); got != tt.expected {
				t.Errorf("ServerProvider.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerProvider_IsCloud(t *testing.T) {
	tests := []struct {
		provider ServerProvider
		expected bool
	}{
		{ProviderDigitalOcean, true},
		{ProviderHetzner, true},
		{ProviderCustom, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if got := tt.provider.IsCloud(); got != tt.expected {
				t.Errorf("ServerProvider.IsCloud() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerProvider_GetDefaultUsername(t *testing.T) {
	if username := ProviderAWS.GetDefaultUsername(OSUbuntu24); username != "ubuntu" {
		t.Errorf("Expected AWS default username 'ubuntu', got %v", username)
	}
	if username := ProviderDigitalOcean.GetDefaultUsername(OSUbuntu24); username != "root" {
		t.Errorf("Expected DigitalOcean default username 'root', got %v", username)
	}
}

func TestServerProvider_Scan(t *testing.T) {
	var sp ServerProvider

	if err := sp.Scan("digitalocean"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if sp != ProviderDigitalOcean {
		t.Errorf("Expected ProviderDigitalOcean, got %v", sp)
	}

	if err := sp.Scan([]byte("hetzner")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if sp != ProviderHetzner {
		t.Errorf("Expected ProviderHetzner, got %v", sp)
	}

	if err := sp.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := sp.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestServerProvider_Value(t *testing.T) {
	sp := ProviderDigitalOcean
	v, err := sp.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "digitalocean" {
		t.Errorf("Expected 'digitalocean', got %v", v)
	}
}

func TestParseServerProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected ServerProvider
		hasError bool
	}{
		{"digitalocean", ProviderDigitalOcean, false},
		{"hetzner", ProviderHetzner, false},
		{"custom_server", ProviderCustom, false},
		{"invalid", ProviderCustom, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseServerProvider(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseServerProvider() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseServerProvider() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllServerProviders(t *testing.T) {
	providers := AllServerProviders()
	if len(providers) != 6 {
		t.Errorf("Expected 6 server providers, got %d", len(providers))
	}
}

func TestOperatingSystem_String(t *testing.T) {
	tests := []struct {
		os       OperatingSystem
		expected string
	}{
		{OSUbuntu20, "ubuntu_20"},
		{OSUbuntu22, "ubuntu_22"},
		{OSUbuntu24, "ubuntu_24"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.os.String(); got != tt.expected {
				t.Errorf("OperatingSystem.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOperatingSystem_Label(t *testing.T) {
	tests := []struct {
		os       OperatingSystem
		expected string
	}{
		{OSUbuntu20, "Ubuntu 20.04"},
		{OSUbuntu22, "Ubuntu 22.04"},
		{OSUbuntu24, "Ubuntu 24.04"},
		{OperatingSystem("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.os), func(t *testing.T) {
			if got := tt.os.Label(); got != tt.expected {
				t.Errorf("OperatingSystem.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOperatingSystem_IsValid(t *testing.T) {
	tests := []struct {
		os       OperatingSystem
		expected bool
	}{
		{OSUbuntu20, true},
		{OSUbuntu22, true},
		{OSUbuntu24, true},
		{OperatingSystem("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.os), func(t *testing.T) {
			if got := tt.os.IsValid(); got != tt.expected {
				t.Errorf("OperatingSystem.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOperatingSystem_Scan(t *testing.T) {
	var os OperatingSystem

	if err := os.Scan("ubuntu_24"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if os != OSUbuntu24 {
		t.Errorf("Expected OSUbuntu24, got %v", os)
	}

	if err := os.Scan([]byte("ubuntu_22")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if os != OSUbuntu22 {
		t.Errorf("Expected OSUbuntu22, got %v", os)
	}

	if err := os.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := os.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestOperatingSystem_Value(t *testing.T) {
	os := OSUbuntu24
	v, err := os.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "ubuntu_24" {
		t.Errorf("Expected 'ubuntu_24', got %v", v)
	}
}

func TestParseOperatingSystem(t *testing.T) {
	tests := []struct {
		input    string
		expected OperatingSystem
		hasError bool
	}{
		{"ubuntu_24", OSUbuntu24, false},
		{"ubuntu_22", OSUbuntu22, false},
		{"ubuntu_20", OSUbuntu20, false},
		{"invalid", OSUbuntu24, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOperatingSystem(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseOperatingSystem() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseOperatingSystem() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllOperatingSystems(t *testing.T) {
	oses := AllOperatingSystems()
	if len(oses) != 3 {
		t.Errorf("Expected 3 operating systems, got %d", len(oses))
	}
}

func TestServiceType_String(t *testing.T) {
	tests := []struct {
		serviceType ServiceType
		expected    string
	}{
		{ServiceTypePhp, "php"},
		{ServiceTypeMySql, "mysql"},
		{ServiceTypePostgreSql, "postgresql"},
		{ServiceTypeSupervisor, "process_manager"},
		{ServiceTypeRedis, "memory_database"},
		{ServiceTypeCaddy, "webserver"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.serviceType.String(); got != tt.expected {
				t.Errorf("ServiceType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServiceType_IsDatabase(t *testing.T) {
	tests := []struct {
		serviceType ServiceType
		expected    bool
	}{
		{ServiceTypeMySql, true},
		{ServiceTypePostgreSql, true},
		{ServiceTypePhp, false},
		{ServiceTypeRedis, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.serviceType), func(t *testing.T) {
			if got := tt.serviceType.IsDatabase(); got != tt.expected {
				t.Errorf("ServiceType.IsDatabase() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServiceType_GetDatabasePort(t *testing.T) {
	if port := ServiceTypeMySql.GetDatabasePort(); port != 3306 {
		t.Errorf("Expected MySQL port 3306, got %d", port)
	}
	if port := ServiceTypePostgreSql.GetDatabasePort(); port != 5432 {
		t.Errorf("Expected PostgreSQL port 5432, got %d", port)
	}
	if port := ServiceTypePhp.GetDatabasePort(); port != 0 {
		t.Errorf("Expected PHP port 0, got %d", port)
	}
}

func TestServiceType_GetDatabaseConnection(t *testing.T) {
	if conn := ServiceTypeMySql.GetDatabaseConnection(); conn != "mysql" {
		t.Errorf("Expected MySQL connection 'mysql', got %s", conn)
	}
	if conn := ServiceTypePostgreSql.GetDatabaseConnection(); conn != "pgsql" {
		t.Errorf("Expected PostgreSQL connection 'pgsql', got %s", conn)
	}
}

func TestServiceStatus_String(t *testing.T) {
	tests := []struct {
		status   ServiceStatus
		expected string
	}{
		{ServiceStatusPending, "pending"},
		{ServiceStatusInstalling, "installing"},
		{ServiceStatusFailed, "failed"},
		{ServiceStatusInstalled, "installed"},
		{ServiceStatusStopped, "stopped"},
		{ServiceStatusRunning, "running"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("ServiceStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServiceStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   ServiceStatus
		expected bool
	}{
		{ServiceStatusRunning, true},
		{ServiceStatusInstalled, true},
		{ServiceStatusPending, false},
		{ServiceStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsActive(); got != tt.expected {
				t.Errorf("ServiceStatus.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRuleAction_String(t *testing.T) {
	tests := []struct {
		action   RuleAction
		expected string
	}{
		{RuleActionAllow, "allow"},
		{RuleActionDeny, "deny"},
		{RuleActionReject, "reject"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.action.String(); got != tt.expected {
				t.Errorf("RuleAction.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRuleAction_Label(t *testing.T) {
	tests := []struct {
		action   RuleAction
		expected string
	}{
		{RuleActionAllow, "Allow"},
		{RuleActionDeny, "Deny"},
		{RuleActionReject, "Reject"},
		{RuleAction("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.Label(); got != tt.expected {
				t.Errorf("RuleAction.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseRuleAction(t *testing.T) {
	tests := []struct {
		input    string
		expected RuleAction
		hasError bool
	}{
		{"allow", RuleActionAllow, false},
		{"deny", RuleActionDeny, false},
		{"reject", RuleActionReject, false},
		{"invalid", RuleActionAllow, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRuleAction(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseRuleAction() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseRuleAction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllRuleActions(t *testing.T) {
	actions := AllRuleActions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 rule actions, got %d", len(actions))
	}
}

func TestSoftware_String(t *testing.T) {
	tests := []struct {
		software Software
		expected string
	}{
		{SoftwareCaddy2, "caddy2"},
		{SoftwarePhp84, "php84"},
		{SoftwareMySql80, "mysql80"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.software.String(); got != tt.expected {
				t.Errorf("Software.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSoftware_Label(t *testing.T) {
	tests := []struct {
		software Software
		expected string
	}{
		{SoftwareCaddy2, "Caddy 2"},
		{SoftwarePhp84, "PHP 8.4"},
		{SoftwareMySql80, "MySQL 8.0"},
		{SoftwareRedis, "Redis"},
		{Software("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.software), func(t *testing.T) {
			if got := tt.software.Label(); got != tt.expected {
				t.Errorf("Software.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSoftware_GetVersion(t *testing.T) {
	tests := []struct {
		software Software
		expected string
	}{
		{SoftwarePhp84, "8.4"},
		{SoftwarePhp74, "7.4"},
		{SoftwareMySql80, "8.0"},
		{SoftwareRedis, "latest"},
	}

	for _, tt := range tests {
		t.Run(string(tt.software), func(t *testing.T) {
			if got := tt.software.GetVersion(); got != tt.expected {
				t.Errorf("Software.GetVersion() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSoftware_GetServiceType(t *testing.T) {
	tests := []struct {
		software    Software
		serviceType ServiceType
	}{
		{SoftwarePhp84, ServiceTypePhp},
		{SoftwarePhp74, ServiceTypePhp},
		{SoftwareMySql80, ServiceTypeMySql},
		{SoftwarePostgreSql16, ServiceTypePostgreSql},
		{SoftwareRedis, ServiceTypeRedis},
		{SoftwareCaddy2, ServiceTypeCaddy},
	}

	for _, tt := range tests {
		t.Run(string(tt.software), func(t *testing.T) {
			if got := tt.software.GetServiceType(); got != tt.serviceType {
				t.Errorf("Software.GetServiceType() = %v, want %v", got, tt.serviceType)
			}
		})
	}
}

func TestSoftware_IsPhp(t *testing.T) {
	tests := []struct {
		software Software
		expected bool
	}{
		{SoftwarePhp84, true},
		{SoftwarePhp74, true},
		{SoftwarePhp56, true},
		{SoftwareMySql80, false},
		{SoftwareRedis, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.software), func(t *testing.T) {
			if got := tt.software.IsPhp(); got != tt.expected {
				t.Errorf("Software.IsPhp() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSoftware_IsDatabase(t *testing.T) {
	tests := []struct {
		software Software
		expected bool
	}{
		{SoftwareMySql80, true},
		{SoftwarePostgreSql16, true},
		{SoftwarePhp84, false},
		{SoftwareRedis, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.software), func(t *testing.T) {
			if got := tt.software.IsDatabase(); got != tt.expected {
				t.Errorf("Software.IsDatabase() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSoftware_Group(t *testing.T) {
	tests := []struct {
		software Software
		expected string
	}{
		{SoftwarePhp84, "php"},
		{SoftwareMySql80, "mysql"},
		{SoftwarePostgreSql16, "postgresql"},
		{SoftwareRedis, "redis"},
	}

	for _, tt := range tests {
		t.Run(string(tt.software), func(t *testing.T) {
			if got := tt.software.Group(); got != tt.expected {
				t.Errorf("Software.Group() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseSoftware(t *testing.T) {
	tests := []struct {
		input    string
		expected Software
		hasError bool
	}{
		{"php84", SoftwarePhp84, false},
		{"mysql80", SoftwareMySql80, false},
		{"redis", SoftwareRedis, false},
		{"invalid", Software(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSoftware(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseSoftware() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseSoftware() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllPhpVersions(t *testing.T) {
	versions := AllPhpVersions()
	if len(versions) == 0 {
		t.Error("Expected at least one PHP version")
	}
	for _, v := range versions {
		if !v.IsPhp() {
			t.Errorf("Expected %v to be PHP", v)
		}
	}
}

func TestAllDatabaseTypes(t *testing.T) {
	types := AllDatabaseTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 database types, got %d", len(types))
	}
	for _, t := range types {
		if !t.IsDatabase() {
			// Note: can't use t.Errorf here since we shadowed t
			panic("Expected database type")
		}
	}
}

func TestServerFeature_String(t *testing.T) {
	tests := []struct {
		feature  ServerFeature
		expected string
	}{
		{ServerFeatureSites, "sites"},
		{ServerFeaturePhpManagement, "php_management"},
		{ServerFeatureDatabaseManagement, "database_management"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.feature.String(); got != tt.expected {
				t.Errorf("ServerFeature.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerFeature_NavigationKey(t *testing.T) {
	tests := []struct {
		feature  ServerFeature
		expected string
	}{
		{ServerFeatureSites, "sites"},
		{ServerFeatureDatabaseManagement, "databases"},
		{ServerFeaturePhpManagement, "php"},
	}

	for _, tt := range tests {
		t.Run(string(tt.feature), func(t *testing.T) {
			if got := tt.feature.NavigationKey(); got != tt.expected {
				t.Errorf("ServerFeature.NavigationKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProcessManager_String(t *testing.T) {
	tests := []struct {
		pm       ProcessManager
		expected string
	}{
		{ProcessManagerSupervisor, "supervisor"},
		{ProcessManagerNone, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.pm.String(); got != tt.expected {
				t.Errorf("ProcessManager.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProcessManager_ServiceName(t *testing.T) {
	if name := ProcessManagerSupervisor.ServiceName(); name != "supervisor" {
		t.Errorf("Expected 'supervisor', got %s", name)
	}
	if name := ProcessManagerNone.ServiceName(); name != "" {
		t.Errorf("Expected empty string, got %s", name)
	}
}

func TestServiceOption_String(t *testing.T) {
	tests := []struct {
		option   ServiceOption
		expected string
	}{
		{ServiceOptionStart, "start"},
		{ServiceOptionStop, "stop"},
		{ServiceOptionRestart, "restart"},
		{ServiceOptionRemove, "remove"},
		{ServiceOptionStatus, "status"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.option.String(); got != tt.expected {
				t.Errorf("ServiceOption.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServiceOption_Label(t *testing.T) {
	tests := []struct {
		option   ServiceOption
		expected string
	}{
		{ServiceOptionStart, "Start"},
		{ServiceOptionStop, "Stop"},
		{ServiceOptionRestart, "Restart"},
		{ServiceOptionRemove, "Remove"},
		{ServiceOptionStatus, "Status"},
		{ServiceOption("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.option), func(t *testing.T) {
			if got := tt.option.Label(); got != tt.expected {
				t.Errorf("ServiceOption.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseServiceOption(t *testing.T) {
	tests := []struct {
		input    string
		expected ServiceOption
		hasError bool
	}{
		{"start", ServiceOptionStart, false},
		{"stop", ServiceOptionStop, false},
		{"restart", ServiceOptionRestart, false},
		{"remove", ServiceOptionRemove, false},
		{"status", ServiceOptionStatus, false},
		{"invalid", ServiceOption(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseServiceOption(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseServiceOption() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseServiceOption() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllServiceOptions(t *testing.T) {
	options := AllServiceOptions()
	if len(options) != 5 {
		t.Errorf("Expected 5 service options, got %d", len(options))
	}
}
