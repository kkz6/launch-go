package site

import (
	"testing"
)

func TestSiteType_String(t *testing.T) {
	tests := []struct {
		siteType SiteType
		expected string
	}{
		{SiteTypeLaravel, "laravel"},
		{SiteTypeWordpress, "wordpress"},
		{SiteTypeStatic, "static"},
		{SiteTypeGeneric, "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.siteType.String(); got != tt.expected {
				t.Errorf("SiteType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSiteType_Label(t *testing.T) {
	tests := []struct {
		siteType SiteType
		expected string
	}{
		{SiteTypeLaravel, "Laravel"},
		{SiteTypeWordpress, "Wordpress"},
		{SiteTypeStatic, "Static"},
		{SiteTypeGeneric, "Generic"},
		{SiteType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.siteType), func(t *testing.T) {
			if got := tt.siteType.Label(); got != tt.expected {
				t.Errorf("SiteType.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSiteType_IsValid(t *testing.T) {
	tests := []struct {
		siteType SiteType
		expected bool
	}{
		{SiteTypeLaravel, true},
		{SiteTypeWordpress, true},
		{SiteTypeStatic, true},
		{SiteTypeGeneric, true},
		{SiteType("unknown"), false},
		{SiteType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.siteType), func(t *testing.T) {
			if got := tt.siteType.IsValid(); got != tt.expected {
				t.Errorf("SiteType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSiteType_HasEnvironment(t *testing.T) {
	tests := []struct {
		siteType SiteType
		expected bool
	}{
		{SiteTypeLaravel, true},
		{SiteTypeWordpress, true},
		{SiteTypeStatic, false},
		{SiteTypeGeneric, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.siteType), func(t *testing.T) {
			if got := tt.siteType.HasEnvironment(); got != tt.expected {
				t.Errorf("SiteType.HasEnvironment() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSiteType_RequiresGitAccount(t *testing.T) {
	tests := []struct {
		siteType SiteType
		expected bool
	}{
		{SiteTypeLaravel, true},
		{SiteTypeWordpress, false},
		{SiteTypeStatic, true},
		{SiteTypeGeneric, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.siteType), func(t *testing.T) {
			if got := tt.siteType.RequiresGitAccount(); got != tt.expected {
				t.Errorf("SiteType.RequiresGitAccount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSiteType_GetDatabaseEnvVarNames(t *testing.T) {
	laravelVars := SiteTypeLaravel.GetDatabaseEnvVarNames()
	if laravelVars["database"] != "DB_DATABASE" {
		t.Errorf("Expected Laravel database var to be DB_DATABASE, got %s", laravelVars["database"])
	}
	if laravelVars["port"] != "DB_PORT" {
		t.Errorf("Expected Laravel port var to be DB_PORT, got %s", laravelVars["port"])
	}

	wordpressVars := SiteTypeWordpress.GetDatabaseEnvVarNames()
	if wordpressVars["database"] != "DB_NAME" {
		t.Errorf("Expected WordPress database var to be DB_NAME, got %s", wordpressVars["database"])
	}
	if wordpressVars["port"] != "" {
		t.Errorf("Expected WordPress port var to be empty, got %s", wordpressVars["port"])
	}
}

func TestSiteType_GetDefaultAttributes(t *testing.T) {
	// Test Laravel defaults with zero downtime
	laravelDefaults := SiteTypeLaravel.GetDefaultAttributes(true)
	if _, ok := laravelDefaults["shared_directories"]; !ok {
		t.Error("Expected Laravel defaults to have shared_directories")
	}
	if _, ok := laravelDefaults["hook_before_making_current"]; !ok {
		t.Error("Expected Laravel defaults to have hook_before_making_current")
	}

	// Test Laravel defaults without zero downtime
	laravelNoZD := SiteTypeLaravel.GetDefaultAttributes(false)
	if _, ok := laravelNoZD["hook_before_updating_repository"]; !ok {
		t.Error("Expected Laravel non-ZD defaults to have hook_before_updating_repository")
	}

	// Test WordPress defaults
	wpDefaults := SiteTypeWordpress.GetDefaultAttributes(true)
	if webFolder, ok := wpDefaults["web_folder"].(string); !ok || webFolder != "/" {
		t.Errorf("Expected WordPress web_folder to be '/', got %v", wpDefaults["web_folder"])
	}

	// Test Static defaults
	staticDefaults := SiteTypeStatic.GetDefaultAttributes(true)
	if _, ok := staticDefaults["hook_before_making_current"]; !ok {
		t.Error("Expected Static defaults to have hook_before_making_current")
	}

	// Test Generic defaults (empty)
	genericDefaults := SiteTypeGeneric.GetDefaultAttributes(true)
	if len(genericDefaults) != 0 {
		t.Errorf("Expected Generic defaults to be empty, got %v", genericDefaults)
	}
}

func TestSiteType_Scan(t *testing.T) {
	var st SiteType

	// Test scanning string
	if err := st.Scan("laravel"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if st != SiteTypeLaravel {
		t.Errorf("Expected SiteTypeLaravel, got %v", st)
	}

	// Test scanning bytes
	if err := st.Scan([]byte("wordpress")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if st != SiteTypeWordpress {
		t.Errorf("Expected SiteTypeWordpress, got %v", st)
	}

	// Test scanning nil
	if err := st.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	// Test scanning invalid type
	if err := st.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestSiteType_Value(t *testing.T) {
	st := SiteTypeLaravel
	v, err := st.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "laravel" {
		t.Errorf("Expected 'laravel', got %v", v)
	}
}

func TestParseSiteType(t *testing.T) {
	tests := []struct {
		input    string
		expected SiteType
		hasError bool
	}{
		{"laravel", SiteTypeLaravel, false},
		{"wordpress", SiteTypeWordpress, false},
		{"static", SiteTypeStatic, false},
		{"generic", SiteTypeGeneric, false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSiteType(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseSiteType() error = %v, wantError %v", err, tt.hasError)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseSiteType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllSiteTypes(t *testing.T) {
	types := AllSiteTypes()
	if len(types) != 4 {
		t.Errorf("Expected 4 site types, got %d", len(types))
	}
}

func TestDeploymentStatus(t *testing.T) {
	tests := []struct {
		status   DeploymentStatus
		label    string
		isValid  bool
		isActive bool
		isComplete bool
	}{
		{DeploymentStatusPending, "Pending", true, true, false},
		{DeploymentStatusQueued, "Queued", true, false, false},
		{DeploymentStatusInstalling, "Installing", true, true, false},
		{DeploymentStatusFinished, "Finished", true, false, true},
		{DeploymentStatusFailed, "Failed", true, false, true},
		{DeploymentStatusTimeout, "Timeout", true, false, true},
		{DeploymentStatus("unknown"), "unknown", false, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Label(); got != tt.label {
				t.Errorf("Label() = %v, want %v", got, tt.label)
			}
			if got := tt.status.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
			if got := tt.status.IsActive(); got != tt.isActive {
				t.Errorf("IsActive() = %v, want %v", got, tt.isActive)
			}
			if got := tt.status.IsComplete(); got != tt.isComplete {
				t.Errorf("IsComplete() = %v, want %v", got, tt.isComplete)
			}
		})
	}
}

func TestDeploymentStatus_Scan(t *testing.T) {
	var ds DeploymentStatus

	if err := ds.Scan("pending"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ds != DeploymentStatusPending {
		t.Errorf("Expected DeploymentStatusPending, got %v", ds)
	}

	if err := ds.Scan([]byte("finished")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ds != DeploymentStatusFinished {
		t.Errorf("Expected DeploymentStatusFinished, got %v", ds)
	}

	if err := ds.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := ds.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestDeploymentStatus_Value(t *testing.T) {
	ds := DeploymentStatusPending
	v, err := ds.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "pending" {
		t.Errorf("Expected 'pending', got %v", v)
	}
}

func TestAllDeploymentStatuses(t *testing.T) {
	statuses := AllDeploymentStatuses()
	if len(statuses) != 6 {
		t.Errorf("Expected 6 deployment statuses, got %d", len(statuses))
	}
}

func TestTlsSetting(t *testing.T) {
	tests := []struct {
		setting   TlsSetting
		label     string
		isValid   bool
		isEnabled bool
		port      int
		protocol  string
	}{
		{TlsSettingAuto, "Auto", true, true, 443, "https"},
		{TlsSettingCustom, "Custom", true, true, 443, "https"},
		{TlsSettingInternal, "Internal", true, true, 443, "https"},
		{TlsSettingOff, "Off", true, false, 80, "http"},
		{TlsSetting("unknown"), "unknown", false, true, 443, "https"},
	}

	for _, tt := range tests {
		t.Run(string(tt.setting), func(t *testing.T) {
			if got := tt.setting.Label(); got != tt.label {
				t.Errorf("Label() = %v, want %v", got, tt.label)
			}
			if got := tt.setting.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
			if got := tt.setting.IsEnabled(); got != tt.isEnabled {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.isEnabled)
			}
			if got := tt.setting.GetPort(); got != tt.port {
				t.Errorf("GetPort() = %v, want %v", got, tt.port)
			}
			if got := tt.setting.GetProtocol(); got != tt.protocol {
				t.Errorf("GetProtocol() = %v, want %v", got, tt.protocol)
			}
		})
	}
}

func TestTlsSetting_Scan(t *testing.T) {
	var ts TlsSetting

	if err := ts.Scan("auto"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ts != TlsSettingAuto {
		t.Errorf("Expected TlsSettingAuto, got %v", ts)
	}

	if err := ts.Scan([]byte("custom")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ts != TlsSettingCustom {
		t.Errorf("Expected TlsSettingCustom, got %v", ts)
	}

	if err := ts.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := ts.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestTlsSetting_Value(t *testing.T) {
	ts := TlsSettingAuto
	v, err := ts.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "auto" {
		t.Errorf("Expected 'auto', got %v", v)
	}
}

func TestAllTlsSettings(t *testing.T) {
	settings := AllTlsSettings()
	if len(settings) != 4 {
		t.Errorf("Expected 4 TLS settings, got %d", len(settings))
	}
}

func TestRedirectMode(t *testing.T) {
	tests := []struct {
		mode       RedirectMode
		statusCode int
		label      string
		isValid    bool
	}{
		{RedirectModePermanent, 301, "Permanent", true},
		{RedirectModeTemporary, 302, "Temporary", true},
		{RedirectMode(99), 302, "Unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := tt.mode.StatusCode(); got != tt.statusCode {
				t.Errorf("StatusCode() = %v, want %v", got, tt.statusCode)
			}
			if got := tt.mode.Label(); got != tt.label {
				t.Errorf("Label() = %v, want %v", got, tt.label)
			}
			if got := tt.mode.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
		})
	}
}

func TestRedirectMode_Scan(t *testing.T) {
	var rm RedirectMode

	if err := rm.Scan(int64(1)); err != nil {
		t.Errorf("Scan int64 failed: %v", err)
	}
	if rm != RedirectModePermanent {
		t.Errorf("Expected RedirectModePermanent, got %v", rm)
	}

	if err := rm.Scan(2); err != nil {
		t.Errorf("Scan int failed: %v", err)
	}
	if rm != RedirectModeTemporary {
		t.Errorf("Expected RedirectModeTemporary, got %v", rm)
	}

	if err := rm.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := rm.Scan("invalid"); err == nil {
		t.Error("Expected error when scanning string")
	}
}

func TestRedirectMode_Value(t *testing.T) {
	rm := RedirectModePermanent
	v, err := rm.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != int64(1) {
		t.Errorf("Expected 1, got %v", v)
	}
}

func TestCommandStatus(t *testing.T) {
	tests := []struct {
		status  CommandStatus
		isValid bool
	}{
		{CommandStatusPending, true},
		{CommandStatusRunning, true},
		{CommandStatusFinished, true},
		{CommandStatusFailed, true},
		{CommandStatusTimeout, true},
		{CommandStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
		})
	}
}

func TestCommandStatus_Scan(t *testing.T) {
	var cs CommandStatus

	if err := cs.Scan("pending"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if cs != CommandStatusPending {
		t.Errorf("Expected CommandStatusPending, got %v", cs)
	}

	if err := cs.Scan([]byte("running")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if cs != CommandStatusRunning {
		t.Errorf("Expected CommandStatusRunning, got %v", cs)
	}

	if err := cs.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := cs.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestQueueStatus(t *testing.T) {
	tests := []struct {
		status  QueueStatus
		isValid bool
	}{
		{QueueStatusPending, true},
		{QueueStatusInstalling, true},
		{QueueStatusActive, true},
		{QueueStatusFailed, true},
		{QueueStatusUninstalling, true},
		{QueueStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
		})
	}
}

func TestQueueStatus_Scan(t *testing.T) {
	var qs QueueStatus

	if err := qs.Scan("pending"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if qs != QueueStatusPending {
		t.Errorf("Expected QueueStatusPending, got %v", qs)
	}

	if err := qs.Scan([]byte("active")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if qs != QueueStatusActive {
		t.Errorf("Expected QueueStatusActive, got %v", qs)
	}

	if err := qs.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := qs.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestCertificateType(t *testing.T) {
	tests := []struct {
		certType CertificateType
		isValid  bool
	}{
		{CertificateTypeAuto, true},
		{CertificateTypeCustom, true},
		{CertificateType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.certType), func(t *testing.T) {
			if got := tt.certType.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
		})
	}
}

func TestCertificateType_Scan(t *testing.T) {
	var ct CertificateType

	if err := ct.Scan("auto"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ct != CertificateTypeAuto {
		t.Errorf("Expected CertificateTypeAuto, got %v", ct)
	}

	if err := ct.Scan([]byte("custom")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ct != CertificateTypeCustom {
		t.Errorf("Expected CertificateTypeCustom, got %v", ct)
	}

	if err := ct.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := ct.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}

func TestSiteStatus(t *testing.T) {
	tests := []struct {
		status  SiteStatus
		isValid bool
	}{
		{SiteStatusPending, true},
		{SiteStatusInstalling, true},
		{SiteStatusActive, true},
		{SiteStatusFailed, true},
		{SiteStatusUninstalling, true},
		{SiteStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
		})
	}
}

func TestSiteStatus_Scan(t *testing.T) {
	var ss SiteStatus

	if err := ss.Scan("active"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ss != SiteStatusActive {
		t.Errorf("Expected SiteStatusActive, got %v", ss)
	}

	if err := ss.Scan([]byte("pending")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ss != SiteStatusPending {
		t.Errorf("Expected SiteStatusPending, got %v", ss)
	}

	if err := ss.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := ss.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}
