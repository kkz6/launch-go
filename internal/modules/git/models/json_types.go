package models

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONMap is a helper type for storing JSON data in the database
type JSONMap map[string]interface{}

// Value implements driver.Valuer
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}

	return json.Marshal(j)
}

// Scan implements sql.Scanner
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil
	}

	return json.Unmarshal(data, j)
}

// JSONArray is a helper type for storing JSON arrays in the database
type JSONArray []interface{}

// Value implements driver.Valuer
func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}

	return json.Marshal(j)
}

// Scan implements sql.Scanner
func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil
	}

	return json.Unmarshal(data, j)
}
