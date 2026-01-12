package enums

import (
	"database/sql/driver"
	"fmt"
)

// ProvisionStep represents a step in the server provisioning process
type ProvisionStep string

const (
	ProvisionStepAptUpdateUpgrade         ProvisionStep = "apt_update_upgrade"
	ProvisionStepConfigureFirewall        ProvisionStep = "configure_firewall"
	ProvisionStepConfigureSwap            ProvisionStep = "configure_swap"
	ProvisionStepInstallEssentialPackages ProvisionStep = "install_essential_packages"
	ProvisionStepSetupDefaultUser         ProvisionStep = "setup_default_user"
	ProvisionStepSetupRoot                ProvisionStep = "setup_root"
	ProvisionStepSetupUnattendedUpgrades  ProvisionStep = "setup_unattended_upgrades"
	ProvisionStepSSHSecurity              ProvisionStep = "ssh_security"
)

func (p ProvisionStep) String() string {
	return string(p)
}

// TemplateName returns the template path for this provision step
func (p ProvisionStep) TemplateName() string {
	templateNames := map[ProvisionStep]string{
		ProvisionStepAptUpdateUpgrade:         "provision/apt-update-upgrade.sh",
		ProvisionStepConfigureFirewall:        "provision/configure-firewall.sh",
		ProvisionStepConfigureSwap:            "provision/configure-swap.sh",
		ProvisionStepInstallEssentialPackages: "provision/install-essential-packages.sh",
		ProvisionStepSetupDefaultUser:         "provision/setup-default-user.sh",
		ProvisionStepSetupRoot:                "provision/setup-root.sh",
		ProvisionStepSetupUnattendedUpgrades:  "provision/setup-unattended-upgrades.sh",
		ProvisionStepSSHSecurity:              "provision/ssh-security.sh",
	}

	if name, ok := templateNames[p]; ok {
		return name
	}

	return "provision/" + string(p) + ".sh"
}

// RequiresData returns true if this step requires template data
func (p ProvisionStep) RequiresData() bool {
	switch p {
	case ProvisionStepConfigureSwap, ProvisionStepConfigureFirewall,
		ProvisionStepSetupRoot, ProvisionStepSetupDefaultUser:
		return true
	default:
		return false
	}
}

// Description returns a human-readable description of this step
func (p ProvisionStep) Description() string {
	descriptions := map[ProvisionStep]string{
		ProvisionStepAptUpdateUpgrade:         "Update package lists and installed system packages",
		ProvisionStepConfigureFirewall:        "Configure the firewall with the default rules (SSH, HTTP, HTTPS)",
		ProvisionStepConfigureSwap:            "Configure a swap file so the server can handle more memory-intensive tasks",
		ProvisionStepInstallEssentialPackages: "Install essential packages (curl, git, wget, etc.)",
		ProvisionStepSetupDefaultUser:         "Create a default user account",
		ProvisionStepSetupRoot:                "Configure the root user",
		ProvisionStepSetupUnattendedUpgrades:  "Configure unattended upgrades to keep the server up to date automatically",
		ProvisionStepSSHSecurity:              "Enhance SSH security by disabling password authentication and root login",
	}
	if desc, ok := descriptions[p]; ok {
		return desc
	}
	return string(p)
}

// Label returns a short label for UI display
func (p ProvisionStep) Label() string {
	labels := map[ProvisionStep]string{
		ProvisionStepAptUpdateUpgrade:         "Apt Update & Upgrade",
		ProvisionStepConfigureFirewall:        "Configure Firewall",
		ProvisionStepConfigureSwap:            "Configure Swap",
		ProvisionStepInstallEssentialPackages: "Install Essential Packages",
		ProvisionStepSetupDefaultUser:         "Setup Default User",
		ProvisionStepSetupRoot:                "Setup Root",
		ProvisionStepSetupUnattendedUpgrades:  "Setup Unattended Upgrades",
		ProvisionStepSSHSecurity:              "SSH Security",
	}
	if label, ok := labels[p]; ok {
		return label
	}
	return string(p)
}

// IsValid returns true if this is a valid provision step
func (p ProvisionStep) IsValid() bool {
	switch p {
	case ProvisionStepAptUpdateUpgrade, ProvisionStepConfigureFirewall,
		ProvisionStepConfigureSwap, ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupDefaultUser, ProvisionStepSetupRoot,
		ProvisionStepSetupUnattendedUpgrades, ProvisionStepSSHSecurity:
		return true
	}
	return false
}

// ForFreshServer returns the provision steps in order for a fresh server
func ForFreshServer() []ProvisionStep {
	return []ProvisionStep{
		ProvisionStepConfigureSwap,
		ProvisionStepConfigureFirewall,
		ProvisionStepAptUpdateUpgrade,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupUnattendedUpgrades,
		ProvisionStepSetupRoot,
		ProvisionStepSSHSecurity,
		ProvisionStepSetupDefaultUser,
	}
}

// AllProvisionSteps returns all provision steps
func AllProvisionSteps() []ProvisionStep {
	return []ProvisionStep{
		ProvisionStepAptUpdateUpgrade,
		ProvisionStepConfigureFirewall,
		ProvisionStepConfigureSwap,
		ProvisionStepInstallEssentialPackages,
		ProvisionStepSetupDefaultUser,
		ProvisionStepSetupRoot,
		ProvisionStepSetupUnattendedUpgrades,
		ProvisionStepSSHSecurity,
	}
}

// ParseProvisionStep parses a string into a ProvisionStep
func ParseProvisionStep(s string) (ProvisionStep, error) {
	step := ProvisionStep(s)
	if !step.IsValid() {
		return "", fmt.Errorf("invalid provision step: %s", s)
	}
	return step, nil
}

func (p *ProvisionStep) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*p = ProvisionStep(v)
	case string:
		*p = ProvisionStep(v)
	default:
		return fmt.Errorf("cannot scan type %T into ProvisionStep", value)
	}

	return nil
}

func (p ProvisionStep) Value() (driver.Value, error) {
	return string(p), nil
}
