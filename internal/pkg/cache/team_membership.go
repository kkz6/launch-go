package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	// TeamMembershipTTL is the cache TTL for team membership lookups
	TeamMembershipTTL = 5 * time.Minute

	// teamMembershipKeyPrefix is the prefix for team membership cache keys
	teamMembershipKeyPrefix = "team_membership:"
)

// TeamMembership represents cached team membership data
type TeamMembership struct {
	TeamID   string `json:"team_id"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	IsMember bool   `json:"is_member"`
}

// TeamMembershipCache handles team membership caching
type TeamMembershipCache struct {
	cache Cache
	db    *gorm.DB
}

// NewTeamMembershipCache creates a new team membership cache
func NewTeamMembershipCache(cache Cache, db *gorm.DB) *TeamMembershipCache {
	return &TeamMembershipCache{
		cache: cache,
		db:    db,
	}
}

// GetMembership retrieves team membership, using cache if available
func (c *TeamMembershipCache) GetMembership(ctx context.Context, userID, teamID string) (*TeamMembership, error) {
	key := c.buildKey(userID, teamID)

	// Try cache first
	cached, err := c.cache.Get(ctx, key)
	if err == nil {
		var membership TeamMembership
		if err := json.Unmarshal([]byte(cached), &membership); err == nil {
			return &membership, nil
		}
	}

	// Cache miss or error, query database
	membership, err := c.queryMembership(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(membership); err == nil {
		_ = c.cache.Set(ctx, key, string(data), TeamMembershipTTL)
	}

	return membership, nil
}

// IsMember checks if a user is a member of a team
func (c *TeamMembershipCache) IsMember(ctx context.Context, userID, teamID string) (bool, error) {
	membership, err := c.GetMembership(ctx, userID, teamID)
	if err != nil {
		return false, err
	}
	return membership.IsMember, nil
}

// GetRole returns the user's role in a team
func (c *TeamMembershipCache) GetRole(ctx context.Context, userID, teamID string) (string, error) {
	membership, err := c.GetMembership(ctx, userID, teamID)
	if err != nil {
		return "", err
	}
	if !membership.IsMember {
		return "", fmt.Errorf("user is not a member of team")
	}
	return membership.Role, nil
}

// InvalidateMembership removes a membership entry from the cache
// Call this when team membership changes (add/remove member, role change)
func (c *TeamMembershipCache) InvalidateMembership(ctx context.Context, userID, teamID string) error {
	key := c.buildKey(userID, teamID)
	return c.cache.Delete(ctx, key)
}

// InvalidateUserMemberships invalidates all memberships for a user
// This is a best-effort operation since we don't track all teams
func (c *TeamMembershipCache) InvalidateUserMemberships(ctx context.Context, userID string) error {
	// For now, this is a no-op since we'd need to track all teams the user belongs to
	// In practice, individual invalidation when membership changes is sufficient
	return nil
}

// buildKey creates a cache key for a user-team membership
func (c *TeamMembershipCache) buildKey(userID, teamID string) string {
	return fmt.Sprintf("%s%s:%s", teamMembershipKeyPrefix, userID, teamID)
}

// queryMembership queries the database for team membership
func (c *TeamMembershipCache) queryMembership(ctx context.Context, userID, teamID string) (*TeamMembership, error) {
	// First check if user is the team owner
	var team struct {
		UserID string
	}

	err := c.db.WithContext(ctx).
		Table("teams").
		Select("user_id").
		Where("id = ?", teamID).
		First(&team).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &TeamMembership{
			TeamID:   teamID,
			UserID:   userID,
			IsMember: false,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query team: %w", err)
	}

	// If user is the team owner, they are a member with "owner" role
	if team.UserID == userID {
		return &TeamMembership{
			TeamID:   teamID,
			UserID:   userID,
			Role:     "owner",
			IsMember: true,
		}, nil
	}

	// Check if user is a member via the pivot table
	var result struct {
		Role string
	}

	err = c.db.WithContext(ctx).
		Table("team_user").
		Select("role").
		Where("user_id = ? AND team_id = ?", userID, teamID).
		First(&result).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &TeamMembership{
			TeamID:   teamID,
			UserID:   userID,
			IsMember: false,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query team membership: %w", err)
	}

	return &TeamMembership{
		TeamID:   teamID,
		UserID:   userID,
		Role:     result.Role,
		IsMember: true,
	}, nil
}
