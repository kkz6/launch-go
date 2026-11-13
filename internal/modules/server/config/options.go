package config

import (
	"slices"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
)

// PlanOption represents a plan option for cloud providers
type PlanOption struct {
	Value   string   `json:"value"`
	Title   string   `json:"title"`
	Regions []string `json:"regions,omitempty"`
}

// RegionOption represents a region option for cloud providers
type RegionOption struct {
	Value string `json:"value"`
	Title string `json:"title"`
}

// ProviderConfig represents the configuration for a cloud provider
type ProviderConfig struct {
	Plans   []PlanOption           `json:"plans"`
	Regions []RegionOption         `json:"regions"`
	Images  map[string]interface{} `json:"images,omitempty"`
}

// GetPhpVersions returns all available PHP versions as key-value pairs
func GetPhpVersions() map[string]string {
	versions := make(map[string]string)
	for _, php := range servertypes.AllPhpVersions() {
		versions[php.String()] = php.Label()
	}
	return versions
}

// GetDatabaseTypes returns all available database types as key-value pairs
func GetDatabaseTypes() map[string]string {
	result := make(map[string]string)
	for _, db := range servertypes.AllDatabaseTypes() {
		result[db.String()] = db.Label()
	}
	return result
}

// GetServerTypes returns all available server types as key-value pairs
func GetServerTypes() map[string]string {
	result := make(map[string]string)
	for _, st := range servertypes.AllServerTypes() {
		result[st.String()] = st.Label()
	}
	return result
}

// GetOperatingSystems returns all available operating systems as key-value pairs
func GetOperatingSystems() map[string]string {
	systems := make(map[string]string)
	for _, os := range servertypes.AllOperatingSystems() {
		systems[os.String()] = os.Label()
	}
	return systems
}

// GetProviderConfigs returns the configuration for all cloud providers
func GetProviderConfigs() map[string]ProviderConfig {
	return map[string]ProviderConfig{
		"digitalocean": getDigitalOceanConfig(),
		"hetzner":      getHetznerConfig(),
		"linode":       getLinodeConfig(),
		"vultr":        getVultrConfig(),
		"aws":          getAWSConfig(),
	}
}

