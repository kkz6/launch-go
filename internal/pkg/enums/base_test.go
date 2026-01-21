package enums

import (
	"testing"
)

// TestEnum is a test enum type
type TestEnum string

const (
	TestEnumA TestEnum = "a"
	TestEnumB TestEnum = "b"
	TestEnumC TestEnum = "c"
)

func (t TestEnum) String() string {
	return string(t)
}

func (t TestEnum) IsValid() bool {
	switch t {
	case TestEnumA, TestEnumB, TestEnumC:
		return true
	}
	return false
}

func (t TestEnum) Label() string {
	labels := map[TestEnum]string{
		TestEnumA: "Option A",
		TestEnumB: "Option B",
		TestEnumC: "Option C",
	}
	if label, ok := labels[t]; ok {
		return label
	}
	return string(t)
}

var testEnumValues = []TestEnum{TestEnumA, TestEnumB, TestEnumC}

func TestContains(t *testing.T) {
	if !Contains(testEnumValues, TestEnumA) {
		t.Error("Expected Contains to return true for TestEnumA")
	}
	if Contains(testEnumValues, TestEnum("x")) {
		t.Error("Expected Contains to return false for invalid value")
	}
}

func TestParseEnum(t *testing.T) {
	val, err := ParseEnum("a", testEnumValues)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if val != TestEnumA {
		t.Errorf("Expected TestEnumA, got %v", val)
	}

	_, err = ParseEnum("invalid", testEnumValues)
	if err == nil {
		t.Error("Expected error for invalid value")
	}
}

func TestParseEnumWithDefault(t *testing.T) {
	val := ParseEnumWithDefault("a", testEnumValues, TestEnumB)
	if val != TestEnumA {
		t.Errorf("Expected TestEnumA, got %v", val)
	}

	val = ParseEnumWithDefault("invalid", testEnumValues, TestEnumB)
	if val != TestEnumB {
		t.Errorf("Expected default TestEnumB, got %v", val)
	}
}

func TestEnumDefinition(t *testing.T) {
	def := NewEnumDefinition(testEnumValues, map[TestEnum]string{
		TestEnumA: "Label A",
		TestEnumB: "Label B",
		TestEnumC: "Label C",
	})

	if len(def.Values()) != 3 {
		t.Errorf("Expected 3 values, got %d", len(def.Values()))
	}

	if def.Label(TestEnumA) != "Label A" {
		t.Errorf("Expected 'Label A', got %s", def.Label(TestEnumA))
	}

	if !def.IsValid(TestEnumA) {
		t.Error("Expected IsValid to return true for TestEnumA")
	}

	if def.IsValid(TestEnum("invalid")) {
		t.Error("Expected IsValid to return false for invalid value")
	}

	val, err := def.Parse("a")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if val != TestEnumA {
		t.Errorf("Expected TestEnumA, got %v", val)
	}

	_, err = def.Parse("invalid")
	if err == nil {
		t.Error("Expected error for invalid value")
	}

	val = def.ParseWithDefault("invalid", TestEnumB)
	if val != TestEnumB {
		t.Errorf("Expected default TestEnumB, got %v", val)
	}
}

func TestScanString(t *testing.T) {
	var result TestEnum

	err := ScanString(&result, "a")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != TestEnumA {
		t.Errorf("Expected TestEnumA, got %v", result)
	}

	err = ScanString(&result, []byte("b"))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != TestEnumB {
		t.Errorf("Expected TestEnumB, got %v", result)
	}

	err = ScanString(&result, nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty string, got %v", result)
	}

	err = ScanString(&result, 123)
	if err == nil {
		t.Error("Expected error for unsupported type")
	}
}

func TestValueString(t *testing.T) {
	val, err := ValueString(TestEnumA)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if val != "a" {
		t.Errorf("Expected 'a', got %v", val)
	}
}

func TestStringValues(t *testing.T) {
	values := StringValues(testEnumValues)
	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}
	if values[0] != "a" || values[1] != "b" || values[2] != "c" {
		t.Errorf("Unexpected values: %v", values)
	}
}

func TestFilter(t *testing.T) {
	filtered := Filter(testEnumValues, func(v TestEnum) bool {
		return v != TestEnumB
	})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 values, got %d", len(filtered))
	}
}

func TestFirst(t *testing.T) {
	first := First(testEnumValues, func(v TestEnum) bool {
		return v == TestEnumB
	})
	if first != TestEnumB {
		t.Errorf("Expected TestEnumB, got %v", first)
	}

	first = First(testEnumValues, func(v TestEnum) bool {
		return v == TestEnum("x")
	})
	if first != "" {
		t.Errorf("Expected empty string, got %v", first)
	}
}
