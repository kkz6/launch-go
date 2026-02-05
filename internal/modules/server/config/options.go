package config

import servertypes "github.com/kkz6/launch-go/internal/modules/server/types"

// PlanOption represents a plan option for cloud providers
type PlanOption struct {
	Value string `json:"value"`
	Title string `json:"title"`
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
			{Value: "ams2", Title: "Amsterdam 2"},
			{Value: "sgp1", Title: "Singapore 1"},
			{Value: "lon1", Title: "London 1"},
			{Value: "nyc3", Title: "New York 3"},
			{Value: "ams3", Title: "Amsterdam 3"},
			{Value: "fra1", Title: "Frankfurt 1"},
			{Value: "tor1", Title: "Toronto 1"},
			{Value: "blr1", Title: "Bangalore 1"},
			{Value: "sfo3", Title: "San Francisco 3"},
		},
		Images: map[string]interface{}{
			"ubuntu_20": "112929454",
			"ubuntu_22": "159651797",
			"ubuntu_24": "168977420",
		},
	}
}

func getHetznerConfig() ProviderConfig {
	return ProviderConfig{
		Plans: []PlanOption{
			{Value: "cx11", Title: "CX11 - 1 Cores - 2 Memory - 20 Disk"},
			{Value: "cx21", Title: "CX21 - 2 Cores - 4 Memory - 40 Disk"},
			{Value: "cx31", Title: "CX31 - 2 Cores - 8 Memory - 80 Disk"},
			{Value: "cx41", Title: "CX41 - 4 Cores - 16 Memory - 160 Disk"},
			{Value: "cx51", Title: "CX51 - 8 Cores - 32 Memory - 240 Disk"},
			{Value: "ccx11", Title: "CCX11 Dedicated CPU - 2 Cores - 8 Memory - 80 Disk"},
			{Value: "ccx21", Title: "CCX21 Dedicated CPU - 4 Cores - 16 Memory - 160 Disk"},
			{Value: "ccx31", Title: "CCX31 Dedicated CPU - 8 Cores - 32 Memory - 240 Disk"},
			{Value: "ccx41", Title: "CCX41 Dedicated CPU - 16 Cores - 64 Memory - 360 Disk"},
			{Value: "ccx51", Title: "CCX51 Dedicated CPU - 32 Cores - 128 Memory - 600 Disk"},
			{Value: "cpx11", Title: "CPX 11 - 2 Cores - 2 Memory - 40 Disk"},
			{Value: "cpx21", Title: "CPX 21 - 3 Cores - 4 Memory - 80 Disk"},
			{Value: "cpx31", Title: "CPX 31 - 4 Cores - 8 Memory - 160 Disk"},
			{Value: "cpx41", Title: "CPX 41 - 8 Cores - 16 Memory - 240 Disk"},
			{Value: "cpx51", Title: "CPX 51 - 16 Cores - 32 Memory - 360 Disk"},
			{Value: "ccx12", Title: "CCX12 Dedicated CPU - 2 Cores - 8 Memory - 80 Disk"},
			{Value: "ccx22", Title: "CCX22 Dedicated CPU - 4 Cores - 16 Memory - 160 Disk"},
			{Value: "ccx32", Title: "CCX32 Dedicated CPU - 8 Cores - 32 Memory - 240 Disk"},
			{Value: "ccx42", Title: "CCX42 Dedicated CPU - 16 Cores - 64 Memory - 360 Disk"},
			{Value: "ccx52", Title: "CCX52 Dedicated CPU - 32 Cores - 128 Memory - 600 Disk"},
			{Value: "ccx62", Title: "CCX62 Dedicated CPU - 48 Cores - 192 Memory - 960 Disk"},
			{Value: "cax11", Title: "CAX11 - 2 Cores - 4 Memory - 40 Disk"},
			{Value: "cax21", Title: "CAX21 - 4 Cores - 8 Memory - 80 Disk"},
			{Value: "cax31", Title: "CAX31 - 8 Cores - 16 Memory - 160 Disk"},
			{Value: "cax41", Title: "CAX41 - 16 Cores - 32 Memory - 320 Disk"},
		},
		Regions: []RegionOption{
			{Value: "fsn1", Title: "DE - Falkenstein"},
			{Value: "nbg1", Title: "DE - Nuremberg"},
			{Value: "hel1", Title: "FI - Helsinki"},
			{Value: "ash", Title: "US - Ashburn, VA"},
			{Value: "hil", Title: "US - Hillsboro, OR"},
		},
		Images: map[string]interface{}{
			"ubuntu_18": "ubuntu-18.04",
			"ubuntu_20": "ubuntu-20.04",
			"ubuntu_22": "ubuntu-22.04",
			"ubuntu_24": "ubuntu-24.04",
		},
	}
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
			{Value: "g6-dedicated-64", Title: "Dedicated 512GB"},
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
			"ubuntu_18": "linode/ubuntu18.04",
			"ubuntu_20": "linode/ubuntu20.04",
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
			{Value: "vdc-2vcpu-8gb", Title: "2 CPU - 8192MB Ram - 110GB Disk"},
			{Value: "vdc-4vcpu-16gb", Title: "4 CPU - 16384MB Ram - 110GB Disk"},
			{Value: "vdc-6vcpu-24gb", Title: "6 CPU - 24576MB Ram - 110GB Disk"},
			{Value: "vdc-8vcpu-32gb", Title: "8 CPU - 32768MB Ram - 110GB Disk"},
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
		Images: map[string]interface{}{
			"ubuntu_18": "270",
			"ubuntu_20": "387",
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
		Images: map[string]interface{}{
			"eu-south-2":     map[string]string{"ubuntu_24": "ami-0a50f993202fe4f22", "ubuntu_22": "ami-043e9941c6aec0f52", "ubuntu_20": "ami-086f353893612e446"},
			"eu-west-1":      map[string]string{"ubuntu_24": "ami-0776c814353b4814d", "ubuntu_22": "ami-0d0fa503c811361ab", "ubuntu_20": "ami-0008aa5cb0cde3400"},
			"af-south-1":     map[string]string{"ubuntu_24": "ami-0bfda59e8f84ff5ed", "ubuntu_22": "ami-0d06a4031539a9be6", "ubuntu_20": "ami-0ea465fccfaf199ce"},
			"eu-west-2":      map[string]string{"ubuntu_24": "ami-053a617c6207ecc7b", "ubuntu_22": "ami-0eb5c35d7b89f3488", "ubuntu_20": "ami-0608dbf22649c0159"},
			"eu-south-1":     map[string]string{"ubuntu_24": "ami-0355c99d0faba8847", "ubuntu_22": "ami-0bdcf995dcfebf29c", "ubuntu_20": "ami-034ea9dc86027e603"},
			"ap-south-1":     map[string]string{"ubuntu_24": "ami-0f58b397bc5c1f2e8", "ubuntu_22": "ami-0f16c6c3de733b474", "ubuntu_20": "ami-02f829375c976f810"},
			"il-central-1":   map[string]string{"ubuntu_24": "ami-04a4b28d712600827", "ubuntu_22": "ami-09cd8eea397932e88", "ubuntu_20": "ami-03988803bd4e18212"},
			"eu-north-1":     map[string]string{"ubuntu_24": "ami-0705384c0b33c194c", "ubuntu_22": "ami-0fff1012fc5cb9f25", "ubuntu_20": "ami-07cca21629288f454"},
			"me-central-1":   map[string]string{"ubuntu_24": "ami-048798fd481c4c791", "ubuntu_22": "ami-042fcc4c33a3b6429", "ubuntu_20": "ami-0f98fff9d77968c80"},
			"ca-central-1":   map[string]string{"ubuntu_24": "ami-0c4596ce1e7ae3e68", "ubuntu_22": "ami-04fea581fe25e2675", "ubuntu_20": "ami-05690acfbddfbeaf6"},
			"eu-west-3":      map[string]string{"ubuntu_24": "ami-00ac45f3035ff009e", "ubuntu_22": "ami-0b020d95f579c8f43", "ubuntu_20": "ami-0130b7d3ec1d07e4f"},
			"ap-south-2":     map[string]string{"ubuntu_24": "ami-008616ec4a2c6975e", "ubuntu_22": "ami-088e75eecea53e53e", "ubuntu_20": "ami-0688d182e7c22ec3f"},
			"ca-west-1":      map[string]string{"ubuntu_24": "ami-07022089d2e36ace0", "ubuntu_22": "ami-02e22cefcad05a835", "ubuntu_20": "ami-03890126b7675fac8"},
			"eu-central-1":   map[string]string{"ubuntu_24": "ami-01e444924a2233b07", "ubuntu_22": "ami-01a93368cab494eb5", "ubuntu_20": "ami-07fd6b7604806e876"},
			"me-south-1":     map[string]string{"ubuntu_24": "ami-087f3ec3fdda67295", "ubuntu_22": "ami-03ae386fab11fa0a1", "ubuntu_20": "ami-0f65a186b3552f348"},
			"ap-northeast-1": map[string]string{"ubuntu_24": "ami-01bef798938b7644d", "ubuntu_22": "ami-08e32db9e33e28876", "ubuntu_20": "ami-0ed286a950292f370"},
			"ap-southeast-1": map[string]string{"ubuntu_24": "ami-003c463c8207b4dfa", "ubuntu_22": "ami-084cab24460184bd3", "ubuntu_20": "ami-081ee02c4cdf3917c"},
			"us-west-1":      map[string]string{"ubuntu_24": "ami-08012c0a9ee8e21c4", "ubuntu_22": "ami-023f8bebe991375fd", "ubuntu_20": "ami-0344f34a6875de16a"},
			"ap-southeast-3": map[string]string{"ubuntu_24": "ami-00c31062c5966e820", "ubuntu_22": "ami-0fd547652d1673e30", "ubuntu_20": "ami-0699dddffd3542faf"},
			"ap-northeast-2": map[string]string{"ubuntu_24": "ami-0e6f2b2fa0ca704d0", "ubuntu_22": "ami-0720c7fcba4b88b36", "ubuntu_20": "ami-03ec7d02334d21d49"},
			"ap-southeast-2": map[string]string{"ubuntu_24": "ami-080660c9757080771", "ubuntu_22": "ami-0d9d3b991cfa8ac6e", "ubuntu_20": "ami-06c7a70c38594fef6"},
			"us-east-1":      map[string]string{"ubuntu_24": "ami-04b70fa74e45c3917", "ubuntu_22": "ami-0cfa2ad4242c3168d", "ubuntu_20": "ami-0e3a6d8ff4c8fe246"},
			"us-west-2":      map[string]string{"ubuntu_24": "ami-0cf2b4e024cdb6960", "ubuntu_22": "ami-09c3a3c2cf6003f6c", "ubuntu_20": "ami-091c4300a778841cc"},
			"ap-east-1":      map[string]string{"ubuntu_24": "ami-026789b06a607b9a5", "ubuntu_22": "ami-0361acb22fef7522b", "ubuntu_20": "ami-0c0665dcea29a292d"},
			"eu-central-2":   map[string]string{"ubuntu_24": "ami-053ea2f9d1d6ac54c", "ubuntu_22": "ami-09407f9985de426af", "ubuntu_20": "ami-00c9866441e3616dd"},
			"us-east-2":      map[string]string{"ubuntu_24": "ami-09040d770ffe2224f", "ubuntu_22": "ami-0b986fc833876b42e", "ubuntu_20": "ami-010e55fe08af05fa7"},
			"ap-northeast-3": map[string]string{"ubuntu_24": "ami-0b9bc7dcdbcff394e", "ubuntu_22": "ami-063600dcf13c07ebc", "ubuntu_20": "ami-0b7108d627f57c7c8"},
			"sa-east-1":      map[string]string{"ubuntu_24": "ami-04716897be83e3f04", "ubuntu_22": "ami-0e6dfcf4e0e4dfc52", "ubuntu_20": "ami-050e1159c5a10dd81"},
			"ap-southeast-4": map[string]string{"ubuntu_24": "ami-0396cf525fd0aa5c1", "ubuntu_22": "ami-097638dc9b6250206", "ubuntu_20": "ami-02d0fccf5cdcdd8c5"},
		},
	}
}

// GetAWSImageForRegion returns the AMI ID for a specific region and OS
func GetAWSImageForRegion(region, os string) string {
	config := getAWSConfig()
	if regionImages, ok := config.Images[region]; ok {
		if osImages, ok := regionImages.(map[string]string); ok {
			if ami, ok := osImages[os]; ok {
				return ami
			}
		}
	}
	return ""
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
