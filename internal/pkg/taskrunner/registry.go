package taskrunner

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ListRegisteredTypes returns a sorted list of all registered task type names.
// Useful for debugging and validation.
func ListRegisteredTypes() []string {
	DefaultRegistry.mu.RLock()
	defer DefaultRegistry.mu.RUnlock()

	types := make([]string, 0, len(DefaultRegistry.factories))
	for typeName := range DefaultRegistry.factories {
		types = append(types, typeName)
	}
	sort.Strings(types)
	return types
}

// IsRegistered checks if a task type is registered
func IsRegistered(typeName string) bool {
	DefaultRegistry.mu.RLock()
	defer DefaultRegistry.mu.RUnlock()
	_, ok := DefaultRegistry.factories[typeName]
	return ok
}

// RegisteredCount returns the number of registered task types
func RegisteredCount() int {
	DefaultRegistry.mu.RLock()
	defer DefaultRegistry.mu.RUnlock()
	return len(DefaultRegistry.factories)
}

// MustRegister registers a task type and panics if it's already registered.
// Use this when duplicate registrations indicate a programming error.
func MustRegister(typeName string, factory TaskFactory) {
	DefaultRegistry.mu.Lock()
	defer DefaultRegistry.mu.Unlock()

	if _, exists := DefaultRegistry.factories[typeName]; exists {
		panic(fmt.Sprintf("task type already registered: %s", typeName))
	}
	DefaultRegistry.factories[typeName] = factory
}

// MustRegisterCallbackState is like RegisterCallbackState but panics on duplicate registration.
func MustRegisterCallbackState[S CallbackStateFactory](typeName string) {
	MustRegister(typeName, func(payload []byte) (CallbackHandler, error) {
		var state S
		if err := json.Unmarshal(payload, &state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state for %s: %w", typeName, err)
		}
		return state.NewTask(), nil
	})
}

// RegistryInfo holds information about the task registry
type RegistryInfo struct {
	TotalCount int
	Types      []string
}

// GetRegistryInfo returns information about the current registry state
func GetRegistryInfo() RegistryInfo {
	types := ListRegisteredTypes()
	return RegistryInfo{
		TotalCount: len(types),
		Types:      types,
	}
}

// ValidateRegistry checks for common registration issues.
// Returns nil if everything is valid, or an error describing the issues.
func ValidateRegistry(expectedTypes []string) error {
	registeredTypes := ListRegisteredTypes()
	registered := make(map[string]bool)
	for _, t := range registeredTypes {
		registered[t] = true
	}

	var missing []string
	for _, expected := range expectedTypes {
		if !registered[expected] {
			missing = append(missing, expected)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing task type registrations: %v", missing)
	}

	return nil
}

// ClearRegistry removes all registered task types.
// Only use this in tests.
func ClearRegistry() {
	DefaultRegistry.mu.Lock()
	defer DefaultRegistry.mu.Unlock()
	DefaultRegistry.factories = make(map[string]TaskFactory)
}
