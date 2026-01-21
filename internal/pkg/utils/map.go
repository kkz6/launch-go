package utils

// SafeGet retrieves a value from a map, returning the value and a boolean
// indicating whether the key was found. This is a generic wrapper around
// map access that provides explicit handling of missing keys.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	val, ok := SafeGet(m, "a") // Returns 1, true
//	val, ok := SafeGet(m, "c") // Returns 0, false
func SafeGet[K comparable, V any](m map[K]V, key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	val, ok := m[key]
	return val, ok
}

// SafeGetOr retrieves a value from a map, returning the default value
// if the key is not found or the map is nil.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	val := SafeGetOr(m, "a", 0)   // Returns 1
//	val := SafeGetOr(m, "c", 99)  // Returns 99
//	val := SafeGetOr(nil, "a", 0) // Returns 0
func SafeGetOr[K comparable, V any](m map[K]V, key K, defaultVal V) V {
	if m == nil {
		return defaultVal
	}
	val, ok := m[key]
	if !ok {
		return defaultVal
	}
	return val
}

// Keys returns all keys from a map as a slice.
// The order of keys is not guaranteed.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	keys := Keys(m) // Returns []string{"a", "b"} (order may vary)
func Keys[K comparable, V any](m map[K]V) []K {
	if m == nil {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values from a map as a slice.
// The order of values is not guaranteed.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	values := Values(m) // Returns []int{1, 2} (order may vary)
func Values[K comparable, V any](m map[K]V) []V {
	if m == nil {
		return nil
	}
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// Merge combines multiple maps into a new map. If a key exists in multiple maps,
// the value from the later map takes precedence.
//
// Example:
//
//	m1 := map[string]int{"a": 1, "b": 2}
//	m2 := map[string]int{"b": 3, "c": 4}
//	merged := Merge(m1, m2) // Returns {"a": 1, "b": 3, "c": 4}
func Merge[K comparable, V any](maps ...map[K]V) map[K]V {
	result := make(map[K]V)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// MapKeys transforms map keys using the provided function.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	upper := MapKeys(m, strings.ToUpper) // Returns {"A": 1, "B": 2}
func MapKeys[K1, K2 comparable, V any](m map[K1]V, fn func(K1) K2) map[K2]V {
	if m == nil {
		return nil
	}
	result := make(map[K2]V, len(m))
	for k, v := range m {
		result[fn(k)] = v
	}
	return result
}

// MapValues transforms map values using the provided function.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	doubled := MapValues(m, func(v int) int { return v * 2 }) // Returns {"a": 2, "b": 4}
func MapValues[K comparable, V1, V2 any](m map[K]V1, fn func(V1) V2) map[K]V2 {
	if m == nil {
		return nil
	}
	result := make(map[K]V2, len(m))
	for k, v := range m {
		result[k] = fn(v)
	}
	return result
}

// FilterMap returns a new map containing only entries that satisfy the predicate.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2, "c": 3}
//	even := FilterMap(m, func(k string, v int) bool { return v%2 == 0 })
//	// Returns {"b": 2}
func FilterMap[K comparable, V any](m map[K]V, predicate func(K, V) bool) map[K]V {
	if m == nil {
		return nil
	}
	result := make(map[K]V)
	for k, v := range m {
		if predicate(k, v) {
			result[k] = v
		}
	}
	return result
}

// HasKey checks if a key exists in the map.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	HasKey(m, "a") // Returns true
//	HasKey(m, "c") // Returns false
func HasKey[K comparable, V any](m map[K]V, key K) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// Invert swaps keys and values in a map.
// If there are duplicate values, later keys will overwrite earlier ones.
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	inverted := Invert(m) // Returns {1: "a", 2: "b"}
func Invert[K, V comparable](m map[K]V) map[V]K {
	if m == nil {
		return nil
	}
	result := make(map[V]K, len(m))
	for k, v := range m {
		result[v] = k
	}
	return result
}
