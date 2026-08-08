package services

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
)

// fakeDNSLookuper answers LookupIPAddr with canned addresses or a canned
// error, so ValidateDNS's branches can be driven without hitting real DNS.
type fakeDNSLookuper struct {
	addrs []string
	err   error
}

func (f *fakeDNSLookuper) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]net.IPAddr, 0, len(f.addrs))
	for _, addr := range f.addrs {
		out = append(out, net.IPAddr{IP: net.ParseIP(addr)})
	}
	return out, nil
}

// dnsValidateTestService builds a real DomainService backed by an
// in-memory server repo — the only dependency validateDNSAgainstServer
// actually reads — with everything else left zero-valued. Every other
// field on Dependencies is nil-tolerant by design (see the "silently
// succeed in test mode" comment on Base.EnqueueTask), and this code path
// never touches the docker repos, queue, broadcaster, or logger.
func dnsValidateTestService(t *testing.T, lookuper dnsLookuper) (*DomainService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// ServerRepository's base wraps every FindByID in a preload of the
	// Services relation (see NewServerRepository's "Services" argument), so
	// that table has to exist even with nothing in it.
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &servermodels.InstalledService{}))

	svc := NewDomainService(&ServiceDeps{ServerRepos: serverrepos.NewRegistry(db)})
	svc.SetDNSLookuper(lookuper)
	return svc, db
}

func createTestServer(t *testing.T, db *gorm.DB, publicIP string) string {
	t.Helper()
	server := &servermodels.Server{Name: "test-server"}
	server.ID = "srv-dns-test"
	server.TeamID = "team-dns-test"
	if publicIP != "" {
		server.PublicIPv4 = &publicIP
	}
	require.NoError(t, db.Create(server).Error)
	return server.ID
}

// =============================================================================
// wildcard short-circuit — runs before the server lookup, so a bogus
// serverID still succeeds.
// =============================================================================

func TestValidateDNSAgainstServerWildcardSuffix(t *testing.T) {
	svc, _ := dnsValidateTestService(t, &fakeDNSLookuper{err: errors.New("must not be called")})

	// traefik.me is deliberately NOT a wildcard suffix (see the comment on
	// wildcardDNSSuffixes) — sslip.io is one of the three that are.
	resp, err := svc.validateDNSAgainstServer(context.Background(), "app.sslip.io", "does-not-exist")
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.True(t, resp.Wildcard)
	assert.False(t, resp.Proxied)
}

// =============================================================================
// server lookup failure
// =============================================================================

func TestValidateDNSAgainstServerMissingServer(t *testing.T) {
	svc, _ := dnsValidateTestService(t, &fakeDNSLookuper{})

	_, err := svc.validateDNSAgainstServer(context.Background(), "example.com", "does-not-exist")
	assert.Error(t, err)
}

// =============================================================================
// DNS lookup failure
// =============================================================================

func TestValidateDNSAgainstServerLookupFailure(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{err: errors.New("no such host")})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.False(t, resp.Proxied)
	assert.Contains(t, resp.Message, "DNS lookup failed")
}

// =============================================================================
// resolves correctly
// =============================================================================

func TestValidateDNSAgainstServerResolvesToExpectedIP(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"203.0.113.10"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.False(t, resp.Proxied)
	assert.Contains(t, resp.Message, "203.0.113.10")
	assert.Contains(t, resp.Message, "✓")
}

// =============================================================================
// resolves to an unrelated, non-Cloudflare IP — a real misconfiguration,
// not something proxied.
// =============================================================================

func TestValidateDNSAgainstServerResolvesElsewhere(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"198.51.100.20"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.False(t, resp.Proxied)
	assert.Contains(t, resp.Message, "198.51.100.20")
	assert.Contains(t, resp.Message, "expected 203.0.113.10")
}

// =============================================================================
// resolves to a Cloudflare edge IP — the actual bug this PR fixes. Not a
// misconfiguration; the origin is deliberately hidden behind the proxy.
// =============================================================================

func TestValidateDNSAgainstServerResolvesToCloudflareEdge(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"104.16.132.229"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK, "the origin behind Cloudflare is unverifiable, not confirmed correct")
	assert.True(t, resp.Proxied)
	assert.Contains(t, resp.Message, "Proxied through Cloudflare")
	assert.Contains(t, resp.Message, "104.16.132.229")
	assert.Contains(t, resp.Message, "203.0.113.10")
}

// A resolver can return more than one A record. If any of them matches the
// server, that takes priority over a Cloudflare IP also being present —
// OK wins even when the answer is mixed.
func TestValidateDNSAgainstServerMatchWinsOverCloudflareInMixedAnswer(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{
		addrs: []string{"104.16.132.229", "203.0.113.10"},
	})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.Contains(t, resp.Message, "✓")
}

// =============================================================================
// no A records at all
// =============================================================================

func TestValidateDNSAgainstServerNoARecords(t *testing.T) {
	// An IPv6-only answer: To4() returns nil for a real IPv6 address, so it
	// never lands in ResolvedIPs. This is a pre-existing gap (the function
	// only ever looks at A records), reproduced here rather than papered
	// over, since fixing IPv6 support is a separate change.
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"2606:4700::1111"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.False(t, resp.Proxied)
	assert.Empty(t, resp.ResolvedIPs)
	assert.Contains(t, resp.Message, "doesn't resolve to any A record yet")
}

// =============================================================================
// server has no public IP recorded yet
// =============================================================================

func TestValidateDNSAgainstServerNoExpectedIP(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"198.51.100.20"}})
	serverID := createTestServer(t, db, "")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK, "an empty expected IP can never match")
	assert.Empty(t, resp.ExpectedIP)
	assert.Contains(t, resp.Message, "198.51.100.20")
}

// =============================================================================
// host normalization
// =============================================================================

func TestValidateDNSAgainstServerNormalizesHost(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"203.0.113.10"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "  EXAMPLE.com  ", serverID)
	require.NoError(t, err)
	assert.Equal(t, "example.com", resp.Host)
	assert.True(t, strings.EqualFold(resp.Host, "example.com"))
}
