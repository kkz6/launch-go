package serializers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrLaravelInvalidPayload = errors.New("invalid laravel encrypted payload")
	ErrLaravelInvalidMAC     = errors.New("invalid laravel MAC")
)

// LaravelPayload represents the JSON structure of Laravel encrypted data
type LaravelPayload struct {
	IV    string `json:"iv"`
	Value string `json:"value"`
	MAC   string `json:"mac"`
}

// DecryptLaravel decrypts data encrypted by Laravel's encrypt() function
// The key should be the raw 32-byte key (not base64 encoded)
func DecryptLaravel(encryptedData string, key []byte) (string, error) {
	// Decode outer base64
	jsonData, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", ErrLaravelInvalidPayload
	}

	// Parse JSON payload
	var payload LaravelPayload
	if err := json.Unmarshal(jsonData, &payload); err != nil {
		return "", ErrLaravelInvalidPayload
	}

	// Validate MAC
	if !validateLaravelMAC(&payload, key) {
		return "", ErrLaravelInvalidMAC
	}

	// Decode IV and value
	iv, err := base64.StdEncoding.DecodeString(payload.IV)
	if err != nil {
		return "", ErrLaravelInvalidPayload
	}

	value, err := base64.StdEncoding.DecodeString(payload.Value)
	if err != nil {
		return "", ErrLaravelInvalidPayload
	}

	// Decrypt using AES-256-CBC
	plaintext, err := aesDecryptCBC(value, key, iv)
	if err != nil {
		return "", err
	}

	// Laravel serializes strings with PHP serialize format: s:N:"string";
	// Try to extract the actual string
	result := extractPHPString(plaintext)

	return result, nil
}

// DecryptLaravelWithBase64Key decrypts using a base64-encoded key (Laravel APP_KEY format)
func DecryptLaravelWithBase64Key(encryptedData, keyStr string) (string, error) {
	// Strip "base64:" prefix if present
	if strings.HasPrefix(keyStr, "base64:") {
		keyStr = keyStr[7:]
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return "", ErrInvalidKey
	}

	return DecryptLaravel(encryptedData, key)
}

func validateLaravelMAC(payload *LaravelPayload, key []byte) bool {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(payload.IV))
	h.Write([]byte(payload.Value))
	computedMAC := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(computedMAC), []byte(payload.MAC))
}

func aesDecryptCBC(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(iv) != aes.BlockSize {
		return nil, ErrLaravelInvalidPayload
	}

	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrLaravelInvalidPayload
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	plaintext, err = removePKCS7Padding(plaintext)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func removePKCS7Padding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	padding := int(data[len(data)-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}

// extractPHPString extracts a string from PHP serialized format
// PHP serializes strings as: s:N:"string"; where N is the length
func extractPHPString(data []byte) string {
	str := string(data)

	// Check if it's PHP serialized string format: s:N:"...";
	if len(str) > 4 && str[0] == 's' && str[1] == ':' {
		// Find the first quote
		firstQuote := strings.Index(str, "\"")
		if firstQuote == -1 {
			return str
		}

		// Find the last quote (before the semicolon)
		lastQuote := strings.LastIndex(str, "\"")
		if lastQuote == -1 || lastQuote <= firstQuote {
			return str
		}

		return str[firstQuote+1 : lastQuote]
	}

	return str
}

// MigrateLaravelToEncrypted decrypts Laravel-encrypted data and re-encrypts with our format
// Useful for migrating existing encrypted data
func MigrateLaravelToEncrypted(laravelEncrypted, laravelKey string) (string, error) {
	// Decrypt using Laravel format
	plaintext, err := DecryptLaravelWithBase64Key(laravelEncrypted, laravelKey)
	if err != nil {
		return "", err
	}

	// Re-encrypt with our AES-GCM format
	return Encrypt(plaintext)
}
