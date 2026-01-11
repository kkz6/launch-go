package database

import "time"

// CreateDatabaseRequest represents the request to create a database
type CreateDatabaseRequest struct {
	Name           string  `json:"name" validate:"required,min=1,max=64"`
	CreateUser     bool    `json:"create_user"`
	ExistingUserID *string `json:"existing_user_id" validate:"omitempty,len=26"`
	UserName       string  `json:"user_name" validate:"required_if=CreateUser true,omitempty,min=1,max=32"`
	UserPassword   string  `json:"user_password" validate:"required_if=CreateUser true,omitempty,min=8"`
}

// CreateDatabaseUserRequest represents the request to create a database user
type CreateDatabaseUserRequest struct {
	Name      string   `json:"name" validate:"required,min=1,max=32"`
	Password  string   `json:"password" validate:"required,min=8,max=255"`
	Databases []string `json:"databases" validate:"omitempty,dive,len=26"`
}

// UpdateDatabaseUserRequest represents the request to update a database user
type UpdateDatabaseUserRequest struct {
	Password  string   `json:"password" validate:"required,min=8,max=255"`
	Databases []string `json:"databases" validate:"omitempty,dive,len=26"`
}

// DatabaseResponse represents the response for a database
type DatabaseResponse struct {
	ID                        string               `json:"id"`
	ServerID                  string               `json:"server_id"`
	Name                      string               `json:"name"`
	Status                    string               `json:"status"`
	InstalledAt               *string              `json:"installed_at,omitempty"`
	InstallationFailedAt      *string              `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *string              `json:"uninstallation_requested_at,omitempty"`
	CreatedAt                 string               `json:"created_at"`
	UpdatedAt                 string               `json:"updated_at"`
	Users                     []DatabaseUserBrief  `json:"users,omitempty"`
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
func ToDatabaseResponse(db *Database) DatabaseResponse {
	resp := DatabaseResponse{
		ID:        db.ID,
		ServerID:  db.ServerID,
		Name:      db.Name,
		Status:    string(db.Status()),
		CreatedAt: db.CreatedAt.Format(time.RFC3339),
		UpdatedAt: db.UpdatedAt.Format(time.RFC3339),
	}

	if db.InstalledAt != nil {
		t := db.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &t
	}

	if db.InstallationFailedAt != nil {
		t := db.InstallationFailedAt.Format(time.RFC3339)
		resp.InstallationFailedAt = &t
	}

	if db.UninstallationRequestedAt != nil {
		t := db.UninstallationRequestedAt.Format(time.RFC3339)
		resp.UninstallationRequestedAt = &t
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
func ToDatabaseUserResponse(user *DatabaseUser) DatabaseUserResponse {
	resp := DatabaseUserResponse{
		ID:        user.ID,
		ServerID:  user.ServerID,
		Name:      user.Name,
		Host:      user.Host,
		Status:    string(user.Status()),
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}

	if user.InstalledAt != nil {
		t := user.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &t
	}

	if user.InstallationFailedAt != nil {
		t := user.InstallationFailedAt.Format(time.RFC3339)
		resp.InstallationFailedAt = &t
	}

	if user.UninstallationRequestedAt != nil {
		t := user.UninstallationRequestedAt.Format(time.RFC3339)
		resp.UninstallationRequestedAt = &t
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
func ToDatabaseResponseList(databases []Database) []DatabaseResponse {
	result := make([]DatabaseResponse, len(databases))
	for i, db := range databases {
		result[i] = ToDatabaseResponse(&db)
	}

	return result
}

// ToDatabaseUserResponseList converts a slice of DatabaseUser models to a slice of DatabaseUserResponse
func ToDatabaseUserResponseList(users []DatabaseUser) []DatabaseUserResponse {
	result := make([]DatabaseUserResponse, len(users))
	for i, user := range users {
		result[i] = ToDatabaseUserResponse(&user)
	}

	return result
}
