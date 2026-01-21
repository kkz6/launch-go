package enums

import (
	"sync"
)

// EnumRegistry provides a centralized registry for enum metadata.
// This is useful for:
// - API documentation generation
// - Form field validation
// - Dynamic UI generation
// - Centralized enum lookup
type EnumRegistry struct {
	mu     sync.RWMutex
	enums  map[string]EnumInfo
	byType map[string]string // maps type name to registry key
}

// EnumInfo holds metadata about a registered enum.
type EnumInfo struct {
	// Name is a human-readable name for the enum
	Name string

	// Description describes the purpose of the enum
	Description string

	// Values are the valid string values for this enum
	Values []string

	// Labels maps values to human-readable labels
	Labels map[string]string

	// DefaultValue is the default value, if any
	DefaultValue string
}

var (
	globalRegistry     *EnumRegistry
	globalRegistryOnce sync.Once
)

// GlobalRegistry returns the singleton enum registry.
func GlobalRegistry() *EnumRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewEnumRegistry()
	})
	return globalRegistry
}

// NewEnumRegistry creates a new enum registry.
func NewEnumRegistry() *EnumRegistry {
	return &EnumRegistry{
		enums:  make(map[string]EnumInfo),
		byType: make(map[string]string),
	}
}

// Register registers an enum with the given key and info.
func (r *EnumRegistry) Register(key string, info EnumInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enums[key] = info
}

// RegisterType registers an enum type name to a registry key.
// This allows looking up enum info by Go type name.
func (r *EnumRegistry) RegisterType(typeName, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byType[typeName] = key
}

// Get retrieves enum info by key.
func (r *EnumRegistry) Get(key string) (EnumInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.enums[key]
	return info, ok
}

// GetByType retrieves enum info by Go type name.
func (r *EnumRegistry) GetByType(typeName string) (EnumInfo, bool) {
	r.mu.RLock()
	key, ok := r.byType[typeName]
	r.mu.RUnlock()

	if !ok {
		return EnumInfo{}, false
	}

	return r.Get(key)
}

// All returns all registered enums.
func (r *EnumRegistry) All() map[string]EnumInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]EnumInfo, len(r.enums))
	for k, v := range r.enums {
		result[k] = v
	}
	return result
}

// Keys returns all registered enum keys.
func (r *EnumRegistry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.enums))
	for k := range r.enums {
		keys = append(keys, k)
	}
	return keys
}

// IsValidValue checks if a value is valid for the given enum key.
func (r *EnumRegistry) IsValidValue(key, value string) bool {
	info, ok := r.Get(key)
	if !ok {
		return false
	}

	for _, v := range info.Values {
		if v == value {
			return true
		}
	}
	return false
}

// GetLabel returns the label for a value, or the value itself if no label exists.
func (r *EnumRegistry) GetLabel(key, value string) string {
	info, ok := r.Get(key)
	if !ok {
		return value
	}

	if label, exists := info.Labels[value]; exists {
		return label
	}
	return value
}

// RegisterEnum is a convenience function that registers an enum definition
// to the global registry.
func RegisterEnum[T ~string](key, name, description string, def EnumDefinition[T]) {
	values := def.Values()
	stringValues := make([]string, len(values))
	labels := make(map[string]string, len(values))

	for i, v := range values {
		stringValues[i] = string(v)
		labels[string(v)] = def.Label(v)
	}

	GlobalRegistry().Register(key, EnumInfo{
		Name:        name,
		Description: description,
		Values:      stringValues,
		Labels:      labels,
	})
}

// EnumBuilder provides a fluent API for building and registering enums.
type EnumBuilder[T ~string] struct {
	key         string
	name        string
	description string
	values      []T
	labels      map[T]string
	defaultVal  T
}

// NewEnumBuilder creates a new enum builder.
func NewEnumBuilder[T ~string](key string) *EnumBuilder[T] {
	return &EnumBuilder[T]{
		key:    key,
		labels: make(map[T]string),
	}
}

// Name sets the enum name.
func (b *EnumBuilder[T]) Name(name string) *EnumBuilder[T] {
	b.name = name
	return b
}

// Description sets the enum description.
func (b *EnumBuilder[T]) Description(description string) *EnumBuilder[T] {
	b.description = description
	return b
}

// Values sets the enum values.
func (b *EnumBuilder[T]) Values(values ...T) *EnumBuilder[T] {
	b.values = values
	return b
}

// WithLabel adds a label for a specific value.
func (b *EnumBuilder[T]) WithLabel(value T, label string) *EnumBuilder[T] {
	b.labels[value] = label
	return b
}

// WithLabels sets multiple labels at once.
func (b *EnumBuilder[T]) WithLabels(labels map[T]string) *EnumBuilder[T] {
	for k, v := range labels {
		b.labels[k] = v
	}
	return b
}

// Default sets the default value.
func (b *EnumBuilder[T]) Default(value T) *EnumBuilder[T] {
	b.defaultVal = value
	return b
}

// Build creates an EnumDefinition from the builder.
func (b *EnumBuilder[T]) Build() EnumDefinition[T] {
	return NewEnumDefinition(b.values, b.labels)
}

// Register builds and registers the enum to the global registry.
func (b *EnumBuilder[T]) Register() EnumDefinition[T] {
	def := b.Build()

	stringValues := make([]string, len(b.values))
	labels := make(map[string]string, len(b.values))

	for i, v := range b.values {
		stringValues[i] = string(v)
		if label, ok := b.labels[v]; ok {
			labels[string(v)] = label
		} else {
			labels[string(v)] = string(v)
		}
	}

	GlobalRegistry().Register(b.key, EnumInfo{
		Name:         b.name,
		Description:  b.description,
		Values:       stringValues,
		Labels:       labels,
		DefaultValue: string(b.defaultVal),
	})

	return def
}
