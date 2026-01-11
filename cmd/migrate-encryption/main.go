package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Laravel encryption structures
type LaravelPayload struct {
	IV    string `json:"iv"`
	Value string `json:"value"`
	MAC   string `json:"mac"`
}

var encryptionKey []byte

func main() {
	// Load config
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	// Get encryption key
	keyStr := viper.GetString("APP_KEY")
	if keyStr == "" {
		log.Fatal("APP_KEY not set")
	}

	// Parse Laravel key format
	if strings.HasPrefix(keyStr, "base64:") {
		keyStr = keyStr[7:]
	}
	var err error
	encryptionKey, err = base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		log.Fatalf("Failed to decode APP_KEY: %v", err)
	}

	// Initialize Go encryption with the same key
	if err := serializers.SetEncryptionKey(encryptionKey); err != nil {
		log.Fatalf("Failed to set Go encryption key: %v", err)
	}

	// Connect to database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		viper.GetString("DB_USERNAME"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetString("DB_PORT"),
		viper.GetString("DB_DATABASE"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Migrate each table
	migrateServers(db)
	migrateServerProviders(db)
	migrateDomainProviders(db)

	log.Println("✅ Migration completed successfully!")
	log.Println("You can now remove internal/database/serializers/laravel.go")
}

func migrateServers(db *gorm.DB) {
	log.Println("🔄 Migrating servers table...")

	type Server struct {
		ID               string  `gorm:"primaryKey"`
		PublicKey        *string `gorm:"column:public_key"`
		PrivateKey       *string `gorm:"column:private_key"`
		UserPublicKey    *string `gorm:"column:user_public_key"`
		Password         *string `gorm:"column:password"`
		DatabasePassword *string `gorm:"column:database_password"`
	}

	var servers []Server
	if err := db.Table("servers").Find(&servers).Error; err != nil {
		log.Printf("❌ Failed to fetch servers: %v", err)
		return
	}

	migrated := 0
	for _, server := range servers {
		updated := false
		updates := make(map[string]interface{})

		if server.PublicKey != nil && *server.PublicKey != "" {
			if encrypted, err := migrateEncryption(*server.PublicKey); err == nil {
				updates["public_key"] = encrypted
				updated = true
			}
		}
		if server.PrivateKey != nil && *server.PrivateKey != "" {
			if encrypted, err := migrateEncryption(*server.PrivateKey); err == nil {
				updates["private_key"] = encrypted
				updated = true
			}
		}
		if server.UserPublicKey != nil && *server.UserPublicKey != "" {
			if encrypted, err := migrateEncryption(*server.UserPublicKey); err == nil {
				updates["user_public_key"] = encrypted
				updated = true
			}
		}
		if server.Password != nil && *server.Password != "" {
			if encrypted, err := migrateEncryption(*server.Password); err == nil {
				updates["password"] = encrypted
				updated = true
			}
		}
		if server.DatabasePassword != nil && *server.DatabasePassword != "" {
			if encrypted, err := migrateEncryption(*server.DatabasePassword); err == nil {
				updates["database_password"] = encrypted
				updated = true
			}
		}

		if updated {
			if err := db.Table("servers").Where("id = ?", server.ID).Updates(updates).Error; err != nil {
				log.Printf("❌ Failed to update server %s: %v", server.ID, err)
			} else {
				migrated++
			}
		}
	}

	log.Printf("✅ Migrated %d/%d servers", migrated, len(servers))
}

func migrateServerProviders(db *gorm.DB) {
	log.Println("🔄 Migrating server_providers table...")

	type ServerProvider struct {
		ID          string `gorm:"primaryKey"`
		Credentials string `gorm:"column:credentials"`
	}

	var providers []ServerProvider
	if err := db.Table("server_providers").Find(&providers).Error; err != nil {
		log.Printf("❌ Failed to fetch server_providers: %v", err)
		return
	}

	migrated := 0
	for _, provider := range providers {
		if provider.Credentials != "" {
			if encrypted, err := migrateEncryption(provider.Credentials); err == nil {
				if err := db.Table("server_providers").Where("id = ?", provider.ID).
					Update("credentials", encrypted).Error; err != nil {
					log.Printf("❌ Failed to update server_provider %s: %v", provider.ID, err)
				} else {
					migrated++
				}
			}
		}
	}

	log.Printf("✅ Migrated %d/%d server_providers", migrated, len(providers))
}

