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
	"flag"
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
var debugMode bool

func main() {
	// Parse command line flags
	fix := flag.Bool("fix", false, "Fix double-encrypted data")
	migrate := flag.Bool("migrate", false, "Migrate Laravel encryption to Go format")
	debug := flag.Bool("debug", false, "Enable debug output")
	check := flag.Bool("check", false, "Check encryption status without making changes")

	// Table selection flags
	servers := flag.Bool("servers", false, "Process servers table")
	serverProviders := flag.Bool("server-providers", false, "Process server_providers table")
	domainProviders := flag.Bool("domain-providers", false, "Process domain_providers table")
	tasks := flag.Bool("tasks", false, "Process tasks table")
	crons := flag.Bool("crons", false, "Process crons table")
	all := flag.Bool("all", false, "Process all tables")

	flag.Parse()

	debugMode = *debug

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
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Determine which tables to process
	processAll := *all || (!*servers && !*serverProviders && !*domainProviders && !*tasks && !*crons)
	processServers := *servers || processAll
	processServerProviders := *serverProviders || processAll
	processDomainProviders := *domainProviders || processAll
	processTasks := *tasks || processAll
	processCrons := *crons || processAll

	// Check mode - just show status
	if *check {
		log.Println("🔍 Checking encryption status...")
		if processServers {
			checkServers(db)
		}
		if processServerProviders {
			checkServerProviders(db)
		}
		if processDomainProviders {
			checkDomainProviders(db)
		}
		if processTasks {
			checkTasks(db)
		}
		if processCrons {
			checkCrons(db)
		}
		return
	}

	// Fix mode
	if *fix {
		log.Println("🔧 Fixing double-encrypted data...")
		if processServers {
			fixDoubleEncryptedServers(db)
		}
		if processServerProviders {
			fixDoubleEncryptedServerProviders(db)
		}
		if processDomainProviders {
			fixDoubleEncryptedDomainProviders(db)
		}
		if processCrons {
			fixDoubleEncryptedCrons(db)
		}
		if processTasks {
			fixDoubleEncryptedTasks(db)
		}
		log.Println("✅ Fix completed!")
		return
	}

	// Migrate mode
	if *migrate {
		log.Println("🔄 Migrating encryption...")
		if processServers {
			migrateServers(db)
		}
		if processServerProviders {
			migrateServerProviders(db)
		}
		if processDomainProviders {
			migrateDomainProviders(db)
		}
		if processTasks {
			migrateTasks(db)
		}
		if processCrons {
			migrateCrons(db)
		}
		log.Println("✅ Migration completed!")
		return
	}

	// Show usage if no action specified
	fmt.Println("Usage: migrate-encryption [action] [options]")
	fmt.Println("")
	fmt.Println("Actions:")
	fmt.Println("  -migrate    Migrate Laravel encryption to Go format")
	fmt.Println("  -fix        Fix double-encrypted data")
	fmt.Println("  -check      Check encryption status without changes")
	fmt.Println("")
	fmt.Println("Table selection (default: all):")
	fmt.Println("  -all              Process all tables")
	fmt.Println("  -servers          Process servers table")
	fmt.Println("  -server-providers Process server_providers table")
	fmt.Println("  -domain-providers Process domain_providers table")
	fmt.Println("  -tasks            Process tasks table")
	fmt.Println("  -crons            Process crons table")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -debug      Enable debug output")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  migrate-encryption -check -servers")
	fmt.Println("  migrate-encryption -fix -servers -debug")
	fmt.Println("  migrate-encryption -migrate -all")
}