func getDigitalOceanConfig() ProviderConfig {
	return ProviderConfig{
		Plans: []PlanOption{
			{Value: "s-1vcpu-1gb", Title: "1024MB RAM - CPU 1 core(s) - Disk: 25GB (s-1vcpu-1gb)"},
			{Value: "s-1vcpu-1gb-amd", Title: "1024MB RAM - CPU 1 core(s) - Disk: 25GB (s-1vcpu-1gb-amd)"},
			{Value: "s-1vcpu-1gb-intel", Title: "1024MB RAM - CPU 1 core(s) - Disk: 25GB (s-1vcpu-1gb-intel)"},
			{Value: "s-1vcpu-2gb", Title: "2048MB RAM - CPU 1 core(s) - Disk: 50GB (s-1vcpu-2gb)"},
			{Value: "s-1vcpu-2gb-amd", Title: "2048MB RAM - CPU 1 core(s) - Disk: 50GB (s-1vcpu-2gb-amd)"},
			{Value: "s-1vcpu-2gb-intel", Title: "2048MB RAM - CPU 1 core(s) - Disk: 50GB (s-1vcpu-2gb-intel)"},
			{Value: "s-2vcpu-2gb", Title: "2048MB RAM - CPU 2 core(s) - Disk: 60GB (s-2vcpu-2gb)"},
			{Value: "s-2vcpu-2gb-amd", Title: "2048MB RAM - CPU 2 core(s) - Disk: 60GB (s-2vcpu-2gb-amd)"},
			{Value: "s-2vcpu-2gb-intel", Title: "2048MB RAM - CPU 2 core(s) - Disk: 60GB (s-2vcpu-2gb-intel)"},
			{Value: "s-2vcpu-4gb", Title: "4096MB RAM - CPU 2 core(s) - Disk: 80GB (s-2vcpu-4gb)"},
			{Value: "s-2vcpu-4gb-amd", Title: "4096MB RAM - CPU 2 core(s) - Disk: 80GB (s-2vcpu-4gb-amd)"},
			{Value: "s-2vcpu-4gb-intel", Title: "4096MB RAM - CPU 2 core(s) - Disk: 80GB (s-2vcpu-4gb-intel)"},
			{Value: "s-4vcpu-8gb", Title: "8192MB RAM - CPU 4 core(s) - Disk: 160GB (s-4vcpu-8gb)"},
			{Value: "s-4vcpu-8gb-amd", Title: "8192MB RAM - CPU 4 core(s) - Disk: 160GB (s-4vcpu-8gb-amd)"},
			{Value: "s-4vcpu-8gb-intel", Title: "8192MB RAM - CPU 4 core(s) - Disk: 160GB (s-4vcpu-8gb-intel)"},
			{Value: "s-8vcpu-16gb", Title: "16384MB RAM - CPU 8 core(s) - Disk: 320GB (s-8vcpu-16gb)"},
			{Value: "c-2", Title: "4096MB RAM - CPU 2 core(s) - Disk: 25GB (c-2)"},
			{Value: "c2-2vcpu-4gb", Title: "4096MB RAM - CPU 2 core(s) - Disk: 50GB (c2-2vcpu-4gb)"},
			{Value: "g-2vcpu-8gb", Title: "8192MB RAM - CPU 2 core(s) - Disk: 25GB (g-2vcpu-8gb)"},
			{Value: "gd-2vcpu-8gb", Title: "8192MB RAM - CPU 2 core(s) - Disk: 50GB (gd-2vcpu-8gb)"},
		},
		Regions: []RegionOption{
			{Value: "nyc1", Title: "New York 1"},
			{Value: "sgp1", Title: "Singapore 1"},
			{Value: "lon1", Title: "London 1"},
			{Value: "nyc3", Title: "New York 3"},
			{Value: "ams3", Title: "Amsterdam 3"},
			{Value: "fra1", Title: "Frankfurt 1"},
			{Value: "tor1", Title: "Toronto 1"},
			{Value: "blr1", Title: "Bangalore 1"},
			{Value: "sfo3", Title: "San Francisco 3"},
		},
		// DigitalOcean retires the numeric IDs of base images periodically —
		// the worker hit a 422 "image no longer available" when 168977420 was
		// pulled. Slugs like `ubuntu-24-04-x64` are the stable reference; DO
		// repoints the slug to whatever the current published snapshot is.
		Images: map[string]interface{}{
			"ubuntu_22": "ubuntu-22-04-x64",
			"ubuntu_24": "ubuntu-24-04-x64",
		},
	}
}

func getHetznerConfig() ProviderConfig {
	europe := []string{"fsn1", "nbg1", "hel1"}
	us := []string{"ash", "hil"}
	europeAndSingapore := []string{"fsn1", "nbg1", "hel1", "sin"}
	return ProviderConfig{
		Plans: []PlanOption{
			{Value: "cx23", Title: "CX23 - 2 vCPU - 4 GB RAM - 40 GB disk", Regions: europe},
			{Value: "cx33", Title: "CX33 - 4 vCPU - 8 GB RAM - 80 GB disk", Regions: europe},
			{Value: "cx43", Title: "CX43 - 8 vCPU - 16 GB RAM - 160 GB disk", Regions: europe},
			{Value: "cx53", Title: "CX53 - 16 vCPU - 32 GB RAM - 320 GB disk", Regions: europe},
			{Value: "ccx13", Title: "CCX13 - 2 vCPU - 8 GB RAM - 80 GB disk"},
			{Value: "ccx23", Title: "CCX23 - 4 vCPU - 16 GB RAM - 160 GB disk"},
			{Value: "ccx33", Title: "CCX33 - 8 vCPU - 32 GB RAM - 240 GB disk"},
			{Value: "ccx43", Title: "CCX43 - 16 vCPU - 64 GB RAM - 360 GB disk"},
			{Value: "ccx53", Title: "CCX53 - 32 vCPU - 128 GB RAM - 600 GB disk"},
			{Value: "ccx63", Title: "CCX63 - 48 vCPU - 192 GB RAM - 960 GB disk"},
			{Value: "cpx11", Title: "CPX11 - 2 vCPU - 2 GB RAM - 40 GB disk", Regions: us},
			{Value: "cpx21", Title: "CPX21 - 3 vCPU - 4 GB RAM - 80 GB disk", Regions: us},
			{Value: "cpx31", Title: "CPX31 - 4 vCPU - 8 GB RAM - 160 GB disk", Regions: us},
			{Value: "cpx41", Title: "CPX41 - 8 vCPU - 16 GB RAM - 240 GB disk", Regions: us},
			{Value: "cpx51", Title: "CPX51 - 16 vCPU - 32 GB RAM - 360 GB disk", Regions: us},
			{Value: "cpx12", Title: "CPX12 - 1 vCPU - 2 GB RAM - 40 GB disk", Regions: europeAndSingapore},
			{Value: "cpx22", Title: "CPX22 - 2 vCPU - 4 GB RAM - 80 GB disk", Regions: europeAndSingapore},
			{Value: "cpx32", Title: "CPX32 - 4 vCPU - 8 GB RAM - 160 GB disk", Regions: europeAndSingapore},
			{Value: "cpx42", Title: "CPX42 - 8 vCPU - 16 GB RAM - 320 GB disk", Regions: europeAndSingapore},
			{Value: "cpx52", Title: "CPX52 - 12 vCPU - 24 GB RAM - 480 GB disk", Regions: europeAndSingapore},
			{Value: "cpx62", Title: "CPX62 - 16 vCPU - 32 GB RAM - 640 GB disk", Regions: europeAndSingapore},
			{Value: "cax11", Title: "CAX11 - 2 vCPU - 4 GB RAM - 40 GB disk", Regions: europe},
			{Value: "cax21", Title: "CAX21 - 4 vCPU - 8 GB RAM - 80 GB disk", Regions: europe},
			{Value: "cax31", Title: "CAX31 - 8 vCPU - 16 GB RAM - 160 GB disk", Regions: europe},
			{Value: "cax41", Title: "CAX41 - 16 vCPU - 32 GB RAM - 320 GB disk", Regions: europe},
		},
		Regions: []RegionOption{
			{Value: "fsn1", Title: "DE - Falkenstein"},
			{Value: "nbg1", Title: "DE - Nuremberg"},
			{Value: "hel1", Title: "FI - Helsinki"},
			{Value: "ash", Title: "US - Ashburn, VA"},
			{Value: "hil", Title: "US - Hillsboro, OR"},
			{Value: "sin", Title: "SG - Singapore"},
		},
		Images: map[string]interface{}{
			"ubuntu_22": "ubuntu-22.04",
			"ubuntu_24": "ubuntu-24.04",
		},
	}
}

