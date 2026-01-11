package git

import (
	"testing"
)

func TestGitProviderType_String(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected string
	}{
		{"GitHub", GitProviderGitHub, "github"},
		{"GitLab", GitProviderGitLab, "gitlab"},
		{"Bitbucket", GitProviderBitbucket, "bitbucket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.String(); got != tt.expected {
				t.Errorf("GitProviderType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitProviderType_Label(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected string
	}{
		{"GitHub", GitProviderGitHub, "GitHub"},
		{"GitLab", GitProviderGitLab, "GitLab"},
		{"Bitbucket", GitProviderBitbucket, "Bitbucket"},
		{"Unknown", GitProviderType("unknown"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.Label(); got != tt.expected {
				t.Errorf("GitProviderType.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitProviderType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected bool
	}{
		{"GitHub is valid", GitProviderGitHub, true},
		{"GitLab is valid", GitProviderGitLab, true},
		{"Bitbucket is valid", GitProviderBitbucket, true},
		{"Unknown is invalid", GitProviderType("unknown"), false},
		{"Empty is invalid", GitProviderType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.IsValid(); got != tt.expected {
				t.Errorf("GitProviderType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseGitProviderType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  GitProviderType
		expectErr bool
	}{
		{"github lowercase", "github", GitProviderGitHub, false},
		{"GITHUB uppercase", "GITHUB", GitProviderGitHub, false},
		{"GitHub mixed case", "GitHub", GitProviderGitHub, false},
		{"gitlab lowercase", "gitlab", GitProviderGitLab, false},
		{"bitbucket lowercase", "bitbucket", GitProviderBitbucket, false},
		{"invalid provider", "invalid", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitProviderType(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("ParseGitProviderType() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseGitProviderType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitProviderType_Value(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected string
	}{
		{"GitHub", GitProviderGitHub, "github"},
		{"GitLab", GitProviderGitLab, "gitlab"},
		{"Bitbucket", GitProviderBitbucket, "bitbucket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.provider.Value()
			if err != nil {
				t.Errorf("GitProviderType.Value() error = %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("GitProviderType.Value() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitProviderType_Scan(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expected  GitProviderType
		expectErr bool
	}{
		{"string value", "github", GitProviderGitHub, false},
		{"byte slice", []byte("gitlab"), GitProviderGitLab, false},
		{"nil value", nil, "", false},
		{"int value (unsupported)", 123, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider GitProviderType
			err := provider.Scan(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("GitProviderType.Scan() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr && provider != tt.expected {
				t.Errorf("GitProviderType.Scan() = %v, want %v", provider, tt.expected)
			}
		})
	}
}

func TestAllGitProviders(t *testing.T) {
	providers := AllGitProviders()
	if len(providers) != 3 {
		t.Errorf("AllGitProviders() returned %d providers, want 3", len(providers))
	}

	expected := []GitProviderType{GitProviderGitHub, GitProviderGitLab, GitProviderBitbucket}
	for i, p := range providers {
		if p != expected[i] {
			t.Errorf("AllGitProviders()[%d] = %v, want %v", i, p, expected[i])
		}
	}
}

func TestAccountType_String(t *testing.T) {
	tests := []struct {
		name     string
		accType  AccountType
		expected string
	}{
		{"User", AccountTypeUser, "user"},
		{"Organization", AccountTypeOrganization, "organization"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.accType.String(); got != tt.expected {
				t.Errorf("AccountType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccountType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		accType  AccountType
		expected bool
	}{
		{"User is valid", AccountTypeUser, true},
		{"Organization is valid", AccountTypeOrganization, true},
		{"Unknown is invalid", AccountType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.accType.IsValid(); got != tt.expected {
				t.Errorf("AccountType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccountType_IsOrganization(t *testing.T) {
	tests := []struct {
		name     string
		accType  AccountType
		expected bool
	}{
		{"User is not org", AccountTypeUser, false},
		{"Organization is org", AccountTypeOrganization, true},
		{"ORGANIZATION uppercase", AccountType("ORGANIZATION"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.accType.IsOrganization(); got != tt.expected {
				t.Errorf("AccountType.IsOrganization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccountType_IsUser(t *testing.T) {
	tests := []struct {
		name     string
		accType  AccountType
		expected bool
	}{
		{"User is user", AccountTypeUser, true},
		{"Organization is not user", AccountTypeOrganization, false},
		{"USER uppercase", AccountType("USER"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.accType.IsUser(); got != tt.expected {
				t.Errorf("AccountType.IsUser() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseAccountType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected AccountType
	}{
		{"user lowercase", "user", AccountTypeUser},
		{"organization lowercase", "organization", AccountTypeOrganization},
		{"org shorthand", "org", AccountTypeOrganization},
		{"USER uppercase", "USER", AccountTypeUser},
		{"ORGANIZATION uppercase", "ORGANIZATION", AccountTypeOrganization},
		{"unknown defaults to user", "unknown", AccountTypeUser},
		{"empty defaults to user", "", AccountTypeUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseAccountType(tt.input); got != tt.expected {
				t.Errorf("ParseAccountType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRepositorySelection_String(t *testing.T) {
	tests := []struct {
		name     string
		sel      RepositorySelection
		expected string
	}{
		{"All", RepositorySelectionAll, "all"},
		{"Selected", RepositorySelectionSelected, "selected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sel.String(); got != tt.expected {
				t.Errorf("RepositorySelection.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRepositorySelection_IsAll(t *testing.T) {
	if !RepositorySelectionAll.IsAll() {
		t.Error("RepositorySelectionAll.IsAll() should be true")
	}
	if RepositorySelectionSelected.IsAll() {
		t.Error("RepositorySelectionSelected.IsAll() should be false")
	}
}

func TestRepositorySelection_IsSelected(t *testing.T) {
	if !RepositorySelectionSelected.IsSelected() {
		t.Error("RepositorySelectionSelected.IsSelected() should be true")
	}
	if RepositorySelectionAll.IsSelected() {
		t.Error("RepositorySelectionAll.IsSelected() should be false")
	}
}

func TestWebhookEventType_String(t *testing.T) {
	tests := []struct {
		name     string
		event    WebhookEventType
		expected string
	}{
		{"Push", WebhookEventPush, "push"},
		{"PullRequest", WebhookEventPullRequest, "pull_request"},
		{"Installation", WebhookEventInstallation, "installation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.String(); got != tt.expected {
				t.Errorf("WebhookEventType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
