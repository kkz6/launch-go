package certificatecheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"
)

type Status string

const (
	StatusValid       Status = "valid"
	StatusNotIssued   Status = "not_issued"
	StatusExpired     Status = "expired"
	StatusInvalid     Status = "invalid"
	StatusUnreachable Status = "unreachable"
)

type Result struct {
	Host          string     `json:"host"`
	Status        Status     `json:"status"`
	Valid         bool       `json:"valid"`
	Message       string     `json:"message"`
	Issuer        string     `json:"issuer,omitempty"`
	Subject       string     `json:"subject,omitempty"`
	SerialNumber  string     `json:"serial_number,omitempty"`
	ResolvedIP    string     `json:"resolved_ip,omitempty"`
	DNSNames      []string   `json:"dns_names,omitempty"`
	NotBefore     *time.Time `json:"not_before,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	DaysRemaining *int       `json:"days_remaining,omitempty"`
	CheckedAt     time.Time  `json:"checked_at"`
}

type Checker interface {
	Check(context.Context, string) Result
}

type resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type NetworkChecker struct {
	resolver resolver
	dialer   dialer
	roots    *x509.CertPool
	now      func() time.Time
	port     string
}

func New() *NetworkChecker {
	roots, _ := x509.SystemCertPool()
	return &NetworkChecker{
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{Timeout: 5 * time.Second},
		roots:    roots,
		now:      time.Now,
		port:     "443",
	}
}

func (c *NetworkChecker) Check(ctx context.Context, rawHost string) Result {
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rawHost), "."))
	now := c.now().UTC()
	base := Result{Host: host, CheckedAt: now}
	if host == "" {
		base.Status = StatusNotIssued
		base.Message = "No hostname is configured."
		return base
	}

	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	addresses, err := c.resolver.LookupIPAddr(checkCtx, host)
	if err != nil {
		base.Status = StatusNotIssued
		base.Message = fmt.Sprintf("DNS lookup failed for %s: %v", host, err)
		return base
	}

	publicIPs := uniquePublicIPs(addresses)
	if len(publicIPs) == 0 {
		base.Status = StatusUnreachable
		base.Message = "DNS does not resolve to a public IP address."
		return base
	}

	var firstCertificateResult *Result
	var lastErr error
	for _, ip := range publicIPs {
		result, err := c.checkIP(checkCtx, host, ip, now)
		if err != nil {
			lastErr = err
			continue
		}
		if result.Valid {
			return result
		}
		if firstCertificateResult == nil {
			copy := result
			firstCertificateResult = &copy
		}
	}

	if firstCertificateResult != nil {
		return *firstCertificateResult
	}
	base.Status = StatusNotIssued
	base.ResolvedIP = publicIPs[0].String()
	if lastErr != nil {
		base.Message = fmt.Sprintf("No TLS certificate could be retrieved from %s: %v", host, lastErr)
	} else {
		base.Message = fmt.Sprintf("No TLS certificate could be retrieved from %s.", host)
	}
	return base
}

func (c *NetworkChecker) checkIP(
	ctx context.Context,
	host string,
	ip net.IP,
	now time.Time,
) (Result, error) {
	address := net.JoinHostPort(ip.String(), c.port)
	rawConn, err := c.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return Result{}, err
	}
	defer rawConn.Close()

	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return Result{}, err
	}

	result := evaluate(host, tlsConn.ConnectionState().PeerCertificates, c.roots, now)
	result.ResolvedIP = ip.String()
	return result, nil
}

func evaluate(host string, certificates []*x509.Certificate, roots *x509.CertPool, now time.Time) Result {
	result := Result{Host: host, CheckedAt: now.UTC()}
	if len(certificates) == 0 {
		result.Status = StatusNotIssued
		result.Message = "The TLS endpoint did not present a certificate."
		return result
	}

	leaf := certificates[0]
	notBefore := leaf.NotBefore.UTC()
	expiresAt := leaf.NotAfter.UTC()
	remaining := int(expiresAt.Sub(now).Hours() / 24)
	result.Issuer = certificateName(leaf.Issuer.CommonName, leaf.Issuer.Organization, leaf.Issuer.String())
	result.Subject = certificateName(leaf.Subject.CommonName, leaf.Subject.Organization, leaf.Subject.String())
	result.SerialNumber = leaf.SerialNumber.String()
	result.DNSNames = append([]string(nil), leaf.DNSNames...)
	result.NotBefore = &notBefore
	result.ExpiresAt = &expiresAt
	result.DaysRemaining = &remaining

	if now.Before(leaf.NotBefore) {
		result.Status = StatusInvalid
		result.Message = fmt.Sprintf("The served certificate is not valid until %s.", notBefore.Format(time.RFC3339))
		return result
	}
	if !now.Before(leaf.NotAfter) {
		result.Status = StatusExpired
		result.Message = fmt.Sprintf("The served certificate expired on %s.", expiresAt.Format(time.RFC3339))
		return result
	}
	if err := leaf.VerifyHostname(host); err != nil {
		result.Status = StatusInvalid
		result.Message = "The server is not presenting a certificate for this hostname."
		return result
	}

	intermediates := x509.NewCertPool()
	for _, certificate := range certificates[1:] {
		intermediates.AddCert(certificate)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		DNSName:       host,
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   now,
	}); err != nil {
		result.Status = StatusInvalid
		result.Message = fmt.Sprintf("The served certificate is not trusted: %v", err)
		return result
	}

	result.Status = StatusValid
	result.Valid = true
	result.Message = fmt.Sprintf("A valid certificate is being served for %s.", host)
	return result
}

func certificateName(commonName string, organizations []string, fallback string) string {
	if strings.TrimSpace(commonName) != "" {
		return commonName
	}
	if len(organizations) > 0 && strings.TrimSpace(organizations[0]) != "" {
		return organizations[0]
	}
	return fallback
}

func uniquePublicIPs(addresses []net.IPAddr) []net.IP {
	seen := make(map[string]struct{}, len(addresses))
	out := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip := address.IP
		if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			continue
		}
		key := ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ip)
	}
	return out
}