// IsPlanAvailableInRegion rejects known unsupported plan/location combinations.
func IsPlanAvailableInRegion(provider servertypes.ServerProvider, plan, region string) bool {
	if provider == servertypes.ProviderHetzner {
		for _, option := range getHetznerConfig().Plans {
			if option.Value == plan && len(option.Regions) > 0 {
				return slices.Contains(option.Regions, region)
			}
		}
	}
	return true
}

func getLinodeConfig() ProviderConfig {
	return ProviderConfig{
		Plans: []PlanOption{
			{Value: "g6-nanode-1", Title: "Nanode 1GB"},
			{Value: "g6-standard-1", Title: "Linode 2GB"},
			{Value: "g6-standard-2", Title: "Linode 4GB"},
			{Value: "g6-standard-4", Title: "Linode 8GB"},
			{Value: "g6-standard-6", Title: "Linode 16GB"},
			{Value: "g6-standard-8", Title: "Linode 32GB"},
			{Value: "g6-standard-16", Title: "Linode 64GB"},
			{Value: "g6-standard-20", Title: "Linode 96GB"},
			{Value: "g6-standard-24", Title: "Linode 128GB"},
			{Value: "g6-standard-32", Title: "Linode 192GB"},
			{Value: "g7-highmem-1", Title: "Linode 24GB"},
			{Value: "g7-highmem-2", Title: "Linode 48GB"},
			{Value: "g7-highmem-4", Title: "Linode 90GB"},
			{Value: "g7-highmem-8", Title: "Linode 150GB"},
			{Value: "g7-highmem-16", Title: "Linode 300GB"},
			{Value: "g6-dedicated-2", Title: "Dedicated 4GB"},
			{Value: "g6-dedicated-4", Title: "Dedicated 8GB"},
			{Value: "g6-dedicated-8", Title: "Dedicated 16GB"},
			{Value: "g6-dedicated-16", Title: "Dedicated 32GB"},
			{Value: "g6-dedicated-32", Title: "Dedicated 64GB"},
			{Value: "g6-dedicated-48", Title: "Dedicated 96GB"},
			{Value: "g6-dedicated-50", Title: "Dedicated 128GB"},
			{Value: "g6-dedicated-56", Title: "Dedicated 256GB"},
			{Value: "g1-gpu-rtx6000-1", Title: "Dedicated 32GB + RTX6000 GPU x1"},
			{Value: "g1-gpu-rtx6000-2", Title: "Dedicated 64GB + RTX6000 GPU x2"},
			{Value: "g1-gpu-rtx6000-3", Title: "Dedicated 96GB + RTX6000 GPU x3"},
			{Value: "g1-gpu-rtx6000-4", Title: "Dedicated 128GB + RTX6000 GPU x4"},
		},
		Regions: []RegionOption{
			{Value: "ap-west", Title: "ap-west - India"},
			{Value: "ca-central", Title: "ca-central - Canada"},
			{Value: "ap-southeast", Title: "ap-southeast - Australia"},
			{Value: "us-central", Title: "us-central - United States"},
			{Value: "us-west", Title: "us-west - United States"},
			{Value: "us-southeast", Title: "us-southeast - United States"},
			{Value: "us-east", Title: "us-east - United States"},
			{Value: "eu-west", Title: "eu-west - United Kingdom"},
			{Value: "ap-south", Title: "ap-south - Singapore"},
			{Value: "eu-central", Title: "eu-central - Germany"},
			{Value: "ap-northeast", Title: "ap-northeast - Japan"},
		},
		Images: map[string]interface{}{
			"ubuntu_22": "linode/ubuntu22.04",
			"ubuntu_24": "linode/ubuntu24.04",
		},
	}
}

