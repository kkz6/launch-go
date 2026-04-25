package enumtypes

import "testing"

type fruit string

const (
	fruitApple  fruit = "apple"
	fruitBanana fruit = "banana"
)

var allFruits = []fruit{fruitApple, fruitBanana}

type priority int

const (
	priorityLow  priority = 1
	priorityHigh priority = 2
)

func TestIsValid_StringEnum(t *testing.T) {
	tests := []struct {
		name string
		val  fruit
		want bool
	}{
		{"apple is valid", fruitApple, true},
		{"banana is valid", fruitBanana, true},
		{"empty string is invalid", "", false},
		{"unknown value is invalid", fruit("durian"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.val, allFruits...); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestIsValid_IntEnum(t *testing.T) {
	if !IsValid(priorityLow, priorityLow, priorityHigh) {
		t.Error("priorityLow should be valid")
	}
	if IsValid(priority(99), priorityLow, priorityHigh) {
		t.Error("unknown priority should be invalid")
	}
}

func TestIsValid_EmptyAllowed(t *testing.T) {
	// An enum with no declared valid values should reject everything,
	// matching the original "switch with no cases" behaviour.
	if IsValid(fruitApple) {
		t.Error("IsValid with no allowed values should always return false")
	}
}
