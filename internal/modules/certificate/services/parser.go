// internal/modules/certificate/services/parser.go
package services

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ParsedCertificate is the metadata the service layer extracts from a
// user-supplied cert PEM. Stored verbatim on stored_certificates so
// the picker can render "name · *.acme.io · expires Mar 12" without
// re-parsing every list call.
type ParsedCertificate struct {
	Domains           []string
	CommonName        string
	Issuer            string
	NotBefore         time.Time
	NotAfter          time.Time
	SerialNumber      string
	FingerprintSHA256 string // hex
}

// ParseCertificate decodes the first leaf cert from a PEM blob and
// extracts metadata. PEM blobs containing chain entries are accepted
// — we only inspect the first CERTIFICATE block.
func ParseCertificate(pemContent string) (*ParsedCertificate, error) {
	block, _ := pem.Decode([]byte(pemContent))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a valid PEM-encoded certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	domains := uniqueStrings(append([]string{cert.Subject.CommonName}, cert.DNSNames...))
	// Filter empty CN (some certs only have SANs)
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		if strings.TrimSpace(d) != "" {
			out = append(out, d)
		}
	}

	fingerprint := sha256.Sum256(cert.Raw)

	return &ParsedCertificate{
		Domains:           out,
		CommonName:        cert.Subject.CommonName,
		Issuer:            cert.Issuer.CommonName,
		NotBefore:         cert.NotBefore.UTC(),
		NotAfter:          cert.NotAfter.UTC(),
		SerialNumber:      cert.SerialNumber.String(),
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
	}, nil
}

// ValidateKeyMatchesCert returns nil if the private key matches the
// certificate's public key (i.e. tls.X509KeyPair would succeed).
func ValidateKeyMatchesCert(certPEM, keyPEM string) error {
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		return fmt.Errorf("private key does not match certificate: %w", err)
	}
	return nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