// checkServers checks the encryption status of servers
func checkServers(db *gorm.DB) {
	log.Println("🔍 Checking servers table...")

	type Server struct {
		ID               string  `gorm:"primaryKey"`
		Name             string  `gorm:"column:name"`
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

	for _, server := range servers {
		log.Printf("\n  Server: %s (%s)", server.ID, server.Name)

		checkField("private_key", server.PrivateKey)
		checkField("public_key", server.PublicKey)
		checkField("user_public_key", server.UserPublicKey)
		checkField("password", server.Password)
		checkField("database_password", server.DatabasePassword)
	}
}

// checkField checks a single encrypted field and reports its status
func checkField(name string, value *string) {
	if value == nil || *value == "" {
		log.Printf("    %s: (empty)", name)
		return
	}

	raw := *value

	// Try Go decryption
	decrypted1, err1 := serializers.Decrypt(raw)
	if err1 != nil {
		// Check if it's Laravel format
		if isLaravelEncrypted(raw) {
			log.Printf("    %s: ⚠️  Laravel format (needs migration)", name)
		} else {
			log.Printf("    %s: ❌ Cannot decrypt (error: %v)", name, err1)
		}
		return
	}

	// Check if decrypted value looks like plaintext
	if isValidPlaintext(decrypted1) {
		// Try to decrypt again to check for double-encryption
		decrypted2, err2 := serializers.Decrypt(decrypted1)
		if err2 == nil && decrypted2 != decrypted1 && isValidPlaintext(decrypted2) {
			log.Printf("    %s: ⚠️  Double-encrypted! (needs fix)", name)
			if debugMode {
				log.Printf("      Layer 1 (%d chars): %s...", len(decrypted1), truncate(decrypted1, 50))
				log.Printf("      Layer 2 (%d chars): %s...", len(decrypted2), truncate(decrypted2, 50))
			}
		} else {
			log.Printf("    %s: ✅ OK (Go format)", name)
			if debugMode {
				log.Printf("      Decrypted (%d chars): %s...", len(decrypted1), truncate(decrypted1, 50))
			}
		}
	} else {
		// Decrypted but doesn't look like plaintext - might be multi-layer
		log.Printf("    %s: ⚠️  Decrypted but unclear format", name)
		if debugMode {
			log.Printf("      Raw length: %d", len(raw))
			log.Printf("      Decrypted length: %d", len(decrypted1))
			log.Printf("      First 100 chars: %s", truncate(decrypted1, 100))
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func isLaravelEncrypted(data string) bool {
	jsonData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return false
	}
	var payload LaravelPayload
	return json.Unmarshal(jsonData, &payload) == nil && payload.IV != "" && payload.Value != ""
}

// checkServerProviders checks the encryption status of server_providers
func checkServerProviders(db *gorm.DB) {
	log.Println("🔍 Checking server_providers table...")

	type ServerProvider struct {
		ID          string `gorm:"primaryKey"`
		Name        string `gorm:"column:name"`
		Credentials string `gorm:"column:credentials"`
	}

	var providers []ServerProvider
	if err := db.Table("server_providers").Find(&providers).Error; err != nil {
		log.Printf("❌ Failed to fetch server_providers: %v", err)
		return
	}

	for _, provider := range providers {
		log.Printf("\n  Provider: %s (%s)", provider.ID, provider.Name)
		creds := provider.Credentials
		checkField("credentials", &creds)
	}
}

// checkDomainProviders checks the encryption status of domain_providers
func checkDomainProviders(db *gorm.DB) {
	log.Println("🔍 Checking domain_providers table...")

	type DomainProvider struct {
		ID          string `gorm:"primaryKey"`
		Name        string `gorm:"column:name"`
		Credentials string `gorm:"column:credentials"`
	}

	var providers []DomainProvider
	if err := db.Table("domain_providers").Find(&providers).Error; err != nil {
		log.Printf("❌ Failed to fetch domain_providers: %v", err)
		return
	}

	for _, provider := range providers {
		log.Printf("\n  Provider: %s (%s)", provider.ID, provider.Name)
		creds := provider.Credentials
		checkField("credentials", &creds)
	}
}

// checkTasks checks the encryption status of tasks
func checkTasks(db *gorm.DB) {
	log.Println("🔍 Checking tasks table (sample of 5)...")

	type Task struct {
		ID     string  `gorm:"primaryKey"`
		Output *string `gorm:"column:output"`
	}

	var tasks []Task
	if err := db.Table("tasks").Where("output IS NOT NULL AND output != ''").Limit(5).Find(&tasks).Error; err != nil {
		log.Printf("❌ Failed to fetch tasks: %v", err)
		return
	}

	for _, task := range tasks {
		log.Printf("\n  Task: %s", task.ID)
		checkField("output", task.Output)
	}
}

// checkCrons checks the encryption status of crons
func checkCrons(db *gorm.DB) {
	log.Println("🔍 Checking crons table...")

	type Cron struct {
		ID      string `gorm:"primaryKey"`
		Command string `gorm:"column:command"`
	}

	var crons []Cron
	if err := db.Table("crons").Find(&crons).Error; err != nil {
		log.Printf("❌ Failed to fetch crons: %v", err)
		return
	}

	for _, cron := range crons {
		log.Printf("\n  Cron: %s", cron.ID)
		cmd := cron.Command
		checkField("command", &cmd)
	}
}

// fixDoubleEncryptedServers fixes servers that were accidentally double-encrypted
func fixDoubleEncryptedServers(db *gorm.DB) {
	log.Println("🔄 Checking servers table for double-encrypted data...")

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

	fixed := 0
	for _, server := range servers {
		updates := make(map[string]interface{})
		updated := false

		if server.PrivateKey != nil && *server.PrivateKey != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(*server.PrivateKey); ok {
				updates["private_key"] = fixedValue
				updated = true
				log.Printf("  Fixed private_key for server %s", server.ID)
			}
		}
		if server.PublicKey != nil && *server.PublicKey != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(*server.PublicKey); ok {
				updates["public_key"] = fixedValue
				updated = true
			}
		}
		if server.UserPublicKey != nil && *server.UserPublicKey != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(*server.UserPublicKey); ok {
				updates["user_public_key"] = fixedValue
				updated = true
			}
		}
		if server.Password != nil && *server.Password != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(*server.Password); ok {
				updates["password"] = fixedValue
				updated = true
			}
		}
		if server.DatabasePassword != nil && *server.DatabasePassword != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(*server.DatabasePassword); ok {
				updates["database_password"] = fixedValue
				updated = true
			}
		}

		if updated {
			if err := db.Table("servers").Where("id = ?", server.ID).Updates(updates).Error; err != nil {
				log.Printf("❌ Failed to fix server %s: %v", server.ID, err)
			} else {
				fixed++
			}
		}
	}

	log.Printf("✅ Fixed %d/%d servers", fixed, len(servers))
}

