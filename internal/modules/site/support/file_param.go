package support

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/kkz6/launch-go/internal/database/serializers"
)

// FileRouteData represents the data encoded in a file route parameter
type FileRouteData struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// EncodeFileRouteParam encodes file path and type into an encrypted, compressed, URL-safe parameter
// This matches Laravel's routeParameter() method in FileOnServer
func EncodeFileRouteParam(path, fileType string) (string, error) {
	// 1. Create JSON data
	data := FileRouteData{
		Path: path,
		Type: fileType,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal file data: %w", err)
	}

	// 2. Gzip compress
	var compressed bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := gzipWriter.Write(jsonBytes); err != nil {
		gzipWriter.Close()
		return "", fmt.Errorf("failed to write gzip data: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// 3. Encrypt
	encrypted, err := serializers.Encrypt(compressed.String())
	if err != nil {
		return "", fmt.Errorf("failed to encrypt file data: %w", err)
	}

	// 4. Make URL-safe using base64 URL encoding
	urlSafe := base64.URLEncoding.EncodeToString([]byte(encrypted))

	return urlSafe, nil
}

// DecodeFileRouteParam decodes an encrypted file route parameter back to path and type
// This matches Laravel's dataFromRouteParameter() method in FileOnServer
func DecodeFileRouteParam(param string) (*FileRouteData, error) {
	// 1. Decode base64 - try URL-safe first, then standard
	encrypted, err := base64.URLEncoding.DecodeString(param)
	if err != nil {
		// Try standard base64 encoding (Laravel might use this)
		encrypted, err = base64.StdEncoding.DecodeString(param)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64: %w", err)
		}
	}

	// 2. Decrypt
	decrypted, err := serializers.Decrypt(string(encrypted))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt file data: %w", err)
	}

	// 3. Decompress gzip
	gzipReader, err := gzip.NewReader(bytes.NewReader([]byte(decrypted)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	// 4. Parse JSON
	var data FileRouteData
	if err := json.Unmarshal(decompressed, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file data: %w", err)
	}

	return &data, nil
}

// PathFromRouteParam extracts just the path from an encoded route parameter
func PathFromRouteParam(param string) (string, error) {
	data, err := DecodeFileRouteParam(param)
	if err != nil {
		return "", err
	}
	return data.Path, nil
}
