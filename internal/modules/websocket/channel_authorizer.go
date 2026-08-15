package websocket

import (
	"strings"

	"gorm.io/gorm"
)

// channelAuthorizer scopes every resource subscription to the authenticated
// team. Team and user channels are checked without a query; resource channels
// are resolved against their owning table.
type channelAuthorizer struct {
	db *gorm.DB
}

func newChannelAuthorizer(db *gorm.DB) *channelAuthorizer {
	return &channelAuthorizer{db: db}
}

func (a *channelAuthorizer) AuthorizeChannel(userID, teamID, channel string) bool {
	scope, id, ok := strings.Cut(channel, ".")
	if !ok || scope == "" || id == "" || strings.Contains(id, ".") {
		return false
	}

	switch scope {
	case "team":
		return id == teamID
	case "user":
		return id == userID
	}
	if a == nil || a.db == nil || teamID == "" {
		return false
	}

	var count int64
	query := a.db
	switch scope {
	case "server":
		query = query.Table("servers").Where("id = ? AND team_id = ?", id, teamID)
	case "site":
		query = query.Table("sites").Where("id = ? AND team_id = ?", id, teamID)
	case "deployment":
		query = query.Table("deployments").Where("id = ? AND team_id = ?", id, teamID)
	case "task":
		query = query.Table("tasks AS t").Joins("JOIN servers AS s ON s.id = t.server_id").Where("t.id = ? AND s.team_id = ?", id, teamID)
	default:
		return false
	}

	return query.Count(&count).Error == nil && count == 1
}
