package dto

// ConvertSlice converts a slice of models to a slice of response DTOs.
// It uses the provided converter function to transform each element.
//
// Example usage:
//
//	users := []models.User{...}
//	responses := dto.ConvertSlice(users, ToUserResponse)
func ConvertSlice[M, R any](models []M, convert func(*M) R) []R {
	if models == nil {
		return nil
	}
	results := make([]R, len(models))
	for i := range models {
		results[i] = convert(&models[i])
	}
	return results
}

// ConvertSlicePtr converts a slice of models to a slice of response DTO pointers.
//
// Example usage:
//
//	users := []models.User{...}
//	responses := dto.ConvertSlicePtr(users, ToUserResponse)
func ConvertSlicePtr[M, R any](models []M, convert func(*M) *R) []*R {
	if models == nil {
		return nil
	}
	results := make([]*R, len(models))
	for i := range models {
		results[i] = convert(&models[i])
	}
	return results
}

// ConvertPtr safely converts a pointer, returning nil for nil input.
//
// Example usage:
//
//	user := (*models.User)(nil)
//	response := dto.ConvertPtr(user, ToUserResponse) // returns nil
func ConvertPtr[M, R any](model *M, convert func(*M) *R) *R {
	if model == nil {
		return nil
	}
	return convert(model)
}

// ConvertPtrValue safely converts a pointer to a value type, returning zero value for nil input.
//
// Example usage:
//
//	user := (*models.User)(nil)
//	response := dto.ConvertPtrValue(user, ToUserResponse) // returns zero value of R
func ConvertPtrValue[M, R any](model *M, convert func(*M) R) R {
	if model == nil {
		var zero R
		return zero
	}
	return convert(model)
}

// MapSlice applies a transformation function to each element in a slice.
// This is a more generic version of ConvertSlice that works with any transformation.
//
// Example usage:
//
//	ids := dto.MapSlice(users, func(u *models.User) string { return u.ID })
func MapSlice[M, R any](models []M, fn func(*M) R) []R {
	if models == nil {
		return nil
	}
	results := make([]R, len(models))
	for i := range models {
		results[i] = fn(&models[i])
	}
	return results
}

// FilterSlice filters a slice based on a predicate function.
//
// Example usage:
//
//	activeUsers := dto.FilterSlice(users, func(u *models.User) bool { return u.IsActive })
func FilterSlice[M any](models []M, predicate func(*M) bool) []M {
	if models == nil {
		return nil
	}
	var results []M
	for i := range models {
		if predicate(&models[i]) {
			results = append(results, models[i])
		}
	}
	return results
}

// FirstOrNil returns the first element of a slice or nil if empty.
// Useful for queries that return a slice but expect at most one result.
func FirstOrNil[T any](slice []T) *T {
	if len(slice) == 0 {
		return nil
	}
	return &slice[0]
}

// SafeDeref safely dereferences a pointer, returning zero value for nil.
func SafeDeref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// Ptr returns a pointer to the given value.
// Useful for creating pointers to literals or temporary values.
//
// Deprecated: Use ptr.Ptr from internal/pkg/ptr instead.
func Ptr[T any](v T) *T {
	return &v
}

// MapSliceValue applies a transformation function to each element value in a slice.
// Unlike MapSlice which passes pointers, this passes values.
//
// Example usage:
//
//	names := dto.MapSliceValue(users, func(u models.User) string { return u.Name })
func MapSliceValue[M, R any](models []M, fn func(M) R) []R {
	if models == nil {
		return nil
	}
	results := make([]R, len(models))
	for i, m := range models {
		results[i] = fn(m)
	}
	return results
}

// Reduce reduces a slice to a single value using an accumulator function.
//
// Example usage:
//
//	total := dto.Reduce(items, 0, func(acc int, i *Item) int { return acc + i.Price })
func Reduce[M, R any](models []M, initial R, fn func(R, *M) R) R {
	result := initial
	for i := range models {
		result = fn(result, &models[i])
	}
	return result
}

// ReduceValue reduces a slice to a single value (value-based version).
//
// Example usage:
//
//	total := dto.ReduceValue(prices, 0, func(acc, price int) int { return acc + price })
func ReduceValue[M, R any](models []M, initial R, fn func(R, M) R) R {
	result := initial
	for _, m := range models {
		result = fn(result, m)
	}
	return result
}

// GroupBy groups slice elements by a key function.
//
// Example usage:
//
//	byStatus := dto.GroupBy(orders, func(o *Order) string { return o.Status })
func GroupBy[M any, K comparable](models []M, keyFn func(*M) K) map[K][]M {
	if models == nil {
		return nil
	}
	result := make(map[K][]M)
	for i := range models {
		key := keyFn(&models[i])
		result[key] = append(result[key], models[i])
	}
	return result
}

// Unique returns a slice with duplicate elements removed.
// Uses a key function to determine uniqueness.
//
// Example usage:
//
//	uniqueByID := dto.Unique(items, func(i *Item) string { return i.ID })
func Unique[M any, K comparable](models []M, keyFn func(*M) K) []M {
	if models == nil {
		return nil
	}
	seen := make(map[K]bool)
	var result []M
	for i := range models {
		key := keyFn(&models[i])
		if !seen[key] {
			seen[key] = true
			result = append(result, models[i])
		}
	}
	return result
}

// Contains checks if a slice contains an element matching the predicate.
//
// Example usage:
//
//	hasAdmin := dto.Contains(users, func(u *User) bool { return u.Role == "admin" })
func Contains[M any](models []M, predicate func(*M) bool) bool {
	for i := range models {
		if predicate(&models[i]) {
			return true
		}
	}
	return false
}

// Find returns the first element matching the predicate, or nil if not found.
//
// Example usage:
//
//	admin := dto.Find(users, func(u *User) bool { return u.Role == "admin" })
func Find[M any](models []M, predicate func(*M) bool) *M {
	for i := range models {
		if predicate(&models[i]) {
			return &models[i]
		}
	}
	return nil
}

// IndexOf returns the index of the first element matching the predicate, or -1 if not found.
//
// Example usage:
//
//	idx := dto.IndexOf(items, func(i *Item) bool { return i.ID == targetID })
func IndexOf[M any](models []M, predicate func(*M) bool) int {
	for i := range models {
		if predicate(&models[i]) {
			return i
		}
	}
	return -1
}

// Chunk splits a slice into chunks of the specified size.
//
// Example usage:
//
//	batches := dto.Chunk(items, 100) // [[100 items], [100 items], ...]
func Chunk[M any](models []M, size int) [][]M {
	if models == nil || size <= 0 {
		return nil
	}
	var result [][]M
	for i := 0; i < len(models); i += size {
		end := i + size
		if end > len(models) {
			end = len(models)
		}
		result = append(result, models[i:end])
	}
	return result
}

// Partition splits a slice into two based on a predicate.
// Returns (matching, nonMatching).
//
// Example usage:
//
//	active, inactive := dto.Partition(users, func(u *User) bool { return u.IsActive })
func Partition[M any](models []M, predicate func(*M) bool) (matching []M, nonMatching []M) {
	if models == nil {
		return nil, nil
	}
	for i := range models {
		if predicate(&models[i]) {
			matching = append(matching, models[i])
		} else {
			nonMatching = append(nonMatching, models[i])
		}
	}
	return matching, nonMatching
}
