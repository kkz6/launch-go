package models

// Plan represents a billing plan configuration (stored in config, not database)
type Plan struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	MonthlyID      string      `json:"monthly_id"`
	YearlyID       string      `json:"yearly_id"`
	MonthlyPricing int64       `json:"monthly_pricing"`
	YearlyPricing  int64       `json:"yearly_pricing"`
	Features       []string    `json:"features,omitempty"`
	Recommended    bool        `json:"recommended"`
	Options        PlanOptions `json:"options"`
}

// PlanOptions represents the limits and features for a plan
type PlanOptions struct {
	MaxServers            int  `json:"max_servers"`
	MaxSitesPerServer     int  `json:"max_sites_per_server"`
	MaxDeploymentsPerSite int  `json:"max_deployments_per_site"`
	MaxTeamMembers        int  `json:"max_team_members"`
	HasBackups            bool `json:"has_backups"`
	HasMonitoring         bool `json:"has_monitoring"`
}

// DefaultPlans returns default plan configurations with DodoPayments product IDs
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:             "396982",
			Name:           "Hobby Plan",
			MonthlyID:      "616451",
			YearlyID:       "616450",
			MonthlyPricing: 199,
			YearlyPricing:  2380,
			Features: []string{
				"1 server",
				"1 site per server",
				"5 deployments retained",
				"1 team member",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            1,
				MaxSitesPerServer:     1,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        1,
				HasBackups:            false,
				HasMonitoring:         false,
			},
		},
		{
			ID:             "409766",
			Name:           "Compact Plan",
			MonthlyID:      "625979",
			YearlyID:       "625980",
			MonthlyPricing: 699,
			YearlyPricing:  8388,
			Features: []string{
				"3 servers",
				"10 sites per server",
				"5 deployments retained",
				"1 team member",
				"Backups",
				"Monitoring",
			},
			Recommended: true,
			Options: PlanOptions{
				MaxServers:            3,
				MaxSitesPerServer:     10,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        1,
				HasBackups:            true,
				HasMonitoring:         true,
			},
		},
		{
			ID:             "433445",
			Name:           "Turbo Plan",
			MonthlyID:      "666642",
			YearlyID:       "666643",
			MonthlyPricing: 2000,
			YearlyPricing:  24000,
			Features: []string{
				"10 servers",
				"20 sites per server",
				"5 deployments retained",
				"1 team member",
				"Backups",
				"Monitoring",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            10,
				MaxSitesPerServer:     20,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        1,
				HasBackups:            true,
				HasMonitoring:         true,
			},
		},
	}
}
