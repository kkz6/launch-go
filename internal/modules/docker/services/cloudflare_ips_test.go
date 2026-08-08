package services

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A domain proxied through Cloudflare resolves to Cloudflare's edge, not the
// origin — isCloudflareIP is what tells that state apart from a domain that
// genuinely points somewhere wrong. Values below are real addresses inside
// each published range, not just range boundaries, so the test fails if the
// list is ever pasted in shifted or truncated.
func TestIsCloudflareIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "cloudflare 104.16.x range", ip: "104.16.132.229", want: true},
		{name: "cloudflare 172.64.x range", ip: "172.64.100.1", want: true},
		{name: "cloudflare 162.158.x range", ip: "162.158.78.13", want: true},
		{name: "cloudflare 173.245.48.0/20 lower bound", ip: "173.245.48.0", want: true},
		{name: "cloudflare 173.245.48.0/20 upper bound", ip: "173.245.63.255", want: true},
		{name: "one below the 173.245.48.0/20 range", ip: "173.245.47.255", want: false},
		{name: "one above the 173.245.48.0/20 range", ip: "173.245.64.0", want: false},
		{name: "cloudflare ipv6 range", ip: "2606:4700:1234::1", want: true},

		{name: "a real origin server IP", ip: "203.0.113.10", want: false},
		{name: "google DNS, not cloudflare", ip: "8.8.8.8", want: false},
		{name: "cloudflare's own public DNS resolver is not in the proxy ranges", ip: "1.1.1.1", want: false},
		{name: "private range", ip: "10.0.0.1", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			assert.NotNil(t, ip, "test fixture %q must parse", tc.ip)
			assert.Equal(t, tc.want, isCloudflareIP(ip))
		})
	}
}

func TestMustParseCIDRsPanicsOnAnInvalidLiteral(t *testing.T) {
	assert.Panics(t, func() {
		mustParseCIDRs("not-a-cidr")
	})
}
