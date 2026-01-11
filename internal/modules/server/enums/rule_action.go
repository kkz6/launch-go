package enums

import (
	"database/sql/driver"
	"fmt"
)

// RuleAction represents firewall rule actions
type RuleAction string

const (
	RuleActionAllow  RuleAction = "allow"
	RuleActionDeny   RuleAction = "deny"
	RuleActionReject RuleAction = "reject"
)

func (r RuleAction) String() string {
	return string(r)
}

func (r RuleAction) Label() string {
	labels := map[RuleAction]string{
		RuleActionAllow:  "Allow",
		RuleActionDeny:   "Deny",
		RuleActionReject: "Reject",
	}
	if label, ok := labels[r]; ok {
		return label
	}

	return "Unknown"
}

func (r RuleAction) IsValid() bool {
	switch r {
	case RuleActionAllow, RuleActionDeny, RuleActionReject:
		return true
	}

	return false
}

func (r *RuleAction) Scan(value interface{}) error {
	if value == nil {
		*r = RuleActionAllow
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*r = RuleAction(v)
	case string:
		*r = RuleAction(v)
	default:
		return fmt.Errorf("cannot scan type %T into RuleAction", value)
	}

	return nil
}

func (r RuleAction) Value() (driver.Value, error) {
	return string(r), nil
}

func ParseRuleAction(s string) (RuleAction, error) {
	action := RuleAction(s)
	if !action.IsValid() {
		return RuleActionAllow, fmt.Errorf("invalid rule action: %s", s)
	}

	return action, nil
}

func AllRuleActions() []RuleAction {
	return []RuleAction{RuleActionAllow, RuleActionDeny, RuleActionReject}
}