func getVultrConfig() ProviderConfig {
	return ProviderConfig{
		Plans: []PlanOption{
			{Value: "vc2-1c-1gb", Title: "1 CPU - 1024MB Ram - 25GB Disk"},
			{Value: "vc2-1c-2gb", Title: "1 CPU - 2048MB Ram - 55GB Disk"},
			{Value: "vc2-2c-4gb", Title: "2 CPU - 4096MB Ram - 80GB Disk"},
			{Value: "vc2-4c-8gb", Title: "4 CPU - 8192MB Ram - 160GB Disk"},
			{Value: "vc2-6c-16gb", Title: "6 CPU - 16384MB Ram - 320GB Disk"},
			{Value: "vc2-8c-32gb", Title: "8 CPU - 32768MB Ram - 640GB Disk"},
			{Value: "vc2-16c-64gb", Title: "16 CPU - 65536MB Ram - 1280GB Disk"},
			{Value: "vc2-24c-96gb", Title: "24 CPU - 98304MB Ram - 1600GB Disk"},
			{Value: "vhf-1c-1gb", Title: "1 CPU - 1024MB Ram - 32GB Disk"},
			{Value: "vhf-1c-2gb", Title: "1 CPU - 2048MB Ram - 64GB Disk"},
			{Value: "vhf-2c-2gb", Title: "2 CPU - 2048MB Ram - 80GB Disk"},
			{Value: "vhf-2c-4gb", Title: "2 CPU - 4096MB Ram - 128GB Disk"},
			{Value: "vhf-3c-8gb", Title: "3 CPU - 8192MB Ram - 256GB Disk"},
			{Value: "vhf-4c-16gb", Title: "4 CPU - 16384MB Ram - 384GB Disk"},
			{Value: "vhf-6c-24gb", Title: "6 CPU - 24576MB Ram - 448GB Disk"},
			{Value: "vhf-8c-32gb", Title: "8 CPU - 32768MB Ram - 512GB Disk"},
			{Value: "vhf-12c-48gb", Title: "12 CPU - 49152MB Ram - 768GB Disk"},
		},
		Regions: []RegionOption{
			{Value: "ams", Title: "Europe - Amsterdam"},
			{Value: "atl", Title: "North America - Atlanta"},
			{Value: "cdg", Title: "Europe - Paris"},
			{Value: "dfw", Title: "North America - Dallas"},
			{Value: "ewr", Title: "North America - New Jersey"},
			{Value: "fra", Title: "Europe - Frankfurt"},
			{Value: "icn", Title: "Asia - Seoul"},
			{Value: "lax", Title: "North America - Los Angeles"},
			{Value: "lhr", Title: "Europe - London"},
			{Value: "mex", Title: "North America - Mexico City"},
			{Value: "mia", Title: "North America - Miami"},
			{Value: "nrt", Title: "Asia - Tokyo"},
			{Value: "ord", Title: "North America - Chicago"},
			{Value: "sea", Title: "North America - Seattle"},
			{Value: "sgp", Title: "Asia - Singapore"},
			{Value: "sjc", Title: "North America - Silicon Valley"},
			{Value: "sto", Title: "Europe - Stockholm"},
			{Value: "syd", Title: "Australia - Sydney"},
			{Value: "yto", Title: "North America - Toronto"},
		},
		// Vultr uses numeric image IDs which can be retired in the future
		// (the same class of bug that hit DO with snapshot 168977420). When
		// a 422 fires from Vultr, re-check these against `GET /v2/os` and
		// the cmd/validate-images helper.
		Images: map[string]interface{}{
			"ubuntu_22": "1743",
			"ubuntu_24": "2284",
		},
	}
}

