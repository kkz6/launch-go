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

// DefaultPlans returns default plan configurations
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:             "1",
			Name:           "Starter",
			Description:    "Perfect for small projects",
			MonthlyID:      "starter_monthly",
			YearlyID:       "starter_yearly",
			MonthlyPricing: 900,
			YearlyPricing:  9000,
			Features: []string{
				"Up to 3 servers",
				"Up to 5 sites per server",
				"5 deployments retained",
				"2 team members",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            3,
				MaxSitesPerServer:     5,
				MaxDeploymentsPerSite: 5,
				MaxTeamMembers:        2,
				HasBackups:            false,
				HasMonitoring:         false,
			},
		},
		{
			ID:             "2",
			Name:           "Pro",
			Description:    "For growing teams",
			MonthlyID:      "pro_monthly",
			YearlyID:       "pro_yearly",
			MonthlyPricing: 2900,
			YearlyPricing:  29000,
			Features: []string{
				"Up to 10 servers",
				"Up to 20 sites per server",
				"10 deployments retained",
				"5 team members",
				"Server monitoring",
			},
			Recommended: true,
			Options: PlanOptions{
				MaxServers:            10,
				MaxSitesPerServer:     20,
				MaxDeploymentsPerSite: 10,
				MaxTeamMembers:        5,
				HasBackups:            false,
				HasMonitoring:         true,
			},
		},
		{
			ID:             "3",
			Name:           "Enterprise",
			Description:    "For large organizations",
			MonthlyID:      "enterprise_monthly",
			YearlyID:       "enterprise_yearly",
			MonthlyPricing: 9900,
			YearlyPricing:  99000,
			Features: []string{
				"Unlimited servers",
				"Unlimited sites per server",
				"Unlimited deployments",
				"Unlimited team members",
				"Server monitoring",
				"Automated backups",
			},
			Recommended: false,
			Options: PlanOptions{
				MaxServers:            999999,
				MaxSitesPerServer:     999999,
				MaxDeploymentsPerSite: 999999,
				MaxTeamMembers:        999999,
				HasBackups:            true,
				HasMonitoring:         true,
			},
		},
	}
}
