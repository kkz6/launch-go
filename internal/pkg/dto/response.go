package dto

import "time"

// TimestampResponse provides a standard mixin for API responses that include
// created_at and updated_at timestamps. Embed this struct in response DTOs
// to get consistent timestamp handling.
//
// Example:
//
//	type UserResponse struct {
//	    ID   string `json:"id"`
//	    Name string `json:"name"`
//	    dto.TimestampResponse
//	}
//
//	func ToUserResponse(u *models.User) UserResponse {
//	    return UserResponse{
//	        ID:   u.ID,
//	        Name: u.Name,
//	        TimestampResponse: dto.NewTimestampResponse(u.CreatedAt, u.UpdatedAt),
//	    }
//	}
type TimestampResponse struct {
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// NewTimestampResponse creates a TimestampResponse from time pointers.
// Nil times result in empty strings.
func NewTimestampResponse(createdAt, updatedAt *time.Time) TimestampResponse {
	return TimestampResponse{
		CreatedAt: FormatTimeOrEmpty(createdAt),
		UpdatedAt: FormatTimeOrEmpty(updatedAt),
	}
}

// NewTimestampResponseFromValues creates a TimestampResponse from time values.
func NewTimestampResponseFromValues(createdAt, updatedAt time.Time) TimestampResponse {
	return TimestampResponse{
		CreatedAt: FormatTimeValue(createdAt),
		UpdatedAt: FormatTimeValue(updatedAt),
	}
}

// TimestampPtrResponse is like TimestampResponse but uses string pointers.
// Use this when timestamps are optional in the fiberctx.
//
// Example:
//
//	type TaskResponse struct {
//	    ID string `json:"id"`
//	    dto.TimestampPtrResponse
//	}
type TimestampPtrResponse struct {
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// NewTimestampPtrResponse creates a TimestampPtrResponse from time pointers.
func NewTimestampPtrResponse(createdAt, updatedAt *time.Time) TimestampPtrResponse {
	return TimestampPtrResponse{
		CreatedAt: FormatTime(createdAt),
		UpdatedAt: FormatTime(updatedAt),
	}
}

// AuditTimestampResponse extends TimestampResponse with deleted_at for soft-deleted records.
type AuditTimestampResponse struct {
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	DeletedAt *string `json:"deleted_at,omitempty"`
}

// NewAuditTimestampResponse creates an AuditTimestampResponse.
func NewAuditTimestampResponse(createdAt, updatedAt, deletedAt *time.Time) AuditTimestampResponse {
	return AuditTimestampResponse{
		CreatedAt: FormatTimeOrEmpty(createdAt),
		UpdatedAt: FormatTimeOrEmpty(updatedAt),
		DeletedAt: FormatTime(deletedAt),
	}
}

// InstallableTimestampResponse extends TimestampResponse with installed_at for
// resources that have an installation lifecycle (e.g., firewall rules, cron jobs).
type InstallableTimestampResponse struct {
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	InstalledAt *string `json:"installed_at,omitempty"`
}

// NewInstallableTimestampResponse creates an InstallableTimestampResponse.
func NewInstallableTimestampResponse(createdAt, updatedAt, installedAt *time.Time) InstallableTimestampResponse {
	return InstallableTimestampResponse{
		CreatedAt:   FormatTimeOrEmpty(createdAt),
		UpdatedAt:   FormatTimeOrEmpty(updatedAt),
		InstalledAt: FormatTime(installedAt),
	}
}

// PaginationMeta represents pagination metadata in list responses.
type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	Total       int64 `json:"total"`
	LastPage    int   `json:"last_page"`
}

// NewPaginationMeta creates pagination metadata.
func NewPaginationMeta(page, perPage int, total int64) PaginationMeta {
	lastPage := int(total) / perPage
	if int(total)%perPage > 0 {
		lastPage++
	}
	if lastPage < 1 {
		lastPage = 1
	}
	return PaginationMeta{
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		LastPage:    lastPage,
	}
}

// APIResponse is the standard wrapper for all API responses.
//
// Example success response:
//
//	{
//	    "success": true,
//	    "message": "Resource retrieved successfully",
//	    "data": { ... }
//	}
//
// Example error response:
//
//	{
//	    "success": false,
//	    "message": "Validation failed",
//	    "errors": { "field": ["Error message"] }
//	}
type APIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Data    any             `json:"data,omitempty"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
	// Code is an optional machine-readable error code (e.g. "auth.email_taken")
	// populated when an AppError flows through the global error handler.
	// Frontends can branch on this without parsing Message.
	Code   string            `json:"code,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

// NewSuccessResponse creates a successful API fiberctx.
func NewSuccessResponse(message string, data any) APIResponse {
	return APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// NewSuccessResponseWithMeta creates a successful API response with pagination.
func NewSuccessResponseWithMeta(message string, data any, meta PaginationMeta) APIResponse {
	return APIResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    &meta,
	}
}

// NewErrorResponse creates an error API fiberctx.
func NewErrorResponse(message string, errors map[string]string) APIResponse {
	return APIResponse{
		Success: false,
		Message: message,
		Errors:  errors,
	}
}
