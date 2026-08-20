package certificatecheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

type Status string
type Reason string

const (
	StatusValid       Status = "valid"
	StatusNotIssued   Status = "not_issued"
	StatusExpired     Status = "expired"
	StatusInvalid     Status = "invalid"
	StatusUnreachable Status = "unreachable"
)

const (
	ReasonValid            Reason = "valid"
	ReasonNoHostname       Reason = "no_hostname"
	ReasonDNSLookupFailed  Reason = "dns_lookup_failed"
	ReasonDNSNotPublic     Reason = "dns_not_public"
	ReasonNoCertificate    Reason = "no_certificate"
	ReasonNotActive        Reason = "not_active"
	ReasonExpired          Reason = "expired"
	ReasonHostnameMismatch Reason = "hostname_mismatch"
	ReasonUntrusted        Reason = "untrusted"
	ReasonHTTPSDisabled    Reason = "https_disabled"
	ReasonInternalCA       Reason = "internal_ca"
)

type Result struct {
	Host          string     `json:"host"`
	Status        Status     `json:"status"`
	Reason        Reason     `json:"reason"`
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
		base.Reason = ReasonNoHostname
		base.Message = "No hostname is configured."
		return Localize(ctx, base)
	}

	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	addresses, err := c.resolver.LookupIPAddr(checkCtx, host)
	if err != nil {
		base.Status = StatusNotIssued
		base.Reason = ReasonDNSLookupFailed
		base.Message = fmt.Sprintf("DNS lookup failed for %s: %v", host, err)
		return Localize(ctx, base)
	}

	publicIPs := uniquePublicIPs(addresses)
	if len(publicIPs) == 0 {
		base.Status = StatusUnreachable
		base.Reason = ReasonDNSNotPublic
		base.Message = "DNS does not resolve to a public IP address."
		return Localize(ctx, base)
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
			return Localize(ctx, result)
		}
		if firstCertificateResult == nil {
			candidate := result
			firstCertificateResult = &candidate
		}
	}

	if firstCertificateResult != nil {
		return Localize(ctx, *firstCertificateResult)
	}
	base.Status = StatusNotIssued
	base.Reason = ReasonNoCertificate
	base.ResolvedIP = publicIPs[0].String()
	if lastErr != nil {
		base.Message = fmt.Sprintf("No TLS certificate could be retrieved from %s: %v", host, lastErr)
	} else {
		base.Message = fmt.Sprintf("No TLS certificate could be retrieved from %s.", host)
	}
	return Localize(ctx, base)
}

// Localize returns a copy with only the customer-facing message translated.
// Status, reason, host, IPs, certificate metadata, and timestamps remain stable
// for API consumers. English responses are returned byte-for-byte so existing
// clients retain detailed resolver/TLS diagnostics. Japanese responses are
// rebuilt from the stable reason and structured fields, which avoids leaking
// raw upstream errors that are not suitable for translation or display.
func Localize(ctx context.Context, result Result) Result {
	locale := i18n.LocaleFromContext(ctx)
	if locale == i18n.LocaleEnglish {
		return result
	}

	switch result.Reason {
	case ReasonValid:
		result.Message = i18n.Translate(locale, "A valid certificate is being served for %s.", result.Host)
	case ReasonNoHostname:
		result.Message = i18n.Translate(locale, "No hostname is configured.")
	case ReasonDNSLookupFailed:
		result.Message = i18n.Translate(locale, "DNS lookup failed for %s.", result.Host)
	case ReasonDNSNotPublic:
		result.Message = i18n.Translate(locale, "DNS does not resolve to a public IP address.")
	case ReasonNoCertificate:
		if result.Message == "The TLS endpoint did not present a certificate." {
			result.Message = i18n.Translate(locale, result.Message)
		} else {
			result.Message = i18n.Translate(locale, "No TLS certificate could be retrieved from %s.", result.Host)
		}
	case ReasonNotActive:
		if result.NotBefore != nil {
			result.Message = i18n.Translate(locale, "The served certificate is not valid until %s.", result.NotBefore.UTC().Format(time.RFC3339))
		} else {
			result.Message = i18n.Translate(locale, "The served certificate is not valid yet.")
		}
	case ReasonExpired:
		if result.ExpiresAt != nil {
			result.Message = i18n.Translate(locale, "The served certificate expired on %s.", result.ExpiresAt.UTC().Format(time.RFC3339))
		} else {
			result.Message = i18n.Translate(locale, "The served certificate has expired.")
		}
	case ReasonHostnameMismatch:
		result.Message = i18n.Translate(locale, "The server is not presenting a certificate for this hostname.")
	case ReasonUntrusted:
		result.Message = i18n.Translate(locale, "The served certificate is not trusted.")
	case ReasonHTTPSDisabled:
		source := "HTTPS is disabled."
		switch result.Message {
		case "HTTPS is disabled for this domain.":
			source = result.Message
		case "Public HTTPS is disabled for this site.":
			source = result.Message
		}
		result.Message = i18n.Translate(locale, source)
	case ReasonInternalCA:
		result.Message = i18n.Translate(locale, "This site uses Caddy's internal CA, which is not publicly trusted.")
	default:
		result.Message = i18n.Translate(locale, "Certificate status could not be determined.")
	}

	return result
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
		result.Reason = ReasonNoCertificate
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
		result.Reason = ReasonNotActive
		result.Message = fmt.Sprintf("The served certificate is not valid until %s.", notBefore.Format(time.RFC3339))
		return result
	}
	if !now.Before(leaf.NotAfter) {
		result.Status = StatusExpired
		result.Reason = ReasonExpired
		result.Message = fmt.Sprintf("The served certificate expired on %s.", expiresAt.Format(time.RFC3339))
		return result
	}
	if err := leaf.VerifyHostname(host); err != nil {
		result.Status = StatusInvalid
		result.Reason = ReasonHostnameMismatch
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
		result.Reason = ReasonUntrusted
		result.Message = fmt.Sprintf("The served certificate is not trusted: %v", err)
		return result
	}

	result.Status = StatusValid
	result.Reason = ReasonValid
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
