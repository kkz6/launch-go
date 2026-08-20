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
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

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

func dnsValidateTestService(t *testing.T, lookuper dnsLookuper) (*DomainService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// ServerRepository preloads the Services relation on every FindByID.
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

func TestValidateDNSAgainstServerWildcardSuffix(t *testing.T) {
	svc, _ := dnsValidateTestService(t, &fakeDNSLookuper{err: errors.New("must not be called")})

	// traefik.me is not in wildcardDNSSuffixes; sslip.io is.
	resp, err := svc.validateDNSAgainstServer(context.Background(), "app.sslip.io", "does-not-exist")
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.True(t, resp.Wildcard)
	assert.False(t, resp.Proxied)
}

func TestValidateDNSAgainstServerMissingServer(t *testing.T) {
	svc, _ := dnsValidateTestService(t, &fakeDNSLookuper{})

	_, err := svc.validateDNSAgainstServer(context.Background(), "example.com", "does-not-exist")
	assert.Error(t, err)
}

func TestValidateDNSAgainstServerLookupFailure(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{err: errors.New("no such host")})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.False(t, resp.Proxied)
	assert.Equal(t, "DNS lookup failed: no such host", resp.Message)
}

func TestValidateDNSAgainstServerLocalizesJapaneseMessages(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		svc, _ := dnsValidateTestService(t, &fakeDNSLookuper{err: errors.New("must not be called")})
		ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)

		resp, err := svc.validateDNSAgainstServer(ctx, "app.sslip.io", "does-not-exist")

		require.NoError(t, err)
		assert.True(t, resp.OK)
		assert.Contains(t, resp.Message, "ワイルドカードDNS")
	})

	t.Run("lookup failure hides resolver details", func(t *testing.T) {
		svc, db := dnsValidateTestService(t, &fakeDNSLookuper{err: errors.New("resolver secret diagnostic")})
		serverID := createTestServer(t, db, "203.0.113.10")
		ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)

		resp, err := svc.validateDNSAgainstServer(ctx, "example.com", serverID)

		require.NoError(t, err)
		assert.False(t, resp.OK)
		assert.Equal(t, "DNSの検索に失敗しました。", resp.Message)
		assert.NotContains(t, resp.Message, "secret diagnostic")
	})

	t.Run("mismatched IPs remain structured in translated copy", func(t *testing.T) {
		svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"198.51.100.20"}})
		serverID := createTestServer(t, db, "203.0.113.10")
		ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)

		resp, err := svc.validateDNSAgainstServer(ctx, "example.com", serverID)

		require.NoError(t, err)
		assert.Equal(t, []string{"198.51.100.20"}, resp.ResolvedIPs)
		assert.Equal(t, "203.0.113.10", resp.ExpectedIP)
		assert.Contains(t, resp.Message, "198.51.100.20")
		assert.Contains(t, resp.Message, "203.0.113.10")
		assert.Contains(t, resp.Message, "期待されるIP")
	})

	t.Run("Cloudflare explanation", func(t *testing.T) {
		svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"104.16.132.229"}})
		serverID := createTestServer(t, db, "203.0.113.10")
		ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)

		resp, err := svc.validateDNSAgainstServer(ctx, "example.com", serverID)

		require.NoError(t, err)
		assert.True(t, resp.Proxied)
		assert.Contains(t, resp.Message, "Cloudflare経由")
		assert.Contains(t, resp.Message, "104.16.132.229")
		assert.Contains(t, resp.Message, "203.0.113.10")
	})
}

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

func TestValidateDNSAgainstServerResolvesToCloudflareEdge(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"104.16.132.229"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.True(t, resp.Proxied)
	assert.Contains(t, resp.Message, "Proxied through Cloudflare")
	assert.Contains(t, resp.Message, "104.16.132.229")
	assert.Contains(t, resp.Message, "203.0.113.10")
}

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

func TestValidateDNSAgainstServerNoARecords(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"2606:4700::1111"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.False(t, resp.Proxied)
	assert.Empty(t, resp.ResolvedIPs)
	assert.Contains(t, resp.Message, "doesn't resolve to any A record yet")
}

func TestValidateDNSAgainstServerNoExpectedIP(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"198.51.100.20"}})
	serverID := createTestServer(t, db, "")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "example.com", serverID)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.Empty(t, resp.ExpectedIP)
	assert.Contains(t, resp.Message, "198.51.100.20")
}

func TestValidateDNSAgainstServerNormalizesHost(t *testing.T) {
	svc, db := dnsValidateTestService(t, &fakeDNSLookuper{addrs: []string{"203.0.113.10"}})
	serverID := createTestServer(t, db, "203.0.113.10")

	resp, err := svc.validateDNSAgainstServer(context.Background(), "  EXAMPLE.com  ", serverID)
	require.NoError(t, err)
	assert.Equal(t, "example.com", resp.Host)
	assert.True(t, strings.EqualFold(resp.Host, "example.com"))
}
