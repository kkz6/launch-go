package dto

import "strings"

// EmailRequest provides a standard mixin for API requests that require an email field.
// Embed this struct in request DTOs to get consistent email handling with validation.
//
// Example:
//
//	type InviteRequest struct {
//	    dto.EmailRequest
//	    Role string `json:"role" validate:"required"`
//	}
//
// The email field includes validation for required and valid email format.
type EmailRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// Normalize lowercases and trims the email.
func (r *EmailRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// OptionalEmailRequest provides an email field that is not required but validated when present.
//
// Example:
//
//	type NotificationSettingsRequest struct {
//	    dto.OptionalEmailRequest
//	    SlackWebhook string `json:"slack_webhook"`
//	}
type OptionalEmailRequest struct {
	Email string `json:"email" validate:"omitempty,email"`
}

// Normalize lowercases and trims the email.
func (r *OptionalEmailRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// PasswordRequest provides a standard mixin for API requests that require password
// with confirmation. Embed this struct in request DTOs to get consistent password
// handling with validation.
//
// Example:
//
//	type ResetPasswordRequest struct {
//	    Token string `json:"token" validate:"required"`
//	    dto.PasswordRequest
//	}
//
// The password field requires minimum 8 characters and confirmation must match.
type PasswordRequest struct {
	Password             string `json:"password" validate:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// NewPasswordRequest provides a standard mixin for password fields in creation/update
// requests where the password may be optional but confirmation is still required when provided.
//
// Example:
//
//	type UpdateUserRequest struct {
//	    Name string `json:"name" validate:"required"`
//	    dto.NewPasswordRequest
//	}
type NewPasswordRequest struct {
	Password             string `json:"password" validate:"omitempty,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required_with=Password,eqfield=Password"`
}

// NameRequest provides a standard mixin for API requests that require a name field.
// Embed this struct in request DTOs to get consistent name handling with validation.
//
// Example:
//
//	type CreateTeamRequest struct {
//	    dto.NameRequest
//	    Description string `json:"description"`
//	}
//
// The name field requires minimum 2 and maximum 255 characters.
type NameRequest struct {
	Name string `json:"name" validate:"required,min=2,max=255"`
}

// Normalize trims whitespace from the name.
func (r *NameRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
}

// OptionalNameRequest provides a name field that is not required but validated when present.
//
// Example:
//
//	type UpdateTeamRequest struct {
//	    dto.OptionalNameRequest
//	    Description *string `json:"description"`
//	}
type OptionalNameRequest struct {
	Name *string `json:"name" validate:"omitempty,min=2,max=255"`
}

// Normalize trims whitespace from the name if present.
func (r *OptionalNameRequest) Normalize() {
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		r.Name = &trimmed
	}
}

// PaginationRequest provides standard pagination fields for list API requests.
// Embed this struct in request DTOs to get consistent pagination handling.
//
// Example:
//
//	type ListServersRequest struct {
//	    dto.PaginationRequest
//	    Status string `json:"status" validate:"omitempty"`
//	}
//
//	func (r *ListServersRequest) GetPage() int {
//	    return r.PaginationRequest.GetPage()
//	}
//
// Default values: Page = 1, PerPage = 15
// Maximum PerPage is 100.
type PaginationRequest struct {
	Page    int `json:"page" validate:"omitempty,min=1"`
	PerPage int `json:"per_page" validate:"omitempty,min=1,max=100"`
}

// GetPage returns the page number, defaulting to 1 if not set.
func (r *PaginationRequest) GetPage() int {
	if r.Page < 1 {
		return 1
	}

	return r.Page
}

// GetPerPage returns the per-page count, defaulting to 15 if not set.
// Maximum value is capped at 100.
func (r *PaginationRequest) GetPerPage() int {
	if r.PerPage < 1 {
		return 15
	}

	if r.PerPage > 100 {
		return 100
	}

	return r.PerPage
}

// GetOffset returns the offset for database queries based on page and per-page.
func (r *PaginationRequest) GetOffset() int {
	return (r.GetPage() - 1) * r.GetPerPage()
}

// IDRequest provides a standard mixin for API requests that require an ID field.
// Embed this struct in request DTOs to get consistent ID handling with ULID validation.
//
// Example:
//
//	type DeleteResourceRequest struct {
//	    dto.IDRequest
//	}
//
// The ID field is validated as a required ULID.
type IDRequest struct {
	ID string `json:"id" validate:"required,ulid"`
}

// OptionalIDRequest provides an ID field that is not required but validated as ULID when present.
//
// Example:
//
//	type FilterRequest struct {
//	    dto.OptionalIDRequest
//	    Status string `json:"status"`
//	}
type OptionalIDRequest struct {
	ID *string `json:"id" validate:"omitempty,ulid"`
}

// ResourceIDRequest provides a standard mixin for requests that reference a parent resource.
// Use this when you need to identify a specific resource by its ID.
//
// Example:
//
//	type CreateCommentRequest struct {
//	    dto.ResourceIDRequest
//	    Content string `json:"content" validate:"required"`
//	}
type ResourceIDRequest struct {
	ResourceID string `json:"resource_id" validate:"required,ulid"`
}

// ServerIDRequest provides a server_id field for requests that target a specific server.
//
// Example:
//
//	type CreateSiteRequest struct {
//	    dto.ServerIDRequest
//	    Address string `json:"address" validate:"required"`
//	}
type ServerIDRequest struct {
	ServerID string `json:"server_id" validate:"required,ulid"`
}

// SiteIDRequest provides a site_id field for requests that target a specific site.
//
// Example:
//
//	type CreateDeploymentRequest struct {
//	    dto.SiteIDRequest
//	    Branch string `json:"branch"`
//	}
type SiteIDRequest struct {
	SiteID string `json:"site_id" validate:"required,ulid"`
}

// TeamIDRequest provides a team_id field for requests that target a specific team.
//
// Example:
//
//	type InviteMemberRequest struct {
//	    dto.TeamIDRequest
//	    Email string `json:"email" validate:"required,email"`
//	}
type TeamIDRequest struct {
	TeamID string `json:"team_id" validate:"required,ulid"`
}
