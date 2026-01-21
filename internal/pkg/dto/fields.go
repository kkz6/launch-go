package dto

import "strings"

// =============================================================================
// Email Fields
// =============================================================================

// EmailField provides a standardized email field with validation.
// Embed this struct in DTOs to get consistent email handling.
//
// Example:
//
//	type UserDTO struct {
//	    dto.EmailField
//	    Name string `json:"name"`
//	}
//
// The email field includes validation for required and valid email format.
type EmailField struct {
	Email string `json:"email" validate:"required,email"`
}

// Normalize lowercases and trims the email.
func (f *EmailField) Normalize() {
	f.Email = strings.ToLower(strings.TrimSpace(f.Email))
}

// OptionalEmailField provides an optional email field that is validated when present.
//
// Example:
//
//	type UpdateUserDTO struct {
//	    dto.OptionalEmailField
//	    Name *string `json:"name"`
//	}
type OptionalEmailField struct {
	Email string `json:"email" validate:"omitempty,email"`
}

// Normalize lowercases and trims the email.
func (f *OptionalEmailField) Normalize() {
	f.Email = strings.ToLower(strings.TrimSpace(f.Email))
}

// =============================================================================
// Name Fields
// =============================================================================

// NameField provides a standardized name field with validation.
// Embed this struct in DTOs to get consistent name handling.
//
// Example:
//
//	type TeamDTO struct {
//	    dto.NameField
//	    Description string `json:"description"`
//	}
//
// The name field requires minimum 2 and maximum 255 characters.
type NameField struct {
	Name string `json:"name" validate:"required,min=2,max=255"`
}

// Normalize trims whitespace from the name.
func (f *NameField) Normalize() {
	f.Name = strings.TrimSpace(f.Name)
}

// OptionalNameField provides an optional name field that is validated when present.
//
// Example:
//
//	type UpdateTeamDTO struct {
//	    dto.OptionalNameField
//	    Description *string `json:"description"`
//	}
type OptionalNameField struct {
	Name string `json:"name" validate:"omitempty,min=2,max=255"`
}

// Normalize trims whitespace from the name.
func (f *OptionalNameField) Normalize() {
	f.Name = strings.TrimSpace(f.Name)
}

// =============================================================================
// Password Fields
// =============================================================================

// PasswordField provides a standardized password field with validation.
// Use this for simple password fields without confirmation requirement.
//
// Example:
//
//	type LoginDTO struct {
//	    dto.EmailField
//	    dto.PasswordField
//	}
//
// The password field requires minimum 8 characters.
type PasswordField struct {
	Password string `json:"password" validate:"required,min=8"`
}