func migrateDomainProviders(db *gorm.DB) {
	log.Println("🔄 Migrating domain_providers table...")

	type DomainProvider struct {
		ID          string `gorm:"primaryKey"`
		Credentials string `gorm:"column:credentials"`
	}

	var providers []DomainProvider
	if err := db.Table("domain_providers").Find(&providers).Error; err != nil {
		log.Printf("❌ Failed to fetch domain_providers: %v", err)
		return
	}

	migrated := 0
	for _, provider := range providers {
		if provider.Credentials != "" {
			if encrypted, err := migrateEncryption(provider.Credentials); err == nil {
				if err := db.Table("domain_providers").Where("id = ?", provider.ID).
					Update("credentials", encrypted).Error; err != nil {
					log.Printf("❌ Failed to update domain_provider %s: %v", provider.ID, err)
				} else {
					migrated++
				}
			}
		}
	}

	log.Printf("✅ Migrated %d/%d domain_providers", migrated, len(providers))
}

// migrateEncryption decrypts Laravel-encrypted data and re-encrypts with Go format
func migrateEncryption(encrypted string) (string, error) {
	// First decrypt Laravel format
	decrypted, err := decryptLaravel(encrypted)
	if err != nil {
		return "", err
	}

	// If it was already plaintext, encrypt it
	if decrypted == encrypted {
		return serializers.Encrypt(decrypted)
	}

	// Re-encrypt with Go format
	return serializers.Encrypt(decrypted)
}

// decryptLaravel decrypts Laravel-encrypted data
func decryptLaravel(encrypted string) (string, error) {
	// Try to decode as Laravel format
	jsonData, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		// Not Laravel encrypted, return as-is
		return encrypted, nil
	}

	var payload LaravelPayload
	if err := json.Unmarshal(jsonData, &payload); err != nil {
		// Not Laravel format, return as-is
		return encrypted, nil
	}

	// Validate MAC
	if !validateMAC(&payload) {
		return "", errors.New("invalid MAC")
	}

	// Decode IV and value
	iv, err := base64.StdEncoding.DecodeString(payload.IV)
	if err != nil {
		return "", err
	}

	value, err := base64.StdEncoding.DecodeString(payload.Value)
	if err != nil {
		return "", err
	}

	// Decrypt AES-256-CBC
	plaintext, err := aesDecryptCBC(value, iv)
	if err != nil {
		return "", err
	}

	// Extract PHP string if serialized
	return extractPHPString(plaintext), nil
}

func validateMAC(payload *LaravelPayload) bool {
	h := hmac.New(sha256.New, encryptionKey)
	h.Write([]byte(payload.IV))
	h.Write([]byte(payload.Value))
	computedMAC := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(computedMAC), []byte(payload.MAC))
}

func aesDecryptCBC(ciphertext, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	if len(iv) != aes.BlockSize {
		return nil, errors.New("invalid IV size")
	}

	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("invalid ciphertext size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	return plaintext[:len(plaintext)-padding], nil
}

func extractPHPString(data []byte) string {
	str := string(data)

	// Check if it's PHP serialized string format: s:N:"...";
	if len(str) > 4 && str[0] == 's' && str[1] == ':' {
		firstQuote := strings.Index(str, "\"")
		if firstQuote == -1 {
			return str
		}
		lastQuote := strings.LastIndex(str, "\"")
		if lastQuote == -1 || lastQuote <= firstQuote {
			return str
		}
		return str[firstQuote+1 : lastQuote]
	}

	return str
}
