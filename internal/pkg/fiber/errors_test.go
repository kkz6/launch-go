package fiber

import (
	"errors"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
)

func TestNotFound(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    int
	}{
		{
			name:    "with custom message",
			message: "User not found",
			want:    404,
		},
		{
			name:    "with default message",
			message: "",
			want:    404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.message != "" {
				err = NotFound(tt.message)
			} else {
				err = NotFound()
			}

			var fiberErr *gofiber.Error
			if !errors.As(err, &fiberErr) {
				t.Fatalf("expected *fiber.Error, got %T", err)
			}
			if fiberErr.Code != tt.want {
				t.Errorf("expected status %d, got %d", tt.want, fiberErr.Code)
			}
		})
	}
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("invalid input")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 400 {
		t.Errorf("expected status 400, got %d", fiberErr.Code)
	}
	if fiberErr.Message != "invalid input" {
		t.Errorf("expected message 'invalid input', got '%s'", fiberErr.Message)
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("resource already exists")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 409 {
		t.Errorf("expected status 409, got %d", fiberErr.Code)
	}
}

func TestForbidden(t *testing.T) {
	err := Forbidden("access denied")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 403 {
		t.Errorf("expected status 403, got %d", fiberErr.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("not authenticated")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 401 {
		t.Errorf("expected status 401, got %d", fiberErr.Code)
	}
}

func TestInternal(t *testing.T) {
	err := Internal("something went wrong")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 500 {
		t.Errorf("expected status 500, got %d", fiberErr.Code)
	}
}

func TestValidation(t *testing.T) {
	err := Validation("validation failed")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 422 {
		t.Errorf("expected status 422, got %d", fiberErr.Code)
	}
}

func TestTooManyRequests(t *testing.T) {
	err := TooManyRequests("rate limit exceeded")

	var fiberErr *gofiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("expected *fiber.Error, got %T", err)
	}
	if fiberErr.Code != 429 {
		t.Errorf("expected status 429, got %d", fiberErr.Code)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ErrNotFound sentinel",
			err:  ErrNotFound,
			want: true,
		},
		{
			name: "NotFound function error",
			err:  NotFound("User not found"),
			want: true,
		},
		{
			name: "BadRequest error",
			err:  BadRequest("invalid"),
			want: false,
		},
		{
			name: "regular error",
			err:  errors.New("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnauthorized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Unauthorized error",
			err:  Unauthorized("not authenticated"),
			want: true,
		},
		{
			name: "Forbidden error",
			err:  Forbidden("access denied"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnauthorized(tt.err); got != tt.want {
				t.Errorf("IsUnauthorized() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsForbidden(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Forbidden error",
			err:  Forbidden("access denied"),
			want: true,
		},
		{
			name: "Unauthorized error",
			err:  Unauthorized("not authenticated"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForbidden(tt.err); got != tt.want {
				t.Errorf("IsForbidden() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Conflict error",
			err:  Conflict("resource exists"),
			want: true,
		},
		{
			name: "BadRequest error",
			err:  BadRequest("invalid"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConflict(tt.err); got != tt.want {
				t.Errorf("IsConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ErrValidation sentinel",
			err:  ErrValidation,
			want: true,
		},
		{
			name: "Validation function error",
			err:  Validation("validation failed"),
			want: true,
		},
		{
			name: "BadRequest error",
			err:  BadRequest("invalid"),
			want: true,
		},
		{
			name: "NotFound error",
			err:  NotFound("not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidationError(tt.err); got != tt.want {
				t.Errorf("IsValidationError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInternalError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Internal error",
			err:  Internal("server error"),
			want: true,
		},
		{
			name: "BadRequest error",
			err:  BadRequest("invalid"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInternalError(tt.err); got != tt.want {
				t.Errorf("IsInternalError() = %v, want %v", got, tt.want)
			}
		})
	}
}