func getAWSConfig() ProviderConfig {
	return ProviderConfig{
		Plans: []PlanOption{
			{Value: "t2.nano", Title: "[t2 nano] 512MB RAM - CPU 1 core(s)"},
			{Value: "t2.micro", Title: "[t2 micro] 1024MB RAM - CPU 1 core(s)"},
			{Value: "t2.small", Title: "[t2 small] 2048MB RAM - CPU 1 core(s)"},
			{Value: "t2.medium", Title: "[t2 medium] 4096MB RAM - CPU 2 core(s)"},
			{Value: "t2.large", Title: "[t2 large] 8192MB RAM - CPU 2 core(s)"},
			{Value: "t2.xlarge", Title: "[t2 xlarge] 16384MB RAM - CPU 4 core(s)"},
			{Value: "t2.2xlarge", Title: "[t2 2xlarge] 32768MB RAM - CPU 8 core(s)"},
			{Value: "t3a.nano", Title: "[t3a nano] 512MB RAM - CPU 2 core(s)"},
			{Value: "t3a.micro", Title: "[t3a micro] 1024MB RAM - CPU 2 core(s)"},
			{Value: "t3a.small", Title: "[t3a small] 2048MB RAM - CPU 2 core(s)"},
			{Value: "t3a.medium", Title: "[t3a medium] 4096MB RAM - CPU 2 core(s)"},
			{Value: "t3a.large", Title: "[t3a large] 8192MB RAM - CPU 2 core(s)"},
			{Value: "t3a.xlarge", Title: "[t3a xlarge] 16384MB RAM - CPU 4 core(s)"},
		},
		Regions: []RegionOption{
			{Value: "us-east-2", Title: "US East (Ohio)"},
			{Value: "us-east-1", Title: "US East (Virginia)"},
			{Value: "us-west-1", Title: "US West (N. California)"},
			{Value: "us-west-2", Title: "US West (Oregon)"},
			{Value: "af-south-1", Title: "Africa (Cape Town)"},
			{Value: "ap-east-1", Title: "Asia Pacific (Hong Kong)"},
			{Value: "ap-southeast-3", Title: "Asia Pacific (Jakarta)"},
			{Value: "ap-southeast-4", Title: "Asia Pacific (Melbourne)"},
			{Value: "ap-south-1", Title: "Asia Pacific (Mumbai)"},
			{Value: "ap-northeast-3", Title: "Asia Pacific (Osaka)"},
			{Value: "ap-northeast-2", Title: "Asia Pacific (Seoul)"},
			{Value: "ap-southeast-1", Title: "Asia Pacific (Singapore)"},
			{Value: "ap-southeast-2", Title: "Asia Pacific (Sydney)"},
			{Value: "ap-northeast-1", Title: "Asia Pacific (Tokyo)"},
			{Value: "ca-central-1", Title: "Canada (Central)"},
			{Value: "ca-west-1", Title: "Canada West (Calgary)"},
			{Value: "eu-central-1", Title: "Europe (Frankfurt)"},
			{Value: "eu-west-1", Title: "Europe (Ireland)"},
			{Value: "eu-west-2", Title: "Europe (London)"},
			{Value: "eu-south-1", Title: "Europe (Milan)"},
			{Value: "eu-west-3", Title: "Europe (Paris)"},
			{Value: "eu-south-2", Title: "Europe (Spain)"},
			{Value: "eu-north-1", Title: "Europe (Stockholm)"},
			{Value: "eu-central-2", Title: "Europe (Zurich)"},
			{Value: "il-central-1", Title: "Israel (Tel Aviv)"},
			{Value: "me-south-1", Title: "Middle East (Bahrain)"},
			{Value: "me-central-1", Title: "Middle East (UAE)"},
			{Value: "sa-east-1", Title: "South America (São Paulo)"},
		},
	}
}

