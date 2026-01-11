package enums

// PlanInterval represents the billing interval for a plan
type PlanInterval string

const (
	PlanIntervalMonthly PlanInterval = "monthly"
	PlanIntervalYearly  PlanInterval = "yearly"
)

// String returns the string representation of the interval
func (p PlanInterval) String() string {
	return string(p)
}

// IsValid checks if the interval is valid
func (p PlanInterval) IsValid() bool {
	return p == PlanIntervalMonthly || p == PlanIntervalYearly
}

// IsMonthly checks if the interval is monthly
func (p PlanInterval) IsMonthly() bool {
	return p == PlanIntervalMonthly
}

// IsYearly checks if the interval is yearly
func (p PlanInterval) IsYearly() bool {
	return p == PlanIntervalYearly
}

// AllPlanIntervals returns all valid plan intervals
func AllPlanIntervals() []PlanInterval {
	return []PlanInterval{
		PlanIntervalMonthly,
		PlanIntervalYearly,
	}
}
