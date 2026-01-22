package dbtype

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/kkz6/launch-go/internal/database/serializers"
)

// EncryptedString is a string that automatically encrypts when stored and decrypts when read.
// Use this instead of *string with serializer:encrypted tag.
//
// Usage:
//
//	type MyModel struct {
//	    PrivateKey dbtype.EncryptedString `gorm:"type:longtext" json:"-"`
//	}
//
//	// Direct access:
//	model.PrivateKey = "my-secret-key"
//	value := model.PrivateKey
type EncryptedString string

// Value implements driver.Valuer - encrypts the string when storing to database
func (e EncryptedString) Value() (driver.Value, error) {
	if e == "" {
		return nil, nil
	}
	return serializers.Encrypt(string(e))
}

// Scan implements sql.Scanner - decrypts the string when reading from database
func (e *EncryptedString) Scan(value interface{}) error {
	if value == nil {
		*e = ""
		return nil
	}

	var encrypted string
	switch v := value.(type) {
	case []byte:
		encrypted = string(v)
	case string:
		encrypted = v
	default:
		return errors.New("unsupported type for EncryptedString")
	}

	if encrypted == "" {
		*e = ""
		return nil
	}

	decrypted, err := serializers.Decrypt(encrypted)
	if err != nil {
		return err
	}

	*e = EncryptedString(decrypted)
	return nil
}

// String returns the decrypted string value
func (e EncryptedString) String() string {
	return string(e)
}

// IsEmpty returns true if the encrypted string is empty
func (e EncryptedString) IsEmpty() bool {
	return e == ""
}

// Ptr returns a pointer to the EncryptedString (useful for optional fields)
func (e EncryptedString) Ptr() *EncryptedString {
	return &e
}

// EncryptedNullableString is a nullable encrypted string (for optional fields).
// Use when the field can be NULL in the database.
//
// Usage:
//
//	type MyModel struct {
//	    PrivateKey *dbtype.EncryptedNullableString `gorm:"type:longtext" json:"-"`
//	}
type EncryptedNullableString struct {
	String string
	Valid  bool // Valid is true if String is not NULL
}

// Value implements driver.Valuer - encrypts the string when storing to database
func (e EncryptedNullableString) Value() (driver.Value, error) {
	if !e.Valid || e.String == "" {
		return nil, nil
	}
	return serializers.Encrypt(e.String)
}

// Scan implements sql.Scanner - decrypts the string when reading from database
func (e *EncryptedNullableString) Scan(value interface{}) error {
	if value == nil {
		e.String = ""
		e.Valid = false
		return nil
	}

	var encrypted string
	switch v := value.(type) {
	case []byte:
		encrypted = string(v)
	case string:
		encrypted = v
	default:
		return errors.New("unsupported type for EncryptedNullableString")
	}

	if encrypted == "" {
		e.String = ""
		e.Valid = false
		return nil
	}

	decrypted, err := serializers.Decrypt(encrypted)
	if err != nil {
		return err
	}

	e.String = decrypted
	e.Valid = true
	return nil
}

// Set sets the value and marks it as valid
func (e *EncryptedNullableString) Set(s string) {
	e.String = s
	e.Valid = true
}

// Clear clears the value and marks it as invalid (NULL)
func (e *EncryptedNullableString) Clear() {
	e.String = ""
	e.Valid = false
}

// EncryptedJSONStringMap is a map[string]string that is stored as encrypted JSON in the database.
// Use this for credentials and other sensitive key-value data.
//
// Usage:
//
//	type MyModel struct {
//	    Credentials dbtype.EncryptedJSONStringMap `gorm:"type:longtext" json:"-"`
//	}
//
//	// Direct access:
//	model.Credentials = dbtype.EncryptedJSONStringMap{"api_key": "secret"}
//	value := model.Credentials["api_key"]
type EncryptedJSONStringMap map[string]string

// Value implements driver.Valuer - JSON marshals and encrypts when storing to database
func (e EncryptedJSONStringMap) Value() (driver.Value, error) {
	if e == nil {
		return nil, nil
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	// Encrypt the JSON string
	return serializers.Encrypt(string(jsonBytes))
}

// Scan implements sql.Scanner - decrypts and JSON unmarshals when reading from database
func (e *EncryptedJSONStringMap) Scan(value interface{}) error {
	if value == nil {
		*e = nil
		return nil
	}

	var encrypted string
	switch v := value.(type) {
	case []byte:
		encrypted = string(v)
	case string:
		encrypted = v
	default:
		return errors.New("unsupported type for EncryptedJSONStringMap")
	}

	if encrypted == "" {
		*e = nil
		return nil
	}

	// Decrypt the value
	decrypted, err := serializers.Decrypt(encrypted)
	if err != nil {
		return err
	}

	// Try to unmarshal JSON - if it fails, the data might still be in Laravel format
	// or double-encrypted, return empty map instead of error
	if err := json.Unmarshal([]byte(decrypted), e); err != nil {
		// Data might not be migrated yet - return empty map
		*e = make(EncryptedJSONStringMap)
		return nil
	}

	return nil
}
