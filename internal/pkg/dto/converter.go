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
func Ptr[T any](v T) *T {
	return &v
}
