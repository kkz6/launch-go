package dto

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestEmailFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   EmailField
		wantErr bool
	}{
		{
			name:    "valid email",
			field:   EmailField{Email: "test@example.com"},
			wantErr: false,
		},
		{
			name:    "empty email",
			field:   EmailField{Email: ""},
			wantErr: true,
		},
		{
			name:    "invalid email format",
			field:   EmailField{Email: "not-an-email"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("EmailField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptionalEmailFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   OptionalEmailField
		wantErr bool
	}{
		{
			name:    "valid email",
			field:   OptionalEmailField{Email: "test@example.com"},
			wantErr: false,
		},
		{
			name:    "empty email allowed",
			field:   OptionalEmailField{Email: ""},
			wantErr: false,
		},
		{
			name:    "invalid email format",
			field:   OptionalEmailField{Email: "not-an-email"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("OptionalEmailField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNameFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   NameField
		wantErr bool
	}{
		{
			name:    "valid name",
			field:   NameField{Name: "John Doe"},
			wantErr: false,
		},
		{
			name:    "empty name",
			field:   NameField{Name: ""},
			wantErr: true,
		},
		{
			name:    "name too short",
			field:   NameField{Name: "J"},
			wantErr: true,
		},
		{
			name:    "minimum valid length",
			field:   NameField{Name: "Jo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("NameField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPasswordFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   PasswordField
		wantErr bool
	}{
		{
			name:    "valid password",
			field:   PasswordField{Password: "securepassword123"},
			wantErr: false,
		},
		{
			name:    "empty password",
			field:   PasswordField{Password: ""},
			wantErr: true,
		},
		{
			name:    "password too short",
			field:   PasswordField{Password: "short"},
			wantErr: true,
		},
		{
			name:    "minimum valid length",
			field:   PasswordField{Password: "12345678"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("PasswordField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPasswordWithConfirmationValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   PasswordWithConfirmation
		wantErr bool
	}{
		{
			name: "valid matching passwords",
			field: PasswordWithConfirmation{
				Password:             "securepassword123",
				PasswordConfirmation: "securepassword123",
			},
			wantErr: false,
		},
		{
			name: "passwords do not match",
			field: PasswordWithConfirmation{
				Password:             "securepassword123",
				PasswordConfirmation: "differentpassword",
			},
			wantErr: true,
		},
		{
			name: "empty password",
			field: PasswordWithConfirmation{
				Password:             "",
				PasswordConfirmation: "",
			},
			wantErr: true,
		},
		{
			name: "password too short",
			field: PasswordWithConfirmation{
				Password:             "short",
				PasswordConfirmation: "short",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("PasswordWithConfirmation validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestURLFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   URLField
		wantErr bool
	}{
		{
			name:    "valid URL",
			field:   URLField{URL: "https://example.com"},
			wantErr: false,
		},
		{
			name:    "empty URL",
			field:   URLField{URL: ""},
			wantErr: true,
		},
		{
			name:    "invalid URL",
			field:   URLField{URL: "not-a-url"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("URLField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDomainFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   DomainField
		wantErr bool
	}{
		{
			name:    "valid domain",
			field:   DomainField{Domain: "example.com"},
			wantErr: false,
		},
		{
			name:    "valid subdomain",
			field:   DomainField{Domain: "sub.example.com"},
			wantErr: false,
		},
		{
			name:    "empty domain",
			field:   DomainField{Domain: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("DomainField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIPAddressFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   IPAddressField
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			field:   IPAddressField{IPAddress: "192.168.1.1"},
			wantErr: false,
		},
		{
			name:    "valid IPv6",
			field:   IPAddressField{IPAddress: "::1"},
			wantErr: false,
		},
		{
			name:    "empty IP",
			field:   IPAddressField{IPAddress: ""},
			wantErr: true,
		},
		{
			name:    "invalid IP",
			field:   IPAddressField{IPAddress: "not-an-ip"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("IPAddressField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPortFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   PortField
		wantErr bool
	}{
		{
			name:    "valid port",
			field:   PortField{Port: 8080},
			wantErr: false,
		},
		{
			name:    "minimum port",
			field:   PortField{Port: 1},
			wantErr: false,
		},
		{
			name:    "maximum port",
			field:   PortField{Port: 65535},
			wantErr: false,
		},
		{
			name:    "zero port",
			field:   PortField{Port: 0},
			wantErr: true,
		},
		{
			name:    "port too high",
			field:   PortField{Port: 65536},
			wantErr: true,
		},
		{
			name:    "negative port",
			field:   PortField{Port: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("PortField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUsernameFieldValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		field   UsernameField
		wantErr bool
	}{
		{
			name:    "valid username",
			field:   UsernameField{Username: "johndoe123"},
			wantErr: false,
		},
		{
			name:    "empty username",
			field:   UsernameField{Username: ""},
			wantErr: true,
		},
		{
			name:    "username too short",
			field:   UsernameField{Username: "ab"},
			wantErr: true,
		},
		{
			name:    "minimum valid length",
			field:   UsernameField{Username: "abc"},
			wantErr: false,
		},
		{
			name:    "invalid characters",
			field:   UsernameField{Username: "user@name"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("UsernameField validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEmailFieldNormalize(t *testing.T) {
	field := &EmailField{Email: "  TEST@EXAMPLE.COM  "}
	field.Normalize()

	if field.Email != "test@example.com" {
		t.Errorf("EmailField.Normalize() = %q, want %q", field.Email, "test@example.com")
	}
}

func TestNameFieldNormalize(t *testing.T) {
	field := &NameField{Name: "  John Doe  "}
	field.Normalize()

	if field.Name != "John Doe" {
		t.Errorf("NameField.Normalize() = %q, want %q", field.Name, "John Doe")
	}
}

func TestURLFieldNormalize(t *testing.T) {
	field := &URLField{URL: "  https://example.com  "}
	field.Normalize()

	if field.URL != "https://example.com" {
		t.Errorf("URLField.Normalize() = %q, want %q", field.URL, "https://example.com")
	}
}

func TestDomainFieldNormalize(t *testing.T) {
	field := &DomainField{Domain: "  EXAMPLE.COM  "}
	field.Normalize()

	if field.Domain != "example.com" {
		t.Errorf("DomainField.Normalize() = %q, want %q", field.Domain, "example.com")
	}
}

func TestUsernameFieldNormalize(t *testing.T) {
	field := &UsernameField{Username: "  JohnDoe123  "}
	field.Normalize()

	if field.Username != "johndoe123" {
		t.Errorf("UsernameField.Normalize() = %q, want %q", field.Username, "johndoe123")
	}
}

// Test embedding in composite structs
type testRegisterDTO struct {
	EmailField
	NameField
	PasswordWithConfirmation
}

func TestEmbeddingInCompositeStruct(t *testing.T) {
	validate := validator.New()

	dto := testRegisterDTO{
		EmailField:               EmailField{Email: "test@example.com"},
		NameField:                NameField{Name: "John Doe"},
		PasswordWithConfirmation: PasswordWithConfirmation{Password: "securepassword123", PasswordConfirmation: "securepassword123"},
	}

	err := validate.Struct(dto)
	if err != nil {
		t.Errorf("Composite struct validation should pass, got error: %v", err)
	}
}

func TestEmbeddingWithInvalidFields(t *testing.T) {
	validate := validator.New()

	dto := testRegisterDTO{
		EmailField:               EmailField{Email: "invalid-email"},
		NameField:                NameField{Name: "J"},
		PasswordWithConfirmation: PasswordWithConfirmation{Password: "short", PasswordConfirmation: "different"},
	}

	err := validate.Struct(dto)
	if err == nil {
		t.Error("Composite struct validation should fail for invalid fields")
	}
}
