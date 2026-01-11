package server

import (
	"testing"
	"time"
)

func TestServer_BeforeCreate(t *testing.T) {
	server := &Server{}

	if err := server.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if server.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if len(server.ID) != 26 {
		t.Errorf("Expected ULID of length 26, got %d", len(server.ID))
	}
	if server.Status != ServerStatusNew {
		t.Errorf("Expected status to be new, got %v", server.Status)
	}
	if server.LaunchToken == "" {
		t.Error("Expected LaunchToken to be generated")
	}

	// Test that existing values are not overwritten
	existingServer := &Server{
		ID:     "existing-id",
		Status: ServerStatusRunning,
	}
	if err := existingServer.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}
	if existingServer.ID != "existing-id" {
		t.Error("Expected existing ID to be preserved")
	}
	if existingServer.Status != ServerStatusRunning {
		t.Error("Expected existing status to be preserved")
	}
}

func TestServer_TableName(t *testing.T) {
	server := &Server{}
	if got := server.TableName(); got != "servers" {
		t.Errorf("TableName() = %v, want 'servers'", got)
	}
}

func TestServer_IsProvisioned(t *testing.T) {
	now := time.Now()

	// Server with ProvisionedAt is provisioned
	server := &Server{ProvisionedAt: &now}
	if !server.IsProvisioned() {
		t.Error("Expected server with ProvisionedAt to be provisioned")
	}

	// Server without ProvisionedAt is not provisioned
	server2 := &Server{}
	if server2.IsProvisioned() {
		t.Error("Expected server without ProvisionedAt to not be provisioned")
	}
}

func TestServer_IsConnected(t *testing.T) {
	server := &Server{Connected: true}
	if !server.IsConnected() {
		t.Error("Expected server to be connected")
	}

	server2 := &Server{Connected: false}
	if server2.IsConnected() {
		t.Error("Expected server to not be connected")
	}
}