// fixDoubleEncryptedServerProviders fixes server_providers that were accidentally double-encrypted
func fixDoubleEncryptedServerProviders(db *gorm.DB) {
	log.Println("🔄 Checking server_providers table for double-encrypted data...")

	type ServerProvider struct {
		ID          string `gorm:"primaryKey"`
		Credentials string `gorm:"column:credentials"`
	}

	var providers []ServerProvider
	if err := db.Table("server_providers").Find(&providers).Error; err != nil {
		log.Printf("❌ Failed to fetch server_providers: %v", err)
		return
	}

	fixed := 0
	for _, provider := range providers {
		if provider.Credentials != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(provider.Credentials); ok {
				if err := db.Table("server_providers").Where("id = ?", provider.ID).
					Update("credentials", fixedValue).Error; err != nil {
					log.Printf("❌ Failed to fix server_provider %s: %v", provider.ID, err)
				} else {
					fixed++
					log.Printf("  Fixed credentials for server_provider %s", provider.ID)
				}
			}
		}
	}

	log.Printf("✅ Fixed %d/%d server_providers", fixed, len(providers))
}

// fixDoubleEncryptedDomainProviders fixes domain_providers that were accidentally double-encrypted
func fixDoubleEncryptedDomainProviders(db *gorm.DB) {
	log.Println("🔄 Checking domain_providers table for double-encrypted data...")

	type DomainProvider struct {
		ID          string `gorm:"primaryKey"`
		Credentials string `gorm:"column:credentials"`
	}

	var providers []DomainProvider
	if err := db.Table("domain_providers").Find(&providers).Error; err != nil {
		log.Printf("❌ Failed to fetch domain_providers: %v", err)
		return
	}

	fixed := 0
	for _, provider := range providers {
		if provider.Credentials != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(provider.Credentials); ok {
				if err := db.Table("domain_providers").Where("id = ?", provider.ID).
					Update("credentials", fixedValue).Error; err != nil {
					log.Printf("❌ Failed to fix domain_provider %s: %v", provider.ID, err)
				} else {
					fixed++
					log.Printf("  Fixed credentials for domain_provider %s", provider.ID)
				}
			}
		}
	}

	log.Printf("✅ Fixed %d/%d domain_providers", fixed, len(providers))
}

