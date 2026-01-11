package enums

import (
	"database/sql/driver"
	"fmt"
)

// ServerProvider represents cloud providers
type ServerProvider string

const (
	ProviderDigitalOcean ServerProvider = "digitalocean"
	ProviderHetzner      ServerProvider = "hetzner"
	ProviderLinode       ServerProvider = "linode"
	ProviderVultr        ServerProvider = "vultr"
	ProviderAWS          ServerProvider = "aws"
	ProviderCustom       ServerProvider = "custom_server"
)

func (p ServerProvider) String() string {
	return string(p)
}

func (p ServerProvider) Label() string {
	labels := map[ServerProvider]string{
		ProviderDigitalOcean: "DigitalOcean",
		ProviderHetzner:      "Hetzner Cloud",
		ProviderLinode:       "Linode",
		ProviderVultr:        "Vultr",
		ProviderAWS:          "AWS",
		ProviderCustom:       "Custom",
	}
	if label, ok := labels[p]; ok {
		return label
	}

	return "Unknown"
}

func (p ServerProvider) IsValid() bool {
	switch p {
	case ProviderDigitalOcean, ProviderHetzner, ProviderLinode,
		ProviderVultr, ProviderAWS, ProviderCustom:
		return true
	}

	return false
}

func (p ServerProvider) IsCloud() bool {
	return p != ProviderCustom
}

func (p ServerProvider) GetDefaultUsername(os OperatingSystem) string {
	switch p {
	case ProviderAWS:
		return "ubuntu"
	default:
		return "root"
	}
}

func (p *ServerProvider) Scan(value interface{}) error {
	if value == nil {
		*p = ProviderCustom
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*p = ServerProvider(v)
	case string:
		*p = ServerProvider(v)
	default:
		return fmt.Errorf("cannot scan type %T into ServerProvider", value)
	}

	return nil
}

func (p ServerProvider) Value() (driver.Value, error) {
	return string(p), nil
}

func ParseServerProvider(s string) (ServerProvider, error) {
	provider := ServerProvider(s)
	if !provider.IsValid() {
		return ProviderCustom, fmt.Errorf("invalid server provider: %s", s)
	}

	return provider, nil
}

func AllServerProviders() []ServerProvider {
	return []ServerProvider{
		ProviderDigitalOcean, ProviderHetzner, ProviderLinode,
		ProviderVultr, ProviderAWS, ProviderCustom,
	}
}
