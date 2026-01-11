package server

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
)

func TestServerStatus_String(t *testing.T) {
	tests := []struct {
		status   enums.ServerStatus
		expected string
	}{
		{enums.ServerStatusNew, "new"},
		{enums.ServerStatusStarting, "starting"},
		{enums.ServerStatusProvisioning, "provisioning"},
		{enums.ServerStatusRunning, "running"},
		{enums.ServerStatusPaused, "paused"},
		{enums.ServerStatusStopped, "stopped"},
		{enums.ServerStatusDeleting, "deleting"},
		{enums.ServerStatusArchived, "archived"},
		{enums.ServerStatusUnknown, "unknown"},
		{enums.ServerStatusFailed, "failed"},
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
		status   enums.ServerStatus
		expected string
	}{
		{enums.ServerStatusNew, "Connecting"},
		{enums.ServerStatusStarting, "Starting"},
		{enums.ServerStatusProvisioning, "Provisioning"},
		{enums.ServerStatusRunning, "Running"},
		{enums.ServerStatusPaused, "Paused"},
		{enums.ServerStatusStopped, "Stopped"},
		{enums.ServerStatusDeleting, "Deleting"},
		{enums.ServerStatusArchived, "Archived"},
		{enums.ServerStatusUnknown, "Unknown"},
		{enums.ServerStatusFailed, "Failed"},
		{enums.ServerStatus("invalid"), "Unknown"},
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
		status   enums.ServerStatus
		expected bool
	}{
		{enums.ServerStatusNew, true},
		{enums.ServerStatusStarting, true},
		{enums.ServerStatusProvisioning, true},
		{enums.ServerStatusRunning, true},
		{enums.ServerStatusPaused, true},
		{enums.ServerStatusStopped, true},
		{enums.ServerStatusDeleting, true},
		{enums.ServerStatusArchived, true},
		{enums.ServerStatusUnknown, true},
		{enums.ServerStatusFailed, true},
		{enums.ServerStatus("invalid"), false},
		{enums.ServerStatus(""), false},
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
		status   enums.ServerStatus
		expected bool
	}{
		{enums.ServerStatusRunning, true},
		{enums.ServerStatusProvisioning, true},
		{enums.ServerStatusStarting, true},
		{enums.ServerStatusNew, false},
		{enums.ServerStatusPaused, false},
		{enums.ServerStatusStopped, false},
		{enums.ServerStatusArchived, false},
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
		status   enums.ServerStatus
		expected bool
	}{
		{enums.ServerStatusFailed, true},
		{enums.ServerStatusArchived, true},
		{enums.ServerStatusDeleting, true},
		{enums.ServerStatusRunning, false},
		{enums.ServerStatusNew, false},
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
	var ss enums.ServerStatus

	if err := ss.Scan("running"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ss != enums.ServerStatusRunning {
		t.Errorf("Expected ServerStatusRunning, got %v", ss)
	}

	if err := ss.Scan([]byte("provisioning")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ss != enums.ServerStatusProvisioning {
		t.Errorf("Expected ServerStatusProvisioning, got %v", ss)
	}

	if err := ss.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}
	if ss != enums.ServerStatusUnknown {
		t.Errorf("Expected ServerStatusUnknown for nil, got %v", ss)
	}

	if err := ss.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestServerStatus_Value(t *testing.T) {
	ss := enums.ServerStatusRunning
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
		expected enums.ServerStatus
		hasError bool
	}{
		{"running", enums.ServerStatusRunning, false},
		{"new", enums.ServerStatusNew, false},
		{"failed", enums.ServerStatusFailed, false},
		{"invalid", enums.ServerStatusUnknown, true},
		{"", enums.ServerStatusUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseServerStatus(tt.input)
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
	statuses := enums.AllServerStatuses()
	if len(statuses) != 10 {
		t.Errorf("Expected 10 server statuses, got %d", len(statuses))
	}
}

func TestServerType_String(t *testing.T) {
	tests := []struct {
		serverType enums.ServerType
		expected   string
	}{
		{enums.ServerTypePhp, "php"},
		{enums.ServerTypeDatabase, "database"},
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
		serverType enums.ServerType
		expected   string
	}{
		{enums.ServerTypePhp, "PHP Application Server"},
		{enums.ServerTypeDatabase, "Database Server"},
		{enums.ServerType("invalid"), "Unknown"},
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
		serverType enums.ServerType
		expected   bool
	}{
		{enums.ServerTypePhp, true},
		{enums.ServerTypeDatabase, true},
		{enums.ServerType("invalid"), false},
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
	phpFeatures := enums.ServerTypePhp.GetFeatures()
	if len(phpFeatures) == 0 {
		t.Error("Expected PHP server to have features")
	}

	hasPhpManagement := false
	for _, f := range phpFeatures {
		if f == enums.ServerFeaturePhpManagement {
			hasPhpManagement = true
			break
		}
	}
	if !hasPhpManagement {
		t.Error("Expected PHP server to have PHP management feature")
	}

	dbFeatures := enums.ServerTypeDatabase.GetFeatures()
	if len(dbFeatures) == 0 {
		t.Error("Expected Database server to have features")
	}
}

func TestServerType_HasFeature(t *testing.T) {
	if !enums.ServerTypePhp.HasFeature(enums.ServerFeaturePhpManagement) {
		t.Error("Expected PHP server to have PHP management feature")
	}
	if !enums.ServerTypePhp.HasFeature(enums.ServerFeatureSites) {
		t.Error("Expected PHP server to have sites feature")
	}
	if enums.ServerTypeDatabase.HasFeature(enums.ServerFeaturePhpManagement) {
		t.Error("Expected Database server to not have PHP management feature")
	}
}

func TestServerType_GetProcessManager(t *testing.T) {
	if pm := enums.ServerTypePhp.GetProcessManager(); pm != enums.ProcessManagerSupervisor {
		t.Errorf("Expected PHP server to use Supervisor, got %v", pm)
	}
	if pm := enums.ServerTypeDatabase.GetProcessManager(); pm != enums.ProcessManagerNone {
		t.Errorf("Expected Database server to have no process manager, got %v", pm)
	}
}

func TestServerType_Scan(t *testing.T) {
	var st enums.ServerType

	if err := st.Scan("php"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if st != enums.ServerTypePhp {
		t.Errorf("Expected ServerTypePhp, got %v", st)
	}

	if err := st.Scan([]byte("database")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if st != enums.ServerTypeDatabase {
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
	st := enums.ServerTypePhp
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
		expected enums.ServerType
		hasError bool
	}{
		{"php", enums.ServerTypePhp, false},
		{"database", enums.ServerTypeDatabase, false},
		{"invalid", enums.ServerTypePhp, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseServerType(tt.input)
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
	types := enums.AllServerTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 server types, got %d", len(types))
	}
}

func TestServerProvider_String(t *testing.T) {
	tests := []struct {
		provider enums.ServerProvider
		expected string
	}{
		{enums.ProviderDigitalOcean, "digitalocean"},
		{enums.ProviderHetzner, "hetzner"},
		{enums.ProviderLinode, "linode"},
		{enums.ProviderVultr, "vultr"},
		{enums.ProviderAWS, "aws"},
		{enums.ProviderCustom, "custom_server"},
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
		provider enums.ServerProvider
		expected string
	}{
		{enums.ProviderDigitalOcean, "DigitalOcean"},
		{enums.ProviderHetzner, "Hetzner Cloud"},
		{enums.ProviderLinode, "Linode"},
		{enums.ProviderVultr, "Vultr"},
		{enums.ProviderAWS, "AWS"},
		{enums.ProviderCustom, "Custom"},
		{enums.ServerProvider("invalid"), "Unknown"},
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
		provider enums.ServerProvider
		expected bool
	}{
		{enums.ProviderDigitalOcean, true},
		{enums.ProviderHetzner, true},
		{enums.ProviderLinode, true},
		{enums.ProviderVultr, true},
		{enums.ProviderAWS, true},
		{enums.ProviderCustom, true},
		{enums.ServerProvider("invalid"), false},
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
		provider enums.ServerProvider
		expected bool
	}{
		{enums.ProviderDigitalOcean, true},
		{enums.ProviderHetzner, true},
		{enums.ProviderCustom, false},
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
	if username := enums.ProviderAWS.GetDefaultUsername(enums.OSUbuntu24); username != "ubuntu" {
		t.Errorf("Expected AWS default username 'ubuntu', got %v", username)
	}
	if username := enums.ProviderDigitalOcean.GetDefaultUsername(enums.OSUbuntu24); username != "root" {
		t.Errorf("Expected DigitalOcean default username 'root', got %v", username)
	}
}

func TestServerProvider_Scan(t *testing.T) {
	var sp enums.ServerProvider

	if err := sp.Scan("digitalocean"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if sp != enums.ProviderDigitalOcean {
		t.Errorf("Expected ProviderDigitalOcean, got %v", sp)
	}

	if err := sp.Scan([]byte("hetzner")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if sp != enums.ProviderHetzner {
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
	sp := enums.ProviderDigitalOcean
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
		expected enums.ServerProvider
		hasError bool
	}{
		{"digitalocean", enums.ProviderDigitalOcean, false},
		{"hetzner", enums.ProviderHetzner, false},
		{"custom_server", enums.ProviderCustom, false},
		{"invalid", enums.ProviderCustom, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseServerProvider(tt.input)
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
	providers := enums.AllServerProviders()
	if len(providers) != 6 {
		t.Errorf("Expected 6 server providers, got %d", len(providers))
	}
}

func TestOperatingSystem_String(t *testing.T) {
	tests := []struct {
		os       enums.OperatingSystem
		expected string
	}{
		{enums.OSUbuntu20, "ubuntu_20"},
		{enums.OSUbuntu22, "ubuntu_22"},
		{enums.OSUbuntu24, "ubuntu_24"},
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
		os       enums.OperatingSystem
		expected string
	}{
		{enums.OSUbuntu20, "Ubuntu 20.04"},
		{enums.OSUbuntu22, "Ubuntu 22.04"},
		{enums.OSUbuntu24, "Ubuntu 24.04"},
		{enums.OperatingSystem("invalid"), "Unknown"},
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
		os       enums.OperatingSystem
		expected bool
	}{
		{enums.OSUbuntu20, true},
		{enums.OSUbuntu22, true},
		{enums.OSUbuntu24, true},
		{enums.OperatingSystem("invalid"), false},
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
	var os enums.OperatingSystem

	if err := os.Scan("ubuntu_24"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if os != enums.OSUbuntu24 {
		t.Errorf("Expected OSUbuntu24, got %v", os)
	}

	if err := os.Scan([]byte("ubuntu_22")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if os != enums.OSUbuntu22 {
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
	os := enums.OSUbuntu24
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
		expected enums.OperatingSystem
		hasError bool
	}{
		{"ubuntu_24", enums.OSUbuntu24, false},
		{"ubuntu_22", enums.OSUbuntu22, false},
		{"ubuntu_20", enums.OSUbuntu20, false},
		{"invalid", enums.OSUbuntu24, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseOperatingSystem(tt.input)
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
	oses := enums.AllOperatingSystems()
	if len(oses) != 3 {
		t.Errorf("Expected 3 operating systems, got %d", len(oses))
	}
}

func TestServiceType_String(t *testing.T) {
	tests := []struct {
		serviceType enums.ServiceType
		expected    string
	}{
		{enums.ServiceTypePhp, "php"},
		{enums.ServiceTypeMySql, "mysql"},
		{enums.ServiceTypePostgreSql, "postgresql"},
		{enums.ServiceTypeSupervisor, "process_manager"},
		{enums.ServiceTypeRedis, "memory_database"},
		{enums.ServiceTypeCaddy, "webserver"},
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
		serviceType enums.ServiceType
		expected    bool
	}{
		{enums.ServiceTypeMySql, true},
		{enums.ServiceTypePostgreSql, true},
		{enums.ServiceTypePhp, false},
		{enums.ServiceTypeRedis, false},
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
	if port := enums.ServiceTypeMySql.GetDatabasePort(); port != 3306 {
		t.Errorf("Expected MySQL port 3306, got %d", port)
	}
	if port := enums.ServiceTypePostgreSql.GetDatabasePort(); port != 5432 {
		t.Errorf("Expected PostgreSQL port 5432, got %d", port)
	}
	if port := enums.ServiceTypePhp.GetDatabasePort(); port != 0 {
		t.Errorf("Expected PHP port 0, got %d", port)
	}
}

func TestServiceType_GetDatabaseConnection(t *testing.T) {
	if conn := enums.ServiceTypeMySql.GetDatabaseConnection(); conn != "mysql" {
		t.Errorf("Expected MySQL connection 'mysql', got %s", conn)
	}
	if conn := enums.ServiceTypePostgreSql.GetDatabaseConnection(); conn != "pgsql" {
		t.Errorf("Expected PostgreSQL connection 'pgsql', got %s", conn)
	}
}

func TestServiceStatus_String(t *testing.T) {
	tests := []struct {
		status   enums.ServiceStatus
		expected string
	}{
		{enums.ServiceStatusPending, "pending"},
		{enums.ServiceStatusInstalling, "installing"},
		{enums.ServiceStatusFailed, "failed"},
		{enums.ServiceStatusInstalled, "installed"},
		{enums.ServiceStatusStopped, "stopped"},
		{enums.ServiceStatusRunning, "running"},
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
		status   enums.ServiceStatus
		expected bool
	}{
		{enums.ServiceStatusRunning, true},
		{enums.ServiceStatusInstalled, true},
		{enums.ServiceStatusPending, false},
		{enums.ServiceStatusFailed, false},
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
		action   enums.RuleAction
		expected string
	}{
		{enums.RuleActionAllow, "allow"},
		{enums.RuleActionDeny, "deny"},
		{enums.RuleActionReject, "reject"},
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
		action   enums.RuleAction
		expected string
	}{
		{enums.RuleActionAllow, "Allow"},
		{enums.RuleActionDeny, "Deny"},
		{enums.RuleActionReject, "Reject"},
		{enums.RuleAction("invalid"), "Unknown"},
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
		expected enums.RuleAction
		hasError bool
	}{
		{"allow", enums.RuleActionAllow, false},
		{"deny", enums.RuleActionDeny, false},
		{"reject", enums.RuleActionReject, false},
		{"invalid", enums.RuleActionAllow, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseRuleAction(tt.input)
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
	actions := enums.AllRuleActions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 rule actions, got %d", len(actions))
	}
}

func TestSoftware_String(t *testing.T) {
	tests := []struct {
		software enums.Software
		expected string
	}{
		{enums.SoftwareCaddy2, "caddy2"},
		{enums.SoftwarePhp84, "php84"},
		{enums.SoftwareMySql80, "mysql80"},
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
		software enums.Software
		expected string
	}{
		{enums.SoftwareCaddy2, "Caddy 2"},
		{enums.SoftwarePhp84, "PHP 8.4"},
		{enums.SoftwareMySql80, "MySQL 8.0"},
		{enums.SoftwareRedis, "Redis"},
		{enums.Software("invalid"), "Unknown"},
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
		software enums.Software
		expected string
	}{
		{enums.SoftwarePhp84, "8.4"},
		{enums.SoftwarePhp74, "7.4"},
		{enums.SoftwareMySql80, "8.0"},
		{enums.SoftwareRedis, "latest"},
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
		software    enums.Software
		serviceType enums.ServiceType
	}{
		{enums.SoftwarePhp84, enums.ServiceTypePhp},
		{enums.SoftwarePhp74, enums.ServiceTypePhp},
		{enums.SoftwareMySql80, enums.ServiceTypeMySql},
		{enums.SoftwarePostgreSql16, enums.ServiceTypePostgreSql},
		{enums.SoftwareRedis, enums.ServiceTypeRedis},
		{enums.SoftwareCaddy2, enums.ServiceTypeCaddy},
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
		software enums.Software
		expected bool
	}{
		{enums.SoftwarePhp84, true},
		{enums.SoftwarePhp74, true},
		{enums.SoftwarePhp56, true},
		{enums.SoftwareMySql80, false},
		{enums.SoftwareRedis, false},
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
		software enums.Software
		expected bool
	}{
		{enums.SoftwareMySql80, true},
		{enums.SoftwarePostgreSql16, true},
		{enums.SoftwarePhp84, false},
		{enums.SoftwareRedis, false},
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
		software enums.Software
		expected string
	}{
		{enums.SoftwarePhp84, "php"},
		{enums.SoftwareMySql80, "mysql"},
		{enums.SoftwarePostgreSql16, "postgresql"},
		{enums.SoftwareRedis, "redis"},
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
		expected enums.Software
		hasError bool
	}{
		{"php84", enums.SoftwarePhp84, false},
		{"mysql80", enums.SoftwareMySql80, false},
		{"redis", enums.SoftwareRedis, false},
		{"invalid", enums.Software(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseSoftware(tt.input)
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
	versions := enums.AllPhpVersions()
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
	types := enums.AllDatabaseTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 database types, got %d", len(types))
	}
	for _, dbType := range types {
		if !dbType.IsDatabase() {
			t.Errorf("Expected database type, got %v", dbType)
		}
	}
}

func TestServerFeature_String(t *testing.T) {
	tests := []struct {
		feature  enums.ServerFeature
		expected string
	}{
		{enums.ServerFeatureSites, "sites"},
		{enums.ServerFeaturePhpManagement, "php_management"},
		{enums.ServerFeatureDatabaseManagement, "database_management"},
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
		feature  enums.ServerFeature
		expected string
	}{
		{enums.ServerFeatureSites, "sites"},
		{enums.ServerFeatureDatabaseManagement, "databases"},
		{enums.ServerFeaturePhpManagement, "php"},
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
		pm       enums.ProcessManager
		expected string
	}{
		{enums.ProcessManagerSupervisor, "supervisor"},
		{enums.ProcessManagerNone, "none"},
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
	if name := enums.ProcessManagerSupervisor.ServiceName(); name != "supervisor" {
		t.Errorf("Expected 'supervisor', got %s", name)
	}
	if name := enums.ProcessManagerNone.ServiceName(); name != "" {
		t.Errorf("Expected empty string, got %s", name)
	}
}

func TestServiceOption_String(t *testing.T) {
	tests := []struct {
		option   enums.ServiceOption
		expected string
	}{
		{enums.ServiceOptionStart, "start"},
		{enums.ServiceOptionStop, "stop"},
		{enums.ServiceOptionRestart, "restart"},
		{enums.ServiceOptionRemove, "remove"},
		{enums.ServiceOptionStatus, "status"},
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
		option   enums.ServiceOption
		expected string
	}{
		{enums.ServiceOptionStart, "Start"},
		{enums.ServiceOptionStop, "Stop"},
		{enums.ServiceOptionRestart, "Restart"},
		{enums.ServiceOptionRemove, "Remove"},
		{enums.ServiceOptionStatus, "Status"},
		{enums.ServiceOption("invalid"), "Unknown"},
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
		expected enums.ServiceOption
		hasError bool
	}{
		{"start", enums.ServiceOptionStart, false},
		{"stop", enums.ServiceOptionStop, false},
		{"restart", enums.ServiceOptionRestart, false},
		{"remove", enums.ServiceOptionRemove, false},
		{"status", enums.ServiceOptionStatus, false},
		{"invalid", enums.ServiceOption(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseServiceOption(tt.input)
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
	options := enums.AllServiceOptions()
	if len(options) != 5 {
		t.Errorf("Expected 5 service options, got %d", len(options))
	}
}