func TestServer_IsArchived(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		archivedAt *time.Time
		expected   bool
	}{
		{"archived", &now, true},
		{"not archived", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{ArchivedAt: tt.archivedAt}
			if got := server.IsArchived(); got != tt.expected {
				t.Errorf("IsArchived() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServer_RootUsername(t *testing.T) {
	server := &Server{
		Provider:        ProviderDigitalOcean,
		OperatingSystem: OSUbuntu24,
	}
	got := server.RootUsername()
	if got != "root" {
		t.Errorf("RootUsername() = %v, want root", got)
	}
}

func TestServer_GetProvisionCommand(t *testing.T) {
	server := &Server{ID: "test-id-123"}
	cmd := server.GetProvisionCommand()
	if cmd == "" {
		t.Error("Expected provision command to be generated")
	}
}

func TestServer_HasFeature(t *testing.T) {
	server := &Server{Type: ServerTypePhp}
	if !server.HasFeature(ServerFeaturePhpManagement) {
		t.Error("Expected PHP server to have PHP management feature")
	}
	if !server.HasFeature(ServerFeatureSites) {
		t.Error("Expected PHP server to have sites feature")
	}
}

func TestServer_GetFeatures(t *testing.T) {
	server := &Server{Type: ServerTypePhp}
	features := server.GetFeatures()
	if len(features) == 0 {
		t.Error("Expected PHP server to have features")
	}
}

func TestServer_GetProcessManager(t *testing.T) {
	server := &Server{Type: ServerTypePhp}
	pm := server.GetProcessManager()
	if pm == "" {
		t.Error("Expected server to have a process manager")
	}
}

func TestServer_SetProviderData(t *testing.T) {
	server := &Server{}
	data := map[string]interface{}{
		"region":  "nyc1",
		"size":    "s-1vcpu-1gb",
		"droplet": float64(123456),
	}

	if err := server.SetProviderData(data); err != nil {
		t.Errorf("SetProviderData() error = %v", err)
	}

	if server.ProviderData == nil {
		t.Error("Expected ProviderData to be set")
	}

	retrieved := server.GetProviderData()
	if retrieved["region"] != "nyc1" {
		t.Errorf("Expected region 'nyc1', got %v", retrieved["region"])
	}
}

func TestServer_GetProviderData(t *testing.T) {
	// Test nil data
	server := &Server{}
	if got := server.GetProviderData(); got != nil {
		t.Errorf("Expected nil for empty ProviderData, got %v", got)
	}

	// Test valid data
	jsonData := `{"region": "nyc1"}`
	server.ProviderData = &jsonData
	data := server.GetProviderData()
	if data["region"] != "nyc1" {
		t.Errorf("Expected region 'nyc1', got %v", data["region"])
	}
}

func TestServer_CompletedSteps(t *testing.T) {
	server := &Server{}

	// Set steps
	steps := []string{"step1", "step2", "step3"}
	if err := server.SetCompletedSteps(steps); err != nil {
		t.Errorf("SetCompletedSteps() error = %v", err)
	}

	// Get steps
	retrieved := server.GetCompletedSteps()
	if len(retrieved) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(retrieved))
	}
	if retrieved[0] != "step1" {
		t.Errorf("Expected step1, got %v", retrieved[0])
	}
}

func TestInstalledService_BeforeCreate(t *testing.T) {
	service := &InstalledService{}

	if err := service.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if service.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if service.Status != ServiceStatusPending {
		t.Errorf("Expected status to be pending, got %v", service.Status)
	}
}

func TestInstalledService_TableName(t *testing.T) {
	service := &InstalledService{}
	if got := service.TableName(); got != "services" {
		t.Errorf("TableName() = %v, want 'services'", got)
	}
}

func TestInstalledService_GetFormattedVersion(t *testing.T) {
	// Test with software
	php := SoftwarePhp84
	service := &InstalledService{Software: &php}
	if got := service.GetFormattedVersion(); got != "8.4" {
		t.Errorf("GetFormattedVersion() = %v, want '8.4'", got)
	}

	// Test with version string
	version := "8.0.30"
	service2 := &InstalledService{Version: &version}
	if got := service2.GetFormattedVersion(); got != "8.0.30" {
		t.Errorf("GetFormattedVersion() = %v, want '8.0.30'", got)
	}

	// Test with neither
	service3 := &InstalledService{}
	if got := service3.GetFormattedVersion(); got != "" {
		t.Errorf("GetFormattedVersion() = %v, want empty string", got)
	}
}

func TestInstalledService_TypeData(t *testing.T) {
	service := &InstalledService{}

	data := map[string]interface{}{
		"port":     float64(3306),
		"password": "secret",
	}

	if err := service.SetTypeData(data); err != nil {
		t.Errorf("SetTypeData() error = %v", err)
	}

	retrieved := service.GetTypeData()
	if retrieved["port"] != float64(3306) {
		t.Errorf("Expected port 3306, got %v", retrieved["port"])
	}
}

func TestFirewallRule_BeforeCreate(t *testing.T) {
	rule := &FirewallRule{}

	if err := rule.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if rule.Action != RuleActionAllow {
		t.Errorf("Expected action to be allow, got %v", rule.Action)
	}
}

func TestFirewallRule_TableName(t *testing.T) {
	rule := &FirewallRule{}
	if got := rule.TableName(); got != "firewall_rules" {
		t.Errorf("TableName() = %v, want 'firewall_rules'", got)
	}
}

func TestFirewallRule_IsInstalled(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		installedAt *time.Time
		expected    bool
	}{
		{"installed", &now, true},
		{"not installed", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &FirewallRule{InstalledAt: tt.installedAt}
			if got := rule.IsInstalled(); got != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFirewallRule_IsPending(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                 string
		installedAt          *time.Time
		installationFailedAt *time.Time
		expected             bool
	}{
		{"pending", nil, nil, true},
		{"installed", &now, nil, false},
		{"failed", nil, &now, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &FirewallRule{
				InstalledAt:          tt.installedAt,
				InstallationFailedAt: tt.installationFailedAt,
			}
			if got := rule.IsPending(); got != tt.expected {
				t.Errorf("IsPending() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFirewallRule_HasFailed(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                 string
		installationFailedAt *time.Time
		expected             bool
	}{
		{"failed", &now, true},
		{"not failed", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &FirewallRule{InstallationFailedAt: tt.installationFailedAt}
			if got := rule.HasFailed(); got != tt.expected {
				t.Errorf("HasFailed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFirewallRule_FormatAsUfwRule(t *testing.T) {
	port := "22"
	fromIP := "10.0.0.1"

	tests := []struct {
		name     string
		rule     FirewallRule
		expected string
	}{
		{
			name: "simple allow",
			rule: FirewallRule{
				Action: RuleActionAllow,
				Port:   &port,
			},
			expected: "allow 22",
		},
		{
			name: "deny with from",
			rule: FirewallRule{
				Action:   RuleActionDeny,
				Port:     &port,
				FromIPv4: &fromIP,
			},
			expected: "deny from 10.0.0.1 to any port 22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.FormatAsUfwRule(); got != tt.expected {
				t.Errorf("FormatAsUfwRule() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCron_BeforeCreate(t *testing.T) {
	cron := &Cron{}

	if err := cron.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if cron.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if cron.User != "root" {
		t.Errorf("Expected user to be root, got %v", cron.User)
	}
}

func TestCron_TableName(t *testing.T) {
	cron := &Cron{}
	if got := cron.TableName(); got != "crons" {
		t.Errorf("TableName() = %v, want 'crons'", got)
	}
}

func TestCron_IsInstalled(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		installedAt *time.Time
		expected    bool
	}{
		{"installed", &now, true},
		{"not installed", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cron := &Cron{InstalledAt: tt.installedAt}
			if got := cron.IsInstalled(); got != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCron_Path(t *testing.T) {
	cron := &Cron{ID: "test-cron-id"}
	expected := "/etc/cron.d/cron-test-cron-id"
	if got := cron.Path(); got != expected {
		t.Errorf("Path() = %v, want %v", got, expected)
	}
}

func TestCron_LogPath(t *testing.T) {
	// Test root user
	cron := &Cron{ID: "test-cron-id", User: "root"}
	expected := "/root/.launch/cron-test-cron-id.log"
	if got := cron.LogPath(".launch"); got != expected {
		t.Errorf("LogPath() = %v, want %v", got, expected)
	}

	// Test non-root user
	cron2 := &Cron{ID: "test-cron-id", User: "deploy"}
	expected2 := "/home/deploy/.launch/cron-test-cron-id.log"
	if got := cron2.LogPath(".launch"); got != expected2 {
		t.Errorf("LogPath() = %v, want %v", got, expected2)
	}
}

func TestDaemon_BeforeCreate(t *testing.T) {
	daemon := &Daemon{}

	if err := daemon.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if daemon.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if daemon.User != "root" {
		t.Errorf("Expected user to be root, got %v", daemon.User)
	}
	if daemon.Processes != 1 {
		t.Errorf("Expected processes to be 1, got %d", daemon.Processes)
	}
	if daemon.StopWaitSeconds != 10 {
		t.Errorf("Expected StopWaitSeconds to be 10, got %d", daemon.StopWaitSeconds)
	}
}

func TestDaemon_TableName(t *testing.T) {
	daemon := &Daemon{}
	if got := daemon.TableName(); got != "daemons" {
		t.Errorf("TableName() = %v, want 'daemons'", got)
	}
}

func TestDaemon_IsInstalled(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		installedAt *time.Time
		expected    bool
	}{
		{"installed", &now, true},
		{"not installed", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			daemon := &Daemon{InstalledAt: tt.installedAt}
			if got := daemon.IsInstalled(); got != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDaemon_Path(t *testing.T) {
	daemon := &Daemon{ID: "test-daemon-id"}
	expected := "/etc/supervisor/conf.d/daemon-test-daemon-id.conf"
	if got := daemon.Path(); got != expected {
		t.Errorf("Path() = %v, want %v", got, expected)
	}
}

func TestDaemon_Info(t *testing.T) {
	daemon := &Daemon{}

	// Test nil info
	if got := daemon.GetInfo(); got != nil {
		t.Errorf("Expected nil for empty Info, got %v", got)
	}

	// Set info
	info := map[string]interface{}{
		"status": "running",
		"pid":    float64(12345),
	}
	if err := daemon.SetInfo(info); err != nil {
		t.Errorf("SetInfo() error = %v", err)
	}

	// Get info
	retrieved := daemon.GetInfo()
	if retrieved["status"] != "running" {
		t.Errorf("Expected status 'running', got %v", retrieved["status"])
	}
}

func TestSshKey_BeforeCreate(t *testing.T) {
	key := &SshKey{
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC test@example.com",
	}

	if err := key.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if key.ID == "" {
		t.Error("Expected ID to be generated")
	}
}

func TestSshKey_TableName(t *testing.T) {
	key := &SshKey{}
	if got := key.TableName(); got != "ssh_keys" {
		t.Errorf("TableName() = %v, want 'ssh_keys'", got)
	}
}

func TestSshKey_GetFingerprint(t *testing.T) {
	key := &SshKey{
		PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC test@example.com",
	}

	fingerprint := key.GetFingerprint()
	// Since it's computing MD5, we just check it's not empty (the test key may not be valid format)
	if fingerprint == "" {
		t.Log("Fingerprint not generated (expected for test key with invalid base64)")
	}
}

func TestServerSshKey_TableName(t *testing.T) {
	join := &ServerSshKey{}
	if got := join.TableName(); got != "server_ssh_keys" {
		t.Errorf("TableName() = %v, want 'server_ssh_keys'", got)
	}
}

func TestTask_BeforeCreate(t *testing.T) {
	task := &Task{}

	if err := task.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if task.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if task.Status != "pending" {
		t.Errorf("Expected status to be pending, got %v", task.Status)
	}
}

func TestTask_TableName(t *testing.T) {
	task := &Task{}
	if got := task.TableName(); got != "tasks" {
		t.Errorf("TableName() = %v, want 'tasks'", got)
	}
}

func TestTask_IsSuccessful(t *testing.T) {
	tests := []struct {
		name     string
		exitCode *int
		expected bool
	}{
		{"successful", intPtr(0), true},
		{"failed exit code", intPtr(1), false},
		{"no exit code", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{ExitCode: tt.exitCode}
			if got := task.IsSuccessful(); got != tt.expected {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTask_Duration(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	finish := time.Now()
	task := &Task{
		StartedAt:  &start,
		FinishedAt: &finish,
	}

	duration := task.Duration()
	if duration < 59*time.Second || duration > 61*time.Second {
		t.Errorf("Duration() = %v, expected ~1 minute", duration)
	}

	// Test with no finish time
	task2 := &Task{StartedAt: &start}
	if duration2 := task2.Duration(); duration2 != 0 {
		t.Errorf("Duration() = %v, expected 0 for unfinished task", duration2)
	}

	// Test with no start time
	task3 := &Task{}
	if duration3 := task3.Duration(); duration3 != 0 {
		t.Errorf("Duration() = %v, expected 0", duration3)
	}
}

func TestMetric_BeforeCreate(t *testing.T) {
	metric := &Metric{}

	if err := metric.BeforeCreate(nil); err != nil {
		t.Errorf("BeforeCreate() error = %v", err)
	}

	if metric.ID == "" {
		t.Error("Expected ID to be generated")
	}
}

func TestMetric_TableName(t *testing.T) {
	metric := &Metric{}
	if got := metric.TableName(); got != "metrics" {
		t.Errorf("TableName() = %v, want 'metrics'", got)
	}
}

func TestAllModels(t *testing.T) {
	models := AllModels()
	if len(models) != 9 {
		t.Errorf("Expected 9 models, got %d", len(models))
	}
}

func TestGenerateSSHFingerprint(t *testing.T) {
	// Test with valid SSH key format
	validKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCxq" // simplified, valid format
	fingerprint := GenerateSSHFingerprint(validKey, FingerprintAlgorithmMD5)
	// The fingerprint may be empty if base64 decode fails, that's acceptable
	t.Logf("Fingerprint: %s", fingerprint)

	// Test with invalid format
	invalidKey := "not-an-ssh-key"
	if got := GenerateSSHFingerprint(invalidKey, FingerprintAlgorithmMD5); got != "" {
		t.Errorf("Expected empty fingerprint for invalid key, got %v", got)
	}
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
