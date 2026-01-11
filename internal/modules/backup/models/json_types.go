package models

import (
	"encoding/json"
)

// JSON type for storing JSON arrays in the database
type JSON []byte

// Value implements driver.Valuer interface
func (j JSON) Value() (interface{}, error) {
	if len(j) == 0 {
		return "[]", nil
	}

	return string(j), nil
}

// Scan implements sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = []byte("[]")
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = v
	case string:
		*j = []byte(v)
	}

	return nil
}

// MarshalJSON implements json.Marshaler
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}

	return j, nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = data

	return nil
}

// ToStringSlice converts JSON to a string slice
func (j JSON) ToStringSlice() ([]string, error) {
	if len(j) == 0 {
		return []string{}, nil
	}

	var result []string
	if err := json.Unmarshal(j, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// FromStringSlice creates JSON from a string slice
func FromStringSlice(s []string) (JSON, error) {
	if s == nil {
		s = []string{}
	}

	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	return JSON(data), nil
}

// EncryptedJSON type for storing encrypted JSON in the database
// In production, this would use actual encryption with a key from config
type EncryptedJSON []byte

// NewEncryptedJSON creates a new EncryptedJSON from a map
func NewEncryptedJSON(data map[string]interface{}) (EncryptedJSON, error) {
	// In production, this would encrypt the data
	// For now, we just JSON encode it
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return EncryptedJSON(jsonData), nil
}

// Decrypt returns the decrypted credentials as a map
func (e EncryptedJSON) Decrypt() (map[string]interface{}, error) {
	if len(e) == 0 {
		return make(map[string]interface{}), nil
	}

	// In production, this would decrypt the data
	// For now, we just JSON decode it
	var result map[string]interface{}
	if err := json.Unmarshal(e, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Value implements driver.Valuer interface
func (e EncryptedJSON) Value() (interface{}, error) {
	if len(e) == 0 {
		return "", nil
	}

	return string(e), nil
}

// Scan implements sql.Scanner interface
func (e *EncryptedJSON) Scan(value interface{}) error {
	if value == nil {
		*e = []byte{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*e = v
	case string:
		*e = []byte(v)
	}

	return nil
}
