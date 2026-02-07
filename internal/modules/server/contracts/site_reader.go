package contracts

import "context"

// SiteInfo is a read-only projection of site data — avoids importing site models
type SiteInfo struct {
	ID                     string
	ServerID               string
	TeamID                 string
	Address                string
	Type                   string
	LoadBalancedUpstreamID *string
}

// SiteReader provides read-only access to site data from the server module.
// Used by the LoadBalancerService to look up sites by address for domain conflict detection.
type SiteReader interface {
	FindByAddressAndTeam(ctx context.Context, address, teamID string) ([]SiteInfo, error)
	FindByID(ctx context.Context, id string) (*SiteInfo, error)
	IsLoadBalanced(ctx context.Context, siteID string) (bool, error)
	UpdateLoadBalancedUpstreamID(ctx context.Context, siteID string, upstreamID *string) error
}
