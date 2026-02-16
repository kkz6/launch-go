package dto

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
	Password  string   `json:"password" validate:"omitempty,min=8,max=255"`
	Databases []string `json:"databases" validate:"omitempty,dive,len=26"`
}
