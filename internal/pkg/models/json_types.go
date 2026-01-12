package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONMap is a map[string]interface{} that automatically serializes to/from JSON in the database.
// Use this instead of *string with getter/setter methods.
//
// Usage:
//
//	type MyModel struct {
//	    Data models.JSONMap `gorm:"type:json" json:"data,omitempty"`
//	}
//
//	// Direct access without getter/setter:
//	model.Data = models.JSONMap{"key": "value"}
//	value := model.Data["key"]
type JSONMap map[string]interface{}

// Value implements driver.Valuer for database serialization
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner for database deserialization
func (j *JSONMap) Scan(value interface{}) error {
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
		return errors.New("unsupported type for JSONMap")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// JSONStringMap is a map[string]string that automatically serializes to/from JSON in the database.
//
// Usage:
//
//	type MyModel struct {
//	    Credentials models.JSONStringMap `gorm:"type:json" json:"-"`
//	}
type JSONStringMap map[string]string

// Value implements driver.Valuer for database serialization
func (j JSONStringMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner for database deserialization
func (j *JSONStringMap) Scan(value interface{}) error {
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
		return errors.New("unsupported type for JSONStringMap")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// JSONStringSlice is a []string that automatically serializes to/from JSON in the database.
//
// Usage:
//
//	type MyModel struct {
//	    Steps models.JSONStringSlice `gorm:"type:json" json:"steps,omitempty"`
//	}
type JSONStringSlice []string

// Value implements driver.Valuer for database serialization
func (j JSONStringSlice) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner for database deserialization
func (j *JSONStringSlice) Scan(value interface{}) error {
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
		return errors.New("unsupported type for JSONStringSlice")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}
