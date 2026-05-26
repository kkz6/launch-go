// cmd/test-do: standalone script to test DigitalOcean droplet creation end-to-end.
// Run: DO_TOKEN=xxx go run ./cmd/test-do
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const doAPI = "https://api.digitalocean.com/v2"

func main() {
	token := os.Getenv("DO_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Set DO_TOKEN env var")
		os.Exit(1)
	}

	region := getEnv("DO_REGION", "blr1")
	size := getEnv("DO_SIZE", "s-1vcpu-1gb")
	image := getEnv("DO_IMAGE", "ubuntu-24-04-x64")

	fmt.Printf("=== DigitalOcean Creation Test ===\n")
	fmt.Printf("Region: %s | Size: %s | Image: %s\n\n", region, size, image)

	ctx := context.Background()

	// Step 1: Verify account
	fmt.Println("[1] Verifying account...")
	acct, err := doGet(ctx, token, "/account")
	if err != nil {
		fatalf("GET /account failed: %v", err)
	}
	if a, ok := acct["account"].(map[string]interface{}); ok {
		fmt.Printf("    Email: %v | Status: %v | Droplet limit: %v\n",
			a["email"], a["status"], a["droplet_limit"])
	}

	// Step 2: Generate SSH key
	fmt.Println("\n[2] Generating SSH key pair...")
	pubKey, _, err := generateKeyPair()
	if err != nil {
		fatalf("Key generation failed: %v", err)
	}
	fmt.Printf("    Public key: %s...\n", pubKey[:40])

	// Step 3: Upload SSH key to DO
	fmt.Println("\n[3] Uploading SSH key to DigitalOcean...")
	keyName := fmt.Sprintf("test-do-%d", time.Now().Unix())
	keyResp, err := doPost(ctx, token, "/account/keys", map[string]interface{}{
		"name":       keyName,
		"public_key": pubKey,
	})
	if err != nil {
		fatalf("POST /account/keys failed: %v", err)
	}
	sshKeyData, _ := keyResp["ssh_key"].(map[string]interface{})
	if sshKeyData == nil {
		fatalf("Unexpected /account/keys response: %s", jsonPretty(keyResp))
	}

	keyIDFloat, _ := sshKeyData["id"].(float64)
	keyID := int64(keyIDFloat)
	fingerprint, _ := sshKeyData["fingerprint"].(string)
	fmt.Printf("    Key ID (int64): %d\n", keyID)
	fmt.Printf("    Fingerprint:    %s\n", fingerprint)

	// Step 4: Create droplet
	fmt.Println("\n[4] Creating droplet...")
	dropletBody := map[string]interface{}{
		"name":       keyName,
		"region":     region,
		"size":       size,
		"image":      image,
		"backups":    false,
		"ipv6":       false,
		"monitoring": false,
		"ssh_keys":   []interface{}{keyID},
	}
	fmt.Printf("    Request body:\n%s\n", jsonPretty(dropletBody))

	dropletResp, err := doPost(ctx, token, "/droplets", dropletBody)
	if err != nil {
		fmt.Printf("\n    FAILED: %v\n", err)
		// Cleanup the SSH key we created
		cleanupKey(ctx, token, strconv.FormatInt(keyID, 10))
		os.Exit(1)
	}

	droplet, _ := dropletResp["droplet"].(map[string]interface{})
	if droplet == nil {
		fmt.Printf("\n    Unexpected response:\n%s\n", jsonPretty(dropletResp))
		cleanupKey(ctx, token, strconv.FormatInt(keyID, 10))
		os.Exit(1)
	}

	dropletIDFloat, _ := droplet["id"].(float64)
	fmt.Printf("\n    SUCCESS! Droplet ID: %d | Status: %v\n",
		int64(dropletIDFloat), droplet["status"])

	// Cleanup
	fmt.Println("\n[5] Cleaning up (deleting test droplet + SSH key)...")
	dropletID := strconv.FormatInt(int64(dropletIDFloat), 10)
	doDelete(ctx, token, "/droplets/"+dropletID)
	cleanupKey(ctx, token, strconv.FormatInt(keyID, 10))
	fmt.Println("    Done.")
}

// HTTP helpers

func doGet(ctx context.Context, token, path string) (map[string]interface{}, error) {
	return doRequest(ctx, token, http.MethodGet, path, nil)
}

func doPost(ctx context.Context, token, path string, body map[string]interface{}) (map[string]interface{}, error) {
	return doRequest(ctx, token, http.MethodPost, path, body)
}

func doDelete(ctx context.Context, token, path string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, doAPI+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	http.DefaultClient.Do(req) //nolint
}

func cleanupKey(ctx context.Context, token, keyID string) {
	doDelete(ctx, token, "/account/keys/"+keyID)
	fmt.Printf("    Deleted SSH key %s\n", keyID)
}

func doRequest(ctx context.Context, token, method, path string, body map[string]interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, doAPI+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d %s\n    Body: %s", resp.StatusCode, resp.Status, prettyJSON(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	return result, nil
}

// generateKeyPair generates an RSA key pair and returns the OpenSSH public key and PEM private key.
func generateKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))

	return pubKey, string(privPEM), nil
}

func jsonPretty(v interface{}) string {
	b, _ := json.MarshalIndent(v, "    ", "  ")
	return string(b)
}

func prettyJSON(b []byte) string {
	var v interface{}
	if json.Unmarshal(b, &v) == nil {
		out, _ := json.MarshalIndent(v, "    ", "  ")
		return string(out)
	}
	return string(b)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
