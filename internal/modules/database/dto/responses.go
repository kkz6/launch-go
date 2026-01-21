package dto

import (
	"github.com/kkz6/launch-go/internal/modules/database/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// DatabaseResponse represents the response for a database
type DatabaseResponse struct {
	ID                        string              `json:"id"`
	ServerID                  string              `json:"server_id"`
	Name                      string              `json:"name"`
	Status                    string              `json:"status"`
	InstalledAt               *string             `json:"installed_at,omitempty"`
	InstallationFailedAt      *string             `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *string             `json:"uninstallation_requested_at,omitempty"`
	CreatedAt                 string              `json:"created_at"`
	UpdatedAt                 string              `json:"updated_at"`
	Users                     []DatabaseUserBrief `json:"users,omitempty"`
}

// DatabaseUserBrief is a brief representation of a database user
type DatabaseUserBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DatabaseUserResponse represents the response for a database user
type DatabaseUserResponse struct {
	ID                        string          `json:"id"`
	ServerID                  string          `json:"server_id"`
	Name                      string          `json:"name"`
	Host                      string          `json:"host"`
	Status                    string          `json:"status"`
	InstalledAt               *string         `json:"installed_at,omitempty"`
	InstallationFailedAt      *string         `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *string         `json:"uninstallation_requested_at,omitempty"`
	CreatedAt                 string          `json:"created_at"`
	UpdatedAt                 string          `json:"updated_at"`
	Databases                 []DatabaseBrief `json:"databases,omitempty"`
	DatabaseIDs               []string        `json:"database_ids,omitempty"`
}

// DatabaseBrief is a brief representation of a database
type DatabaseBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ToDatabaseResponse converts a Database model to a DatabaseResponse
func ToDatabaseResponse(db *models.Database) DatabaseResponse {
	resp := DatabaseResponse{
		ID:                        db.ID,
		ServerID:                  db.ServerID,
		Name:                      db.Name,
		Status:                    string(db.Status()),
		InstalledAt:               pkgdto.FormatTime(db.InstalledAt),
		InstallationFailedAt:      pkgdto.FormatTime(db.InstallationFailedAt),
		UninstallationRequestedAt: pkgdto.FormatTime(db.UninstallationRequestedAt),
		CreatedAt:                 pkgdto.FormatTimeOrEmpty(db.CreatedAt),
		UpdatedAt:                 pkgdto.FormatTimeOrEmpty(db.UpdatedAt),
	}

	for _, user := range db.Users {
		resp.Users = append(resp.Users, DatabaseUserBrief{
			ID:   user.ID,
			Name: user.Name,
		})
	}

	return resp
}

// ToDatabaseUserResponse converts a DatabaseUser model to a DatabaseUserResponse
func ToDatabaseUserResponse(user *models.DatabaseUser) DatabaseUserResponse {
	resp := DatabaseUserResponse{
		ID:                        user.ID,
		ServerID:                  user.ServerID,
		Name:                      user.Name,
		Host:                      user.Host,
		Status:                    string(user.Status()),
		InstalledAt:               pkgdto.FormatTime(user.InstalledAt),
		InstallationFailedAt:      pkgdto.FormatTime(user.InstallationFailedAt),
		UninstallationRequestedAt: pkgdto.FormatTime(user.UninstallationRequestedAt),
		CreatedAt:                 pkgdto.FormatTimeOrEmpty(user.CreatedAt),
		UpdatedAt:                 pkgdto.FormatTimeOrEmpty(user.UpdatedAt),
	}

	for _, db := range user.Databases {
		resp.Databases = append(resp.Databases, DatabaseBrief{
			ID:   db.ID,
			Name: db.Name,
		})
		resp.DatabaseIDs = append(resp.DatabaseIDs, db.ID)
	}

	return resp
}

// ToDatabaseResponseList converts a slice of Database models to a slice of DatabaseResponse
func ToDatabaseResponseList(databases []models.Database) []DatabaseResponse {
	result := make([]DatabaseResponse, len(databases))
	for i, db := range databases {
		result[i] = ToDatabaseResponse(&db)
	}

	return result
}

// ToDatabaseUserResponseList converts a slice of DatabaseUser models to a slice of DatabaseUserResponse
func ToDatabaseUserResponseList(users []models.DatabaseUser) []DatabaseUserResponse {
	result := make([]DatabaseUserResponse, len(users))
	for i, user := range users {
		result[i] = ToDatabaseUserResponse(&user)
	}

	return result
}
