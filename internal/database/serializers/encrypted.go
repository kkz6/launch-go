package serializers

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	"gorm.io/gorm/schema"
)

var (
	ErrKeyNotSet      = errors.New("encryption key not set")
	ErrInvalidKey     = errors.New("encryption key must be 32 bytes for AES-256")
	ErrDecryptFailed  = errors.New("decryption failed")
	ErrInvalidData    = errors.New("invalid encrypted data")
)

var (
	encryptionKey []byte
	keyMutex      sync.RWMutex
)

// SetEncryptionKey sets the global encryption key (must be 32 bytes for AES-256)
func SetEncryptionKey(key []byte) error {
	if len(key) != 32 {
		return ErrInvalidKey
	}
	keyMutex.Lock()
	defer keyMutex.Unlock()
	encryptionKey = make([]byte, 32)
	copy(encryptionKey, key)
	return nil
}

// SetEncryptionKeyFromBase64 sets the encryption key from a base64-encoded string
// Supports Laravel-style "base64:xxxxx" format
func SetEncryptionKeyFromBase64(keyStr string) error {
	// Strip "base64:" prefix if present (Laravel format)
	if len(keyStr) > 7 && keyStr[:7] == "base64:" {
		keyStr = keyStr[7:]
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return fmt.Errorf("failed to decode base64 key: %w", err)
	}

	return SetEncryptionKey(key)
}

func getKey() ([]byte, error) {
	keyMutex.RLock()
	defer keyMutex.RUnlock()
	if encryptionKey == nil {
		return nil, ErrKeyNotSet
	}
	return encryptionKey, nil
}

// EncryptedSerializer implements GORM's SerializerInterface for AES-256-GCM encryption
type EncryptedSerializer struct{}

func init() {
	schema.RegisterSerializer("encrypted", &EncryptedSerializer{})
}

// Scan decrypts the database value when reading
func (s *EncryptedSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) error {
	if dbValue == nil {
		return nil
	}

	var encrypted string
	switch v := dbValue.(type) {
	case string:
		encrypted = v
	case []byte:
		encrypted = string(v)
	default:
		return fmt.Errorf("encrypted serializer: unsupported type %T", dbValue)
	}

	if encrypted == "" {
		return nil
	}

	decrypted, err := decrypt(encrypted)
	if err != nil {
		return fmt.Errorf("encrypted serializer: %w", err)
	}

	// Set the decrypted value based on field type
	fieldValue := dst
	if dst.Kind() == reflect.Ptr {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		fieldValue = dst.Elem()
	}

	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(decrypted)
	case reflect.Slice:
		if fieldValue.Type().Elem().Kind() == reflect.Uint8 {
			fieldValue.SetBytes([]byte(decrypted))
		}
	default:
		return fmt.Errorf("encrypted serializer: unsupported field type %s", fieldValue.Kind())
	}

	return nil
}

// Value encrypts the field value when writing to database
func (s *EncryptedSerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue interface{}) (interface{}, error) {
	if fieldValue == nil {
		return nil, nil
	}

	var plaintext string
	switch v := fieldValue.(type) {
	case string:
		plaintext = v
	case []byte:
		plaintext = string(v)
	case *string:
		if v == nil {
			return nil, nil
		}
		plaintext = *v
	default:
		return nil, fmt.Errorf("encrypted serializer: unsupported type %T", fieldValue)
	}

	if plaintext == "" {
		return "", nil
	}

	encrypted, err := encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypted serializer: %w", err)
	}

	return encrypted, nil
}

// encrypt encrypts plaintext using AES-256-GCM and returns base64-encoded ciphertext
func encrypt(plaintext string) (string, error) {
	key, err := getKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt and prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts base64-encoded ciphertext using AES-256-GCM
func decrypt(encrypted string) (string, error) {
	key, err := getKey()
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrInvalidData
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidData
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}

	return string(plaintext), nil
}

// Encrypt is a helper function for manual encryption
func Encrypt(plaintext string) (string, error) {
	return encrypt(plaintext)
}

// Decrypt is a helper function for manual decryption
func Decrypt(encrypted string) (string, error) {
	return decrypt(encrypted)
}