// GetSupportedOSesByProvider reports the OS images for providers that can provision servers.
func GetSupportedOSesByProvider() map[string][]string {
	out := make(map[string][]string)
	for provider, cfg := range GetProviderConfigs() {
		if len(cfg.Images) == 0 {
			continue
		}
		oses := make([]string, 0, len(cfg.Images))
		for os := range cfg.Images {
			if s, ok := cfg.Images[os].(string); ok && s != "" {
				oses = append(oses, os)
			}
		}
		out[provider] = sortedStrings(oses)
	}
	return out
}

func sortedStrings(in []string) []string {
	// Tiny, stable sort — we expect ~3 entries, so cost is irrelevant.
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j-1] > in[j]; j-- {
			in[j-1], in[j] = in[j], in[j-1]
		}
	}
	return in
}

// GetProviderImage returns the image ID for a provider and OS
func GetProviderImage(provider, os string) string {
	configs := GetProviderConfigs()
	if config, ok := configs[provider]; ok {
		if images := config.Images; images != nil {
			if image, ok := images[os]; ok {
				if str, ok := image.(string); ok {
					return str
				}
			}
		}
	}
	return ""
}

// PhpExtension represents a PHP extension that can be installed
type PhpExtension struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// GetAvailablePhpExtensions returns PHP extensions available via Ondrej PPA (apt-get install php{version}-{ext})
func GetAvailablePhpExtensions() []PhpExtension {
	return []PhpExtension{
		{Value: "bcmath", Label: "BCMath", Description: "Arbitrary precision mathematics"},
		{Value: "bz2", Label: "Bzip2", Description: "Bzip2 compression"},
		{Value: "curl", Label: "cURL", Description: "URL transfer library"},
		{Value: "dba", Label: "DBA", Description: "Database abstraction layer"},
		{Value: "enchant", Label: "Enchant", Description: "Spell checking library"},
		{Value: "gd", Label: "GD", Description: "Image processing"},
		{Value: "gmp", Label: "GMP", Description: "GNU Multiple Precision"},
		{Value: "igbinary", Label: "Igbinary", Description: "Binary serialization"},
		{Value: "imagick", Label: "ImageMagick", Description: "Image manipulation"},
		{Value: "imap", Label: "IMAP", Description: "Email protocol support"},
		{Value: "intl", Label: "Intl", Description: "Internationalization"},
		{Value: "ldap", Label: "LDAP", Description: "Directory access protocol"},
		{Value: "mbstring", Label: "Mbstring", Description: "Multibyte string handling"},
		{Value: "memcached", Label: "Memcached", Description: "Memcached caching"},
		{Value: "mongodb", Label: "MongoDB", Description: "MongoDB driver"},
		{Value: "msgpack", Label: "MessagePack", Description: "MessagePack serialization"},
		{Value: "mysql", Label: "MySQL", Description: "MySQL database support"},
		{Value: "odbc", Label: "ODBC", Description: "ODBC database access"},
		{Value: "opcache", Label: "OPcache", Description: "Opcode caching"},
		{Value: "pgsql", Label: "PostgreSQL", Description: "PostgreSQL database support"},
		{Value: "pspell", Label: "Pspell", Description: "Spell checking"},
		{Value: "readline", Label: "Readline", Description: "CLI line editing"},
		{Value: "redis", Label: "Redis", Description: "Redis caching"},
		{Value: "snmp", Label: "SNMP", Description: "Network management protocol"},
		{Value: "soap", Label: "SOAP", Description: "SOAP web services"},
		{Value: "sqlite3", Label: "SQLite3", Description: "SQLite database support"},
		{Value: "tidy", Label: "Tidy", Description: "HTML/XML cleanup"},
		{Value: "xdebug", Label: "Xdebug", Description: "Debugging and profiling"},
		{Value: "xml", Label: "XML", Description: "XML parsing"},
		{Value: "xsl", Label: "XSL", Description: "XSL transformations"},
		{Value: "zip", Label: "Zip", Description: "ZIP archive handling"},
	}
}

// GetPhpExtensionLabel returns the label for a PHP extension
func GetPhpExtensionLabel(value string) string {
	for _, ext := range GetAvailablePhpExtensions() {
		if ext.Value == value {
			return ext.Label
		}
	}
	// If not found in our list, capitalize the first letter
	if len(value) > 0 {
		return string(value[0]-32) + value[1:]
	}
	return value
}
