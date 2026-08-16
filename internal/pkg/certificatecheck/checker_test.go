package certificatecheck

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateCertificateStates(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	root, rootKey := makeCertificate(t, nil, nil, certificateTemplate("Launch Test Root", nil, now.Add(-time.Hour), now.Add(365*24*time.Hour), true))
	roots := x509.NewCertPool()
	roots.AddCert(root)

	tests := []struct {
		name       string
		host       string
		notBefore  time.Time
		notAfter   time.Time
		dnsNames   []string
		wantStatus Status
		wantValid  bool
	}{
		{
			name: "valid trusted certificate", host: "app.example.com",
			notBefore: now.Add(-time.Hour), notAfter: now.Add(30 * 24 * time.Hour),
			dnsNames: []string{"app.example.com"}, wantStatus: StatusValid, wantValid: true,
		},
		{
			name: "expired certificate", host: "app.example.com",
			notBefore: now.Add(-48 * time.Hour), notAfter: now.Add(-time.Hour),
			dnsNames: []string{"app.example.com"}, wantStatus: StatusExpired,
		},
		{
			name: "wrong hostname", host: "app.example.com",
			notBefore: now.Add(-time.Hour), notAfter: now.Add(30 * 24 * time.Hour),
			dnsNames: []string{"other.example.com"}, wantStatus: StatusInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaf, _ := makeCertificate(t, root, rootKey, certificateTemplate("leaf", tt.dnsNames, tt.notBefore, tt.notAfter, false))
			got := evaluate(tt.host, []*x509.Certificate{leaf, root}, roots, now)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantValid, got.Valid)
			assert.Equal(t, tt.host, got.Host)
			require.NotNil(t, got.ExpiresAt)
		})
	}
}

func TestEvaluateWithoutPeerCertificate(t *testing.T) {
	now := time.Now().UTC()
	got := evaluate("app.example.com", nil, x509.NewCertPool(), now)
	assert.Equal(t, StatusNotIssued, got.Status)
	assert.False(t, got.Valid)
}

func TestUniquePublicIPsRejectsInternalTargetsAndDeduplicates(t *testing.T) {
	got := uniquePublicIPs([]net.IPAddr{
		{IP: net.ParseIP("127.0.0.1")},
		{IP: net.ParseIP("10.0.0.1")},
		{IP: net.ParseIP("169.254.1.1")},
		{IP: net.ParseIP("8.8.8.8")},
		{IP: net.ParseIP("8.8.8.8")},
		{IP: net.ParseIP("2606:4700:4700::1111")},
	})
	require.Len(t, got, 2)
	assert.Equal(t, "8.8.8.8", got[0].String())
	assert.Equal(t, "2606:4700:4700::1111", got[1].String())
}

func certificateTemplate(commonName string, dnsNames []string, notBefore, notAfter time.Time, isCA bool) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(notBefore.UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		DNSNames:              dnsNames,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}

func makeCertificate(t *testing.T, parent *x509.Certificate, parentKey *rsa.PrivateKey, template *x509.Certificate) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	if parent == nil {
		parent = template
		parentKey = key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return certificate, key
}
