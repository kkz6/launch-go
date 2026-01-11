package site

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
)

func TestSiteType_String(t *testing.T) {
	tests := []struct {
		siteType enums.SiteType
		expected string
	}{
		{enums.SiteTypeLaravel, "laravel"},
		{enums.SiteTypeWordpress, "wordpress"},
		{enums.SiteTypeStatic, "static"},
		{enums.SiteTypeGeneric, "generic"},
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
		siteType enums.SiteType
		expected string
	}{
		{enums.SiteTypeLaravel, "Laravel"},
		{enums.SiteTypeWordpress, "Wordpress"},
		{enums.SiteTypeStatic, "Static"},
		{enums.SiteTypeGeneric, "Generic"},
		{enums.SiteType("unknown"), "unknown"},
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
		siteType enums.SiteType
		expected bool
	}{
		{enums.SiteTypeLaravel, true},
		{enums.SiteTypeWordpress, true},
		{enums.SiteTypeStatic, true},
		{enums.SiteTypeGeneric, true},
		{enums.SiteType("unknown"), false},
		{enums.SiteType(""), false},
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
		siteType enums.SiteType
		expected bool
	}{
		{enums.SiteTypeLaravel, true},
		{enums.SiteTypeWordpress, true},
		{enums.SiteTypeStatic, false},
		{enums.SiteTypeGeneric, false},
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
		siteType enums.SiteType
		expected bool
	}{
		{enums.SiteTypeLaravel, true},
		{enums.SiteTypeWordpress, false},
		{enums.SiteTypeStatic, true},
		{enums.SiteTypeGeneric, true},
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
	laravelVars := enums.SiteTypeLaravel.GetDatabaseEnvVarNames()
	if laravelVars["database"] != "DB_DATABASE" {
		t.Errorf("Expected Laravel database var to be DB_DATABASE, got %s", laravelVars["database"])
	}
	if laravelVars["port"] != "DB_PORT" {
		t.Errorf("Expected Laravel port var to be DB_PORT, got %s", laravelVars["port"])
	}

	wordpressVars := enums.SiteTypeWordpress.GetDatabaseEnvVarNames()
	if wordpressVars["database"] != "DB_NAME" {
		t.Errorf("Expected WordPress database var to be DB_NAME, got %s", wordpressVars["database"])
	}
	if wordpressVars["port"] != "" {
		t.Errorf("Expected WordPress port var to be empty, got %s", wordpressVars["port"])
	}
}

func TestSiteType_GetDefaultAttributes(t *testing.T) {
	// Test Laravel defaults with zero downtime
	laravelDefaults := enums.SiteTypeLaravel.GetDefaultAttributes(true)
	if _, ok := laravelDefaults["shared_directories"]; !ok {
		t.Error("Expected Laravel defaults to have shared_directories")
	}
	if _, ok := laravelDefaults["hook_before_making_current"]; !ok {
		t.Error("Expected Laravel defaults to have hook_before_making_current")
	}

	// Test Laravel defaults without zero downtime
	laravelNoZD := enums.SiteTypeLaravel.GetDefaultAttributes(false)
	if _, ok := laravelNoZD["hook_before_updating_repository"]; !ok {
		t.Error("Expected Laravel non-ZD defaults to have hook_before_updating_repository")
	}

	// Test WordPress defaults
	wpDefaults := enums.SiteTypeWordpress.GetDefaultAttributes(true)
	if webFolder, ok := wpDefaults["web_folder"].(string); !ok || webFolder != "/" {
		t.Errorf("Expected WordPress web_folder to be '/', got %v", wpDefaults["web_folder"])
	}

	// Test Static defaults
	staticDefaults := enums.SiteTypeStatic.GetDefaultAttributes(true)
	if _, ok := staticDefaults["hook_before_making_current"]; !ok {
		t.Error("Expected Static defaults to have hook_before_making_current")
	}

	// Test Generic defaults (empty)
	genericDefaults := enums.SiteTypeGeneric.GetDefaultAttributes(true)
	if len(genericDefaults) != 0 {
		t.Errorf("Expected Generic defaults to be empty, got %v", genericDefaults)
	}
}

func TestSiteType_Scan(t *testing.T) {
	var st enums.SiteType

	// Test scanning string
	if err := st.Scan("laravel"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if st != enums.SiteTypeLaravel {
		t.Errorf("Expected SiteTypeLaravel, got %v", st)
	}

	// Test scanning bytes
	if err := st.Scan([]byte("wordpress")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if st != enums.SiteTypeWordpress {
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
	st := enums.SiteTypeLaravel
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
		expected enums.SiteType
		hasError bool
	}{
		{"laravel", enums.SiteTypeLaravel, false},
		{"wordpress", enums.SiteTypeWordpress, false},
		{"static", enums.SiteTypeStatic, false},
		{"generic", enums.SiteTypeGeneric, false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := enums.ParseSiteType(tt.input)
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
	types := enums.AllSiteTypes()
	if len(types) != 4 {
		t.Errorf("Expected 4 site types, got %d", len(types))
	}
}

func TestDeploymentStatus(t *testing.T) {
	tests := []struct {
		status     enums.DeploymentStatus
		label      string
		isValid    bool
		isActive   bool
		isComplete bool
	}{
		{enums.DeploymentStatusPending, "Pending", true, true, false},
		{enums.DeploymentStatusQueued, "Queued", true, false, false},
		{enums.DeploymentStatusInstalling, "Installing", true, true, false},
		{enums.DeploymentStatusFinished, "Finished", true, false, true},
		{enums.DeploymentStatusFailed, "Failed", true, false, true},
		{enums.DeploymentStatusTimeout, "Timeout", true, false, true},
		{enums.DeploymentStatus("unknown"), "unknown", false, false, false},
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
	var ds enums.DeploymentStatus

	if err := ds.Scan("pending"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ds != enums.DeploymentStatusPending {
		t.Errorf("Expected DeploymentStatusPending, got %v", ds)
	}

	if err := ds.Scan([]byte("finished")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ds != enums.DeploymentStatusFinished {
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
	ds := enums.DeploymentStatusPending
	v, err := ds.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "pending" {
		t.Errorf("Expected 'pending', got %v", v)
	}
}

func TestAllDeploymentStatuses(t *testing.T) {
	statuses := enums.AllDeploymentStatuses()
	if len(statuses) != 6 {
		t.Errorf("Expected 6 deployment statuses, got %d", len(statuses))
	}
}

func TestTlsSetting(t *testing.T) {
	tests := []struct {
		setting   enums.TlsSetting
		label     string
		isValid   bool
		isEnabled bool
		port      int
		protocol  string
	}{
		{enums.TlsSettingAuto, "Auto", true, true, 443, "https"},
		{enums.TlsSettingCustom, "Custom", true, true, 443, "https"},
		{enums.TlsSettingInternal, "Internal", true, true, 443, "https"},
		{enums.TlsSettingOff, "Off", true, false, 80, "http"},
		{enums.TlsSetting("unknown"), "unknown", false, true, 443, "https"},
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
	var ts enums.TlsSetting

	if err := ts.Scan("auto"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ts != enums.TlsSettingAuto {
		t.Errorf("Expected TlsSettingAuto, got %v", ts)
	}

	if err := ts.Scan([]byte("custom")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ts != enums.TlsSettingCustom {
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
	ts := enums.TlsSettingAuto
	v, err := ts.Value()
	if err != nil {
		t.Errorf("Value failed: %v", err)
	}
	if v != "auto" {
		t.Errorf("Expected 'auto', got %v", v)
	}
}

func TestAllTlsSettings(t *testing.T) {
	settings := enums.AllTlsSettings()
	if len(settings) != 4 {
		t.Errorf("Expected 4 TLS settings, got %d", len(settings))
	}
}

func TestRedirectMode(t *testing.T) {
	tests := []struct {
		mode       enums.RedirectMode
		statusCode int
		label      string
		isValid    bool
	}{
		{enums.RedirectModePermanent, 301, "Permanent", true},
		{enums.RedirectModeTemporary, 302, "Temporary", true},
		{enums.RedirectMode(99), 302, "Unknown", false},
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
	var rm enums.RedirectMode

	if err := rm.Scan(int64(1)); err != nil {
		t.Errorf("Scan int64 failed: %v", err)
	}
	if rm != enums.RedirectModePermanent {
		t.Errorf("Expected RedirectModePermanent, got %v", rm)
	}

	if err := rm.Scan(2); err != nil {
		t.Errorf("Scan int failed: %v", err)
	}
	if rm != enums.RedirectModeTemporary {
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
	rm := enums.RedirectModePermanent
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
		status  enums.CommandStatus
		isValid bool
	}{
		{enums.CommandStatusPending, true},
		{enums.CommandStatusRunning, true},
		{enums.CommandStatusFinished, true},
		{enums.CommandStatusFailed, true},
		{enums.CommandStatusTimeout, true},
		{enums.CommandStatus("unknown"), false},
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
	var cs enums.CommandStatus

	if err := cs.Scan("pending"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if cs != enums.CommandStatusPending {
		t.Errorf("Expected CommandStatusPending, got %v", cs)
	}

	if err := cs.Scan([]byte("running")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if cs != enums.CommandStatusRunning {
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
		status  enums.QueueStatus
		isValid bool
	}{
		{enums.QueueStatusPending, true},
		{enums.QueueStatusInstalling, true},
		{enums.QueueStatusActive, true},
		{enums.QueueStatusFailed, true},
		{enums.QueueStatusUninstalling, true},
		{enums.QueueStatus("unknown"), false},
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
	var qs enums.QueueStatus

	if err := qs.Scan("pending"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if qs != enums.QueueStatusPending {
		t.Errorf("Expected QueueStatusPending, got %v", qs)
	}

	if err := qs.Scan([]byte("active")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if qs != enums.QueueStatusActive {
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
		certType enums.CertificateType
		isValid  bool
	}{
		{enums.CertificateTypeAuto, true},
		{enums.CertificateTypeCustom, true},
		{enums.CertificateType("unknown"), false},
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
	var ct enums.CertificateType

	if err := ct.Scan("auto"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ct != enums.CertificateTypeAuto {
		t.Errorf("Expected CertificateTypeAuto, got %v", ct)
	}

	if err := ct.Scan([]byte("custom")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ct != enums.CertificateTypeCustom {
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
		status  enums.SiteStatus
		isValid bool
	}{
		{enums.SiteStatusPending, true},
		{enums.SiteStatusInstalling, true},
		{enums.SiteStatusActive, true},
		{enums.SiteStatusFailed, true},
		{enums.SiteStatusUninstalling, true},
		{enums.SiteStatus("unknown"), false},
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
	var ss enums.SiteStatus

	if err := ss.Scan("active"); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if ss != enums.SiteStatusActive {
		t.Errorf("Expected SiteStatusActive, got %v", ss)
	}

	if err := ss.Scan([]byte("pending")); err != nil {
		t.Errorf("Scan bytes failed: %v", err)
	}
	if ss != enums.SiteStatusPending {
		t.Errorf("Expected SiteStatusPending, got %v", ss)
	}

	if err := ss.Scan(nil); err != nil {
		t.Errorf("Scan nil failed: %v", err)
	}

	if err := ss.Scan(123); err == nil {
		t.Error("Expected error when scanning int")
	}
}
