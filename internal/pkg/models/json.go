package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONMap is a map that can be stored in a JSON database column.
// It implements sql.Scanner and driver.Valuer for automatic serialization.
//
// Usage:
//
//	type MyModel struct {
//	    Data dbtype.JSONMap `gorm:"type:json" json:"data,omitempty"`
//	}
//
//	// Direct access:
//	model.Data = dbtype.JSONMap{"key": "value"}
//	value := model.Data["key"]
//	strValue := model.Data.GetString("key")
type JSONMap map[string]any

// Scan implements sql.Scanner for database deserialization
func (j *JSONMap) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	// Handle case where database contains an array instead of an object
	// This can happen with legacy data or empty arrays stored as "[]"
	trimmed := bytes
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n' || trimmed[0] == '\r') {
		trimmed = trimmed[1:]
	}

	if len(trimmed) > 0 && trimmed[0] == '[' {
		// It's an array - treat as nil/empty map
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer for database serialization
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}

	return json.Marshal(j)
}

// GetString safely gets a string value from the map
func (j JSONMap) GetString(key string) string {
	if j == nil {
		return ""
	}

	if v, ok := j[key].(string); ok {
		return v
	}

	return ""
}

// GetInt safely gets an int value from the map.
// Note: JSON numbers are unmarshaled as float64, so this handles the conversion.
func (j JSONMap) GetInt(key string) int {
	if j == nil {
		return 0
	}

	switch v := j[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}

	return 0
}

// GetInt64 safely gets an int64 value from the map
func (j JSONMap) GetInt64(key string) int64 {
	if j == nil {
		return 0
	}

	switch v := j[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}

	return 0
}

// GetFloat64 safely gets a float64 value from the map
func (j JSONMap) GetFloat64(key string) float64 {
	if j == nil {
		return 0
	}

	if v, ok := j[key].(float64); ok {
		return v
	}

	return 0
}

// GetBool safely gets a bool value from the map
func (j JSONMap) GetBool(key string) bool {
	if j == nil {
		return false
	}

	if v, ok := j[key].(bool); ok {
		return v
	}

	return false
}

// Has checks if a key exists in the map
func (j JSONMap) Has(key string) bool {
	if j == nil {
		return false
	}

	_, ok := j[key]
	return ok
}

// GetMap safely gets a nested map value
func (j JSONMap) GetMap(key string) JSONMap {
	if j == nil {
		return nil
	}

	if v, ok := j[key].(map[string]any); ok {
		return JSONMap(v)
	}

	return nil
}

// GetStringSlice safely gets a string slice from the map
func (j JSONMap) GetStringSlice(key string) []string {
	if j == nil {
		return nil
	}

	if v, ok := j[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}

		return result
	}

	return nil
}

// JSONStringMap is a map[string]string that can be stored in a JSON database column.
// Use this when you know all values will be strings for better type safety.
//
// Usage:
//
//	type MyModel struct {
//	    Credentials dbtype.JSONStringMap `gorm:"type:json" json:"-"`
//	}
type JSONStringMap map[string]string

// Scan implements sql.Scanner for database deserialization
func (j *JSONStringMap) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer for database serialization
func (j JSONStringMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}

	return json.Marshal(j)
}

// Get safely gets a value with a default fallback
func (j JSONStringMap) Get(key string, defaultValue string) string {
	if j == nil {
		return defaultValue
	}

	if v, ok := j[key]; ok {
		return v
	}

	return defaultValue
}

// Has checks if a key exists in the map
func (j JSONStringMap) Has(key string) bool {
	if j == nil {
		return false
	}

	_, ok := j[key]
	return ok
}

// JSONSlice is a generic slice that can be stored in a JSON database column.
//
// Usage:
//
//	type MyModel struct {
//	    Tags dbtype.JSONSlice[string] `gorm:"type:json" json:"tags,omitempty"`
//	    IDs  dbtype.JSONSlice[int]    `gorm:"type:json" json:"ids,omitempty"`
//	}
type JSONSlice[T any] []T

// Scan implements sql.Scanner for database deserialization
func (j *JSONSlice[T]) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer for database serialization
func (j JSONSlice[T]) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}

	return json.Marshal(j)
}

// JSONStringSlice is a []string that can be stored in a JSON database column.
// This is a convenience type alias for JSONSlice[string].
//
// Usage:
//
//	type MyModel struct {
//	    Steps dbtype.JSONStringSlice `gorm:"type:json" json:"steps,omitempty"`
//	}
type JSONStringSlice = JSONSlice[string]

// JSONArray is a slice of any values that can be stored in a JSON database column.
// Use this when you need heterogeneous array values.
//
// Usage:
//
//	type MyModel struct {
//	    Items dbtype.JSONArray `gorm:"type:json" json:"items,omitempty"`
//	}
type JSONArray []any

// Scan implements sql.Scanner for database deserialization
func (j *JSONArray) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer for database serialization
func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}

	return json.Marshal(j)
}
