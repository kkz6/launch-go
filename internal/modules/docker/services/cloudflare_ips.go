package services

import "net"

// cloudflareIPRanges is Cloudflare's published proxy edge — any A/AAAA
// record answering from one of these belongs to Cloudflare, not to the
// origin server behind it. A domain resolving here is not misconfigured;
// it is intentionally proxied (the DNS record's orange cloud is on), and
// the real origin IP is deliberately hidden from public DNS.
//
// Fetched directly from https://www.cloudflare.com/ips-v4 and
// https://www.cloudflare.com/ips-v6 on 2026-08-06. Cloudflare adds ranges
// rarely — re-fetch both URLs and update this list if validation starts
// mislabeling a proxied domain as misconfigured.
var cloudflareIPRanges = mustParseCIDRs(
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
)

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// Only reachable from a typo in the literal list above, at
			// package init — never from user input.
			panic("services: invalid Cloudflare CIDR literal " + cidr + ": " + err.Error())
		}
		nets = append(nets, ipNet)
	}
	return nets
}

// isCloudflareIP reports whether ip belongs to Cloudflare's published proxy
// ranges. Distinguishes "this domain is proxied through Cloudflare, which
// is why it doesn't resolve straight to the origin" from "this domain
// points somewhere genuinely wrong" in DNS validation.
func isCloudflareIP(ip net.IP) bool {
	for _, ipNet := range cloudflareIPRanges {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