// PasswordWithConfirmation provides password with confirmation validation.
// Use this for registration and password reset flows where confirmation is required.
//
// Example:
//
//	type RegisterDTO struct {
//	    dto.EmailField
//	    dto.NameField
//	    dto.PasswordWithConfirmation
//	}
//
// The password field requires minimum 8 characters and confirmation must match.
type PasswordWithConfirmation struct {
	Password             string `json:"password" validate:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// OptionalPasswordField provides an optional password field that is validated when present.
// Use this for update flows where password change is optional.
//
// Example:
//
//	type UpdateProfileDTO struct {
//	    dto.OptionalNameField
//	    dto.OptionalPasswordField
//	}
type OptionalPasswordField struct {
	Password string `json:"password" validate:"omitempty,min=8"`
}

// OptionalPasswordWithConfirmation provides optional password with confirmation.
// Use this for update flows where password change is optional but must include confirmation.
//
// Example:
//
//	type UpdateUserDTO struct {
//	    dto.OptionalNameField
//	    dto.OptionalPasswordWithConfirmation
//	}
//
// When password is provided, it must be at least 8 characters and confirmation must match.
type OptionalPasswordWithConfirmation struct {
	Password             string `json:"password" validate:"omitempty,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required_with=Password,eqfield=Password"`
}

// =============================================================================
// URL Fields
// =============================================================================

// URLField provides a standardized URL field with validation.
//
// Example:
//
//	type WebhookDTO struct {
//	    dto.URLField
//	    Secret string `json:"secret"`
//	}
type URLField struct {
	URL string `json:"url" validate:"required,url"`
}

// Normalize trims whitespace from the URL.
func (f *URLField) Normalize() {
	f.URL = strings.TrimSpace(f.URL)
}

// OptionalURLField provides an optional URL field that is validated when present.
//
// Example:
//
//	type UpdateWebhookDTO struct {
//	    dto.OptionalURLField
//	}
type OptionalURLField struct {
	URL string `json:"url" validate:"omitempty,url"`
}

// Normalize trims whitespace from the URL.
func (f *OptionalURLField) Normalize() {
	f.URL = strings.TrimSpace(f.URL)
}

// =============================================================================
// Domain Fields
// =============================================================================

// DomainField provides a standardized domain field with FQDN validation.
//
// Example:
//
//	type SiteDTO struct {
//	    dto.DomainField
//	    ServerID string `json:"server_id"`
//	}
type DomainField struct {
	Domain string `json:"domain" validate:"required,fqdn"`
}

// Normalize lowercases and trims the domain.
func (f *DomainField) Normalize() {
	f.Domain = strings.ToLower(strings.TrimSpace(f.Domain))
}

// OptionalDomainField provides an optional domain field that is validated when present.
//
// Example:
//
//	type UpdateSiteDTO struct {
//	    dto.OptionalDomainField
//	}
type OptionalDomainField struct {
	Domain string `json:"domain" validate:"omitempty,fqdn"`
}

// Normalize lowercases and trims the domain.
func (f *OptionalDomainField) Normalize() {
	f.Domain = strings.ToLower(strings.TrimSpace(f.Domain))
}

// =============================================================================
// IP Address Fields
// =============================================================================

// IPAddressField provides a standardized IP address field with validation.
//
// Example:
//
//	type FirewallRuleDTO struct {
//	    dto.IPAddressField
//	    Port int `json:"port"`
//	}
type IPAddressField struct {
	IPAddress string `json:"ip_address" validate:"required,ip"`
}

// OptionalIPAddressField provides an optional IP address field that is validated when present.
//
// Example:
//
//	type UpdateFirewallRuleDTO struct {
//	    dto.OptionalIPAddressField
//	}
type OptionalIPAddressField struct {
	IPAddress string `json:"ip_address" validate:"omitempty,ip"`
}

// =============================================================================
// Description Fields
// =============================================================================

// DescriptionField provides a standardized description field with validation.
//
// Example:
//
//	type ProjectDTO struct {
//	    dto.NameField
//	    dto.DescriptionField
//	}
//
// The description field has a maximum of 1000 characters.
type DescriptionField struct {
	Description string `json:"description" validate:"required,max=1000"`
}

// Normalize trims whitespace from the description.
func (f *DescriptionField) Normalize() {
	f.Description = strings.TrimSpace(f.Description)
}

// OptionalDescriptionField provides an optional description field that is validated when present.
//
// Example:
//
//	type UpdateProjectDTO struct {
//	    dto.OptionalNameField
//	    dto.OptionalDescriptionField
//	}
type OptionalDescriptionField struct {
	Description string `json:"description" validate:"omitempty,max=1000"`
}

// Normalize trims whitespace from the description.
func (f *OptionalDescriptionField) Normalize() {
	f.Description = strings.TrimSpace(f.Description)
}

// =============================================================================
// Username Fields
// =============================================================================

// UsernameField provides a standardized username field with validation.
// Usernames must be alphanumeric with underscores, 3-50 characters.
//
// Example:
//
//	type CreateDatabaseUserDTO struct {
//	    dto.UsernameField
//	    dto.PasswordField
//	}
type UsernameField struct {
	Username string `json:"username" validate:"required,min=3,max=50,alphanum"`
}

// Normalize lowercases and trims the username.
func (f *UsernameField) Normalize() {
	f.Username = strings.ToLower(strings.TrimSpace(f.Username))
}

// OptionalUsernameField provides an optional username field that is validated when present.
//
// Example:
//
//	type UpdateDatabaseUserDTO struct {
//	    dto.OptionalUsernameField
//	}
type OptionalUsernameField struct {
	Username string `json:"username" validate:"omitempty,min=3,max=50,alphanum"`
}

// Normalize lowercases and trims the username.
func (f *OptionalUsernameField) Normalize() {
	f.Username = strings.ToLower(strings.TrimSpace(f.Username))
}

// =============================================================================
// Port Fields
// =============================================================================

// PortField provides a standardized port field with validation.
// Ports must be between 1 and 65535.
//
// Example:
//
//	type ServiceDTO struct {
//	    dto.NameField
//	    dto.PortField
//	}
type PortField struct {
	Port int `json:"port" validate:"required,min=1,max=65535"`
}

// OptionalPortField provides an optional port field that is validated when present.
//
// Example:
//
//	type UpdateServiceDTO struct {
//	    dto.OptionalPortField
//	}
type OptionalPortField struct {
	Port *int `json:"port" validate:"omitempty,min=1,max=65535"`
}
