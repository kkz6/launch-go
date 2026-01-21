package service

import (
	"testing"

	"github.com/rs/zerolog"
)

// MockRepos is a test repository registry
type MockRepos struct {
	name string
}

// MockServiceRegistry is a test service registry
type MockServiceRegistry struct {
	initialized bool
}

func TestNewModuleDeps(t *testing.T) {
	logger := zerolog.Nop()
	repos := &MockRepos{name: "test"}

	deps := ModuleDeps[*MockRepos]{
		Dependencies: Dependencies{
			Logger: &logger,
		},
		Repos: repos,
	}

	if deps.Logger == nil {
		t.Error("Expected logger to be set")
	}
	if deps.Repos == nil {
		t.Error("Expected repos to be set")
	}
	if deps.Repos.name != "test" {
		t.Errorf("Expected repos name to be 'test', got '%s'", deps.Repos.name)
	}
}

func TestNewModuleBase(t *testing.T) {
	logger := zerolog.Nop()
	repos := &MockRepos{name: "test"}

	deps := &ModuleDeps[*MockRepos]{
		Dependencies: Dependencies{
			Logger: &logger,
		},
		Repos: repos,
	}

	base := NewModuleBase(deps)

	if base.Repos() == nil {
		t.Error("Expected repos to be accessible via Repos()")
	}
	if base.Repos().name != "test" {
		t.Errorf("Expected repos name to be 'test', got '%s'", base.Repos().name)
	}
	if base.Logger == nil {
		t.Error("Expected logger to be accessible via embedded Base")
	}
}

func TestModuleDeps_Deps(t *testing.T) {
	repos := &MockRepos{name: "test"}

	deps := &ModuleDeps[*MockRepos]{
		Repos: repos,
	}

	result := deps.Deps()
	if result != deps {
		t.Error("Expected Deps() to return the same pointer")
	}
}

func TestModuleDepsWithRegistry(t *testing.T) {
	logger := zerolog.Nop()
	repos := &MockRepos{name: "test"}
	registry := &MockServiceRegistry{initialized: true}

	deps := ModuleDepsWithRegistry[*MockRepos, *MockServiceRegistry]{
		ModuleDeps: ModuleDeps[*MockRepos]{
			Dependencies: Dependencies{
				Logger: &logger,
			},
			Repos: repos,
		},
		Registry: registry,
	}

	if deps.Logger == nil {
		t.Error("Expected logger to be set")
	}
	if deps.Repos == nil {
		t.Error("Expected repos to be set")
	}
	if deps.Registry == nil {
		t.Error("Expected registry to be set")
	}
	if !deps.Registry.initialized {
		t.Error("Expected registry to be initialized")
	}
}

func TestNewModuleBaseWithRegistry(t *testing.T) {
	logger := zerolog.Nop()
	repos := &MockRepos{name: "test"}
	registry := &MockServiceRegistry{initialized: true}

	deps := &ModuleDepsWithRegistry[*MockRepos, *MockServiceRegistry]{
		ModuleDeps: ModuleDeps[*MockRepos]{
			Dependencies: Dependencies{
				Logger: &logger,
			},
			Repos: repos,
		},
		Registry: registry,
	}

	base := NewModuleBaseWithRegistry(deps)

	if base.Repos() == nil {
		t.Error("Expected repos to be accessible via Repos()")
	}
	if base.Repos().name != "test" {
		t.Errorf("Expected repos name to be 'test', got '%s'", base.Repos().name)
	}
	if base.Services() == nil {
		t.Error("Expected services to be accessible via Services()")
	}
	if !base.Services().initialized {
		t.Error("Expected services to be initialized")
	}
	if base.Logger == nil {
		t.Error("Expected logger to be accessible via embedded Base")
	}
}

func TestModuleBaseWithRegistry_SetRegistry(t *testing.T) {
	repos := &MockRepos{name: "test"}

	deps := &ModuleDepsWithRegistry[*MockRepos, *MockServiceRegistry]{
		ModuleDeps: ModuleDeps[*MockRepos]{
			Repos: repos,
		},
	}

	base := NewModuleBaseWithRegistry(deps)

	// Initially nil
	if base.Services() != nil {
		t.Error("Expected services to be nil initially")
	}

	// Set registry
	newRegistry := &MockServiceRegistry{initialized: true}
	base.SetRegistry(newRegistry)

	if base.Services() == nil {
		t.Error("Expected services to be set after SetRegistry")
	}
	if !base.Services().initialized {
		t.Error("Expected services to be initialized after SetRegistry")
	}
	if deps.Registry == nil {
		t.Error("Expected deps.Registry to also be updated")
	}
}

func TestModuleBaseWithRegistry_ModuleDepsPtr(t *testing.T) {
	repos := &MockRepos{name: "test"}

	deps := &ModuleDepsWithRegistry[*MockRepos, *MockServiceRegistry]{
		ModuleDeps: ModuleDeps[*MockRepos]{
			Repos: repos,
		},
	}

	base := NewModuleBaseWithRegistry(deps)

	result := base.ModuleDepsPtr()
	if result != deps {
		t.Error("Expected ModuleDepsPtr() to return the original deps pointer")
	}
}

// Test that the generic types work with different type parameters
type AnotherRepos struct {
	count int
}

type AnotherRegistry struct {
	version string
}

func TestGenericTypes_DifferentTypeParameters(t *testing.T) {
	repos := &AnotherRepos{count: 42}
	registry := &AnotherRegistry{version: "1.0"}

	deps := &ModuleDepsWithRegistry[*AnotherRepos, *AnotherRegistry]{
		ModuleDeps: ModuleDeps[*AnotherRepos]{
			Repos: repos,
		},
		Registry: registry,
	}

	base := NewModuleBaseWithRegistry(deps)

	if base.Repos().count != 42 {
		t.Errorf("Expected repos count to be 42, got %d", base.Repos().count)
	}
	if base.Services().version != "1.0" {
		t.Errorf("Expected services version to be '1.0', got '%s'", base.Services().version)
	}
}
