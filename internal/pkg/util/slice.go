package util

// Unique returns a new slice containing only unique elements.
// The order of first occurrence is preserved.
//
// Example:
//
//	Unique([]int{1, 2, 2, 3, 1}) // Returns []int{1, 2, 3}
//	Unique([]string{"a", "b", "a"}) // Returns []string{"a", "b"}
func Unique[T comparable](items []T) []T {
	if items == nil {
		return nil
	}
	seen := make(map[T]struct{}, len(items))
	result := make([]T, 0, len(items))
	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// First returns the first element of a slice and a boolean indicating success.
// Returns the zero value and false if the slice is empty or nil.
//
// Example:
//
//	First([]int{1, 2, 3})  // Returns 1, true
//	First([]int{})         // Returns 0, false
//	First[int](nil)        // Returns 0, false
func First[T any](items []T) (T, bool) {
	if len(items) == 0 {
		var zero T
		return zero, false
	}
	return items[0], true
}

// FirstOr returns the first element of a slice, or the default value if empty.
//
// Example:
//
//	FirstOr([]int{1, 2, 3}, 0)  // Returns 1
//	FirstOr([]int{}, 99)        // Returns 99
func FirstOr[T any](items []T, defaultVal T) T {
	if len(items) == 0 {
		return defaultVal
	}
	return items[0]
}

// Last returns the last element of a slice and a boolean indicating success.
// Returns the zero value and false if the slice is empty or nil.
//
// Example:
//
//	Last([]int{1, 2, 3})  // Returns 3, true
//	Last([]int{})         // Returns 0, false
//	Last[int](nil)        // Returns 0, false
func Last[T any](items []T) (T, bool) {
	if len(items) == 0 {
		var zero T
		return zero, false
	}
	return items[len(items)-1], true
}

// LastOr returns the last element of a slice, or the default value if empty.
//
// Example:
//
//	LastOr([]int{1, 2, 3}, 0)  // Returns 3
//	LastOr([]int{}, 99)        // Returns 99
func LastOr[T any](items []T, defaultVal T) T {
	if len(items) == 0 {
		return defaultVal
	}
	return items[len(items)-1]
}

// Filter returns a new slice containing only elements that satisfy the predicate.
//
// Example:
//
//	Filter([]int{1, 2, 3, 4}, func(n int) bool { return n%2 == 0 })
//	// Returns []int{2, 4}
func Filter[T any](items []T, predicate func(T) bool) []T {
	if items == nil {
		return nil
	}
	result := make([]T, 0, len(items))
	for _, item := range items {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforms each element of a slice using the provided function.
//
// Example:
//
//	Map([]int{1, 2, 3}, func(n int) int { return n * 2 })
//	// Returns []int{2, 4, 6}
func Map[T, R any](items []T, fn func(T) R) []R {
	if items == nil {
		return nil
	}
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}
	return result
}

// Reduce reduces a slice to a single value using the provided function.
//
// Example:
//
//	Reduce([]int{1, 2, 3, 4}, 0, func(acc, n int) int { return acc + n })
//	// Returns 10
func Reduce[T, R any](items []T, initial R, fn func(R, T) R) R {
	result := initial
	for _, item := range items {
		result = fn(result, item)
	}
	return result
}

// Find returns the first element that satisfies the predicate.
// Returns the zero value and false if no element is found.
//
// Example:
//
//	Find([]int{1, 2, 3, 4}, func(n int) bool { return n > 2 })
//	// Returns 3, true
func Find[T any](items []T, predicate func(T) bool) (T, bool) {
	for _, item := range items {
		if predicate(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// FindIndex returns the index of the first element that satisfies the predicate.
// Returns -1 if no element is found.
//
// Example:
//
//	FindIndex([]int{1, 2, 3, 4}, func(n int) bool { return n > 2 })
//	// Returns 2
func FindIndex[T any](items []T, predicate func(T) bool) int {
	for i, item := range items {
		if predicate(item) {
			return i
		}
	}
	return -1
}

// Any returns true if any element satisfies the predicate.
//
// Example:
//
//	Any([]int{1, 2, 3}, func(n int) bool { return n > 2 })
//	// Returns true
func Any[T any](items []T, predicate func(T) bool) bool {
	for _, item := range items {
		if predicate(item) {
			return true
		}
	}
	return false
}

// All returns true if all elements satisfy the predicate.
// Returns true for empty slices.
//
// Example:
//
//	All([]int{2, 4, 6}, func(n int) bool { return n%2 == 0 })
//	// Returns true
func All[T any](items []T, predicate func(T) bool) bool {
	for _, item := range items {
		if !predicate(item) {
			return false
		}
	}
	return true
}

// None returns true if no element satisfies the predicate.
// Returns true for empty slices.
//
// Example:
//
//	None([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 })
//	// Returns true
func None[T any](items []T, predicate func(T) bool) bool {
	for _, item := range items {
		if predicate(item) {
			return false
		}
	}
	return true
}

// Chunk splits a slice into chunks of the specified size.
//
// Example:
//
//	Chunk([]int{1, 2, 3, 4, 5}, 2)
//	// Returns [][]int{{1, 2}, {3, 4}, {5}}
func Chunk[T any](items []T, size int) [][]T {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	chunks := make([][]T, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// Flatten flattens a slice of slices into a single slice.
//
// Example:
//
//	Flatten([][]int{{1, 2}, {3, 4}, {5}})
//	// Returns []int{1, 2, 3, 4, 5}
func Flatten[T any](items [][]T) []T {
	if items == nil {
		return nil
	}
	var totalLen int
	for _, item := range items {
		totalLen += len(item)
	}
	result := make([]T, 0, totalLen)
	for _, item := range items {
		result = append(result, item...)
	}
	return result
}

// Reverse returns a new slice with elements in reverse order.
//
// Example:
//
//	Reverse([]int{1, 2, 3})
//	// Returns []int{3, 2, 1}
func Reverse[T any](items []T) []T {
	if items == nil {
		return nil
	}
	result := make([]T, len(items))
	for i, item := range items {
		result[len(items)-1-i] = item
	}
	return result
}

// Compact removes zero values from a slice.
//
// Example:
//
//	Compact([]string{"a", "", "b", "", "c"})
//	// Returns []string{"a", "b", "c"}
//	Compact([]int{1, 0, 2, 0, 3})
//	// Returns []int{1, 2, 3}
func Compact[T comparable](items []T) []T {
	if items == nil {
		return nil
	}
	var zero T
	result := make([]T, 0, len(items))
	for _, item := range items {
		if item != zero {
			result = append(result, item)
		}
	}
	return result
}

// Index returns the index of the first occurrence of the element.
// Returns -1 if the element is not found.
//
// Example:
//
//	Index([]string{"a", "b", "c"}, "b")
//	// Returns 1
func Index[T comparable](items []T, element T) int {
	for i, item := range items {
		if item == element {
			return i
		}
	}
	return -1
}

// ContainsAny returns true if the slice contains any of the given elements.
//
// Example:
//
//	ContainsAny([]string{"a", "b", "c"}, "b", "d")
//	// Returns true
func ContainsAny[T comparable](items []T, elements ...T) bool {
	set := make(map[T]struct{}, len(elements))
	for _, e := range elements {
		set[e] = struct{}{}
	}
	for _, item := range items {
		if _, exists := set[item]; exists {
			return true
		}
	}
	return false
}

// ContainsAll returns true if the slice contains all of the given elements.
//
// Example:
//
//	ContainsAll([]string{"a", "b", "c"}, "a", "b")
//	// Returns true
func ContainsAll[T comparable](items []T, elements ...T) bool {
	if len(elements) == 0 {
		return true
	}
	set := make(map[T]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	for _, e := range elements {
		if _, exists := set[e]; !exists {
			return false
		}
	}
	return true
}

// FilterTransform filters a slice and transforms matching elements in a single pass.
// The predicate and transform functions receive pointers to avoid copying large structs.
// Pre-allocates with an estimate of half the elements matching.
//
// Example:
//
//	type User struct { Name string; Age int; Active bool }
//	users := []User{{Name: "Alice", Age: 30, Active: true}, {Name: "Bob", Age: 25, Active: false}}
//	names := FilterTransform(users, func(u *User) bool { return u.Active }, func(u *User) string { return u.Name })
//	// Returns []string{"Alice"}
func FilterTransform[T, U any](slice []T, predicate func(*T) bool, transform func(*T) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, 0, len(slice)/2+1)
	for i := range slice {
		if predicate(&slice[i]) {
			result = append(result, transform(&slice[i]))
		}
	}
	return result
}

// FilterPtr returns elements that match the predicate.
// The predicate receives a pointer to each element to avoid copying large structs.
// Pre-allocates with an estimate of half the elements matching.
//
// Example:
//
//	type User struct { Name string; Active bool }
//	users := []User{{Name: "Alice", Active: true}, {Name: "Bob", Active: false}}
//	active := FilterPtr(users, func(u *User) bool { return u.Active })
//	// Returns []User{{Name: "Alice", Active: true}}
func FilterPtr[T any](slice []T, predicate func(*T) bool) []T {
	if slice == nil {
		return nil
	}
	result := make([]T, 0, len(slice)/2+1)
	for i := range slice {
		if predicate(&slice[i]) {
			result = append(result, slice[i])
		}
	}
	return result
}
