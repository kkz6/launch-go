package dto

import (
	"time"

	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
)

// SiteSummary is the explicit allow-list view of a site returned by the
// back-office /admin/users/:id/sites endpoint. The raw Site model already tags
// every secret field (deploy token, deploy keys, VCS data, type data) as
// json:"-", but this endpoint is cross-tenant, so this DTO is a defense-in-depth
// allow-list: only fields enumerated here ever serialize. A future sensitive
// field added to Site cannot leak through this endpoint unless deliberately
// added below. ServerName is folded in so the UI can show which server each
// site is on.
type SiteSummary struct {
	ID               string     `json:"id"`
	ServerID         string     `json:"server_id"`
	ServerName       string     `json:"server_name"`
	Address          string     `json:"address"`
	Type             string     `json:"type"`
	TLSSetting       string     `json:"tls_setting"`
	RepositoryBranch *string    `json:"repository_branch,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
}

// NewSiteSummary maps a Site model to its safe summary view, folding in the
// owning server's name from the supplied serverID→name lookup.
func NewSiteSummary(s sitemodels.Site, serverNames map[string]string) SiteSummary {
	return SiteSummary{
		ID:               s.ID,
		ServerID:         s.ServerID,
		ServerName:       serverNames[s.ServerID],
		Address:          s.Address,
		Type:             s.Type.String(),
		TLSSetting:       s.TLSSetting.String(),
		RepositoryBranch: s.RepositoryBranch,
		CreatedAt:        s.CreatedAt,
	}
}

// NewSiteSummaries maps a slice of Site models to summary views.
func NewSiteSummaries(sites []sitemodels.Site, serverNames map[string]string) []SiteSummary {
	summaries := make([]SiteSummary, 0, len(sites))
	for _, s := range sites {
		summaries = append(summaries, NewSiteSummary(s, serverNames))
	}

	return summaries
}
