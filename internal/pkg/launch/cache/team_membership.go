package cache

import (
	"gorm.io/gorm"

	basecache "github.com/kkz6/launch-go/internal/pkg/cache"
)

const TeamMembershipTTL = basecache.TeamMembershipTTL

type TeamMembership = basecache.TeamMembership
type TeamMembershipCache = basecache.TeamMembershipCache

func NewTeamMembershipCache(cache basecache.Cache, db *gorm.DB) *TeamMembershipCache {
	return basecache.NewTeamMembershipCache(cache, db)
}
