package enums

import (
	"testing"
)

func TestEnumRegistry(t *testing.T) {
	registry := NewEnumRegistry()

	info := EnumInfo{
		Name:         "Test Enum",
		Description:  "A test enum",
		Values:       []string{"a", "b", "c"},
		Labels:       map[string]string{"a": "Option A", "b": "Option B", "c": "Option C"},
		DefaultValue: "a",
	}

	registry.Register("test", info)

	retrieved, ok := registry.Get("test")
	if !ok {
		t.Fatal("Expected to find registered enum")
	}

	if retrieved.Name != "Test Enum" {
		t.Errorf("Expected 'Test Enum', got %s", retrieved.Name)
	}

	if len(retrieved.Values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(retrieved.Values))
	}
}

func TestEnumRegistryIsValidValue(t *testing.T) {
	registry := NewEnumRegistry()

	registry.Register("test", EnumInfo{
		Values: []string{"a", "b", "c"},
	})

	if !registry.IsValidValue("test", "a") {
		t.Error("Expected 'a' to be valid")
	}

	if registry.IsValidValue("test", "x") {
		t.Error("Expected 'x' to be invalid")
	}

	if registry.IsValidValue("nonexistent", "a") {
		t.Error("Expected nonexistent enum to return false")
	}
}

func TestEnumRegistryGetLabel(t *testing.T) {
	registry := NewEnumRegistry()

	registry.Register("test", EnumInfo{
		Values: []string{"a", "b"},
		Labels: map[string]string{"a": "Option A"},
	})

	if label := registry.GetLabel("test", "a"); label != "Option A" {
		t.Errorf("Expected 'Option A', got %s", label)
	}

	// No label defined, should return value itself
	if label := registry.GetLabel("test", "b"); label != "b" {
		t.Errorf("Expected 'b', got %s", label)
	}

	// Nonexistent enum
	if label := registry.GetLabel("nonexistent", "a"); label != "a" {
		t.Errorf("Expected 'a', got %s", label)
	}
}

func TestEnumRegistryByType(t *testing.T) {
	registry := NewEnumRegistry()

	registry.Register("test", EnumInfo{
		Name:   "Test Enum",
		Values: []string{"a", "b"},
	})

	registry.RegisterType("TestEnum", "test")

	info, ok := registry.GetByType("TestEnum")
	if !ok {
		t.Fatal("Expected to find enum by type")
	}

	if info.Name != "Test Enum" {
		t.Errorf("Expected 'Test Enum', got %s", info.Name)
	}

	_, ok = registry.GetByType("NonExistent")
	if ok {
		t.Error("Expected not to find nonexistent type")
	}
}

func TestEnumRegistryAll(t *testing.T) {
	registry := NewEnumRegistry()

	registry.Register("enum1", EnumInfo{Name: "Enum 1"})
	registry.Register("enum2", EnumInfo{Name: "Enum 2"})

	all := registry.All()
	if len(all) != 2 {
		t.Errorf("Expected 2 enums, got %d", len(all))
	}
}

func TestEnumRegistryKeys(t *testing.T) {
	registry := NewEnumRegistry()

	registry.Register("enum1", EnumInfo{})
	registry.Register("enum2", EnumInfo{})

	keys := registry.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestEnumBuilder(t *testing.T) {
	def := NewEnumBuilder[TestEnum]("builder_test").
		Name("Builder Test Enum").
		Description("Testing the builder").
		Values(TestEnumA, TestEnumB, TestEnumC).
		WithLabel(TestEnumA, "Custom A").
		WithLabels(map[TestEnum]string{
			TestEnumB: "Custom B",
			TestEnumC: "Custom C",
		}).
		Default(TestEnumA).
		Build()

	if len(def.Values()) != 3 {
		t.Errorf("Expected 3 values, got %d", len(def.Values()))
	}

	if def.Label(TestEnumA) != "Custom A" {
		t.Errorf("Expected 'Custom A', got %s", def.Label(TestEnumA))
	}
}

func TestEnumBuilderRegister(t *testing.T) {
	// Clear global registry for this test
	globalRegistry = NewEnumRegistry()

	NewEnumBuilder[TestEnum]("registered_test").
		Name("Registered Test").
		Description("A registered enum").
		Values(TestEnumA, TestEnumB).
		WithLabel(TestEnumA, "Registered A").
		Register()

	info, ok := GlobalRegistry().Get("registered_test")
	if !ok {
		t.Fatal("Expected to find registered enum")
	}

	if info.Name != "Registered Test" {
		t.Errorf("Expected 'Registered Test', got %s", info.Name)
	}

	if info.Labels["a"] != "Registered A" {
		t.Errorf("Expected 'Registered A', got %s", info.Labels["a"])
	}
}

func TestRegisterEnum(t *testing.T) {
	// Clear global registry for this test
	globalRegistry = NewEnumRegistry()

	def := NewEnumDefinition(testEnumValues, map[TestEnum]string{
		TestEnumA: "Def A",
		TestEnumB: "Def B",
		TestEnumC: "Def C",
	})

	RegisterEnum("def_test", "Definition Test", "Testing RegisterEnum", def)

	info, ok := GlobalRegistry().Get("def_test")
	if !ok {
		t.Fatal("Expected to find registered enum")
	}

	if info.Name != "Definition Test" {
		t.Errorf("Expected 'Definition Test', got %s", info.Name)
	}

	if len(info.Values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(info.Values))
	}
}