// isValidPlaintext checks if data looks like valid plaintext (not encrypted)
func isValidPlaintext(data string) bool {
	// SSH keys
	if strings.HasPrefix(data, "-----BEGIN") {
		return true
	}
	// SSH public keys
	if strings.HasPrefix(data, "ssh-") {
		return true
	}
	// JSON
	if strings.HasPrefix(data, "{") || strings.HasPrefix(data, "[") {
		return true
	}
	// Short strings - passwords, tokens (typically <= 64 chars)
	// Even if they look like base64, short alphanumeric strings are likely passwords
	if len(data) <= 64 {
		return true
	}
	// Task outputs - contains newlines, common shell output patterns
	if strings.Contains(data, "\n") {
		return true
	}
	// Common command output patterns
	if strings.Contains(data, "exit") || strings.Contains(data, "error") ||
		strings.Contains(data, "success") || strings.Contains(data, "failed") ||
		strings.Contains(data, "/") || strings.Contains(data, ":") {
		return true
	}
	// Cron commands - typically contain paths or common commands
	if strings.HasPrefix(data, "/") || strings.HasPrefix(data, "php ") ||
		strings.HasPrefix(data, "cd ") || strings.Contains(data, "artisan") {
		return true
	}
	return false
}

// looksLikeBase64 checks if string looks like base64 encoded data
func looksLikeBase64(s string) bool {
	// Base64 strings are usually multiples of 4 and contain only base64 chars
	if len(s) < 20 {
		return false
	}
	// Check if it's valid base64
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// fixDoubleEncryptedValue attempts to fix multi-layer encrypted data
// Returns the fixed value and true if it was multi-encrypted, or empty string and false otherwise
func fixDoubleEncryptedValue(encrypted string) (string, bool) {
	// Recursively decrypt until we hit plaintext or can't decrypt anymore
	current := encrypted
	decryptCount := 0
	maxDecrypts := 10 // Safety limit

	for decryptCount < maxDecrypts {
		decrypted, err := serializers.Decrypt(current)
		if err != nil || decrypted == current {
			// Can't decrypt further
			break
		}

		decryptCount++
		current = decrypted

		// Check if this looks like valid plaintext
		if isValidPlaintext(current) {
			break
		}

		// Also check for Laravel format
		if jsonData, err := base64.StdEncoding.DecodeString(current); err == nil {
			var payload LaravelPayload
			if json.Unmarshal(jsonData, &payload) == nil {
				// It's Laravel encrypted - decrypt it
				if laravelDecrypted, err := decryptLaravel(current); err == nil {
					current = laravelDecrypted
					decryptCount++
					if isValidPlaintext(current) {
						break
					}
				}
			}
		}
	}

	// If we only decrypted once, it's not multi-encrypted
	if decryptCount <= 1 {
		return "", false
	}

	// Re-encrypt with single layer
	reEncrypted, err := serializers.Encrypt(current)
	if err != nil {
		log.Printf("  Warning: Re-encryption failed: %v", err)
		return "", false
	}

	log.Printf("  Found %d-layer encrypted data", decryptCount)
	return reEncrypted, true
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

func migrateTasks(db *gorm.DB) {
	log.Println("🔄 Migrating tasks table...")

	type Task struct {
		ID     string  `gorm:"primaryKey"`
		Output *string `gorm:"column:output"`
	}

	var tasks []Task
	if err := db.Table("tasks").Find(&tasks).Error; err != nil {
		log.Printf("❌ Failed to fetch tasks: %v", err)
		return
	}

	migrated := 0
	for _, task := range tasks {
		if task.Output != nil && *task.Output != "" {
			if encrypted, err := migrateEncryption(*task.Output); err == nil {
				if err := db.Table("tasks").Where("id = ?", task.ID).
					Update("output", encrypted).Error; err != nil {
					log.Printf("❌ Failed to update task %s: %v", task.ID, err)
				} else {
					migrated++
				}
			}
		}
	}

	log.Printf("✅ Migrated %d/%d tasks", migrated, len(tasks))
}

// migrateEncryption decrypts Laravel-encrypted data and re-encrypts with Go format
func migrateEncryption(encrypted string) (string, error) {
	// First, check if it's already in Go format by trying to decrypt
	if isGoEncrypted(encrypted) {
		// Already migrated, skip
		return "", fmt.Errorf("already in Go format")
	}

	// Try to decrypt Laravel format
	decrypted, err := decryptLaravel(encrypted)
	if err != nil {
		return "", err
	}

	// If it wasn't Laravel format (decrypted == encrypted), it might be plaintext
	// Only encrypt if it looks like valid data (not empty)
	if decrypted == encrypted {
		// Not Laravel format - could be plaintext or unknown format
		// Only encrypt if it looks like actual content
		if len(decrypted) > 0 {
			return serializers.Encrypt(decrypted)
		}
		return "", fmt.Errorf("empty data")
	}

	// Re-encrypt with Go format
	return serializers.Encrypt(decrypted)
}

// isGoEncrypted checks if data is already encrypted with Go AES-GCM format
func isGoEncrypted(data string) bool {
	// Try to decrypt with Go format
	decrypted, err := serializers.Decrypt(data)
	if err != nil {
		return false
	}
	// If decryption succeeded and result is different, it was Go-encrypted
	// Also check if decrypted looks like valid content (SSH key, JSON, etc.)
	if decrypted != data && len(decrypted) > 0 {
		// Additional check: valid SSH keys start with "-----BEGIN"
		// Valid JSON starts with "{" or "["
		if strings.HasPrefix(decrypted, "-----BEGIN") ||
			strings.HasPrefix(decrypted, "{") ||
			strings.HasPrefix(decrypted, "[") ||
			len(decrypted) > 100 { // Reasonable content
			return true
		}
	}
	return false
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

func migrateCrons(db *gorm.DB) {
	log.Println("🔄 Migrating crons table...")

	type Cron struct {
		ID      string `gorm:"primaryKey"`
		Command string `gorm:"column:command"`
	}

	var crons []Cron
	if err := db.Table("crons").Find(&crons).Error; err != nil {
		log.Printf("❌ Failed to fetch crons: %v", err)
		return
	}

	migrated := 0
	for _, cron := range crons {
		if cron.Command != "" {
			if encrypted, err := migrateEncryption(cron.Command); err == nil {
				if err := db.Table("crons").Where("id = ?", cron.ID).
					Update("command", encrypted).Error; err != nil {
					log.Printf("❌ Failed to update cron %s: %v", cron.ID, err)
				} else {
					migrated++
				}
			}
		}
	}

	log.Printf("✅ Migrated %d/%d crons", migrated, len(crons))
}

// fixDoubleEncryptedTasks fixes tasks that were accidentally double-encrypted
func fixDoubleEncryptedTasks(db *gorm.DB) {
	log.Println("🔄 Checking tasks table for double-encrypted data...")

	type Task struct {
		ID     string  `gorm:"primaryKey"`
		Output *string `gorm:"column:output"`
	}

	var tasks []Task
	if err := db.Table("tasks").Find(&tasks).Error; err != nil {
		log.Printf("❌ Failed to fetch tasks: %v", err)
		return
	}

	fixed := 0
	for _, task := range tasks {
		if task.Output != nil && *task.Output != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(*task.Output); ok {
				if err := db.Table("tasks").Where("id = ?", task.ID).
					Update("output", fixedValue).Error; err != nil {
					log.Printf("❌ Failed to fix task %s: %v", task.ID, err)
				} else {
					fixed++
				}
			}
		}
	}

	log.Printf("✅ Fixed %d/%d tasks", fixed, len(tasks))
}

// fixDoubleEncryptedCrons fixes crons that were accidentally double-encrypted
func fixDoubleEncryptedCrons(db *gorm.DB) {
	log.Println("🔄 Checking crons table for double-encrypted data...")

	type Cron struct {
		ID      string `gorm:"primaryKey"`
		Command string `gorm:"column:command"`
	}

	var crons []Cron
	if err := db.Table("crons").Find(&crons).Error; err != nil {
		log.Printf("❌ Failed to fetch crons: %v", err)
		return
	}

	fixed := 0
	for _, cron := range crons {
		if cron.Command != "" {
			if fixedValue, ok := fixDoubleEncryptedValue(cron.Command); ok {
				if err := db.Table("crons").Where("id = ?", cron.ID).
					Update("command", fixedValue).Error; err != nil {
					log.Printf("❌ Failed to fix cron %s: %v", cron.ID, err)
				} else {
					fixed++
					log.Printf("  Fixed command for cron %s", cron.ID)
				}
			}
		}
	}

	log.Printf("✅ Fixed %d/%d crons", fixed, len(crons))
}
