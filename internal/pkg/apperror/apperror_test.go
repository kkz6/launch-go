package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	t.Run("without wrapped error", func(t *testing.T) {
		got := ErrNotFound.Error()
		want := "not_found: Resource not found"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("with wrapped error", func(t *testing.T) {
		cause := errors.New("db connection refused")
		got := ErrInternal.Wrap(cause).Error()
		want := "internal_error: Internal server error: db connection refused"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("underlying")
	wrapped := ErrConflict.Wrap(cause)

	if !errors.Is(wrapped, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

func TestAppError_Is(t *testing.T) {
	t.Run("matches predefined sentinel by code", func(t *testing.T) {
		err := ErrNotFound.WithMessage("user does not exist")
		if !errors.Is(err, ErrNotFound) {
			t.Error("errors.Is should match by Code regardless of message")
		}
	})

	t.Run("does not match different code", func(t *testing.T) {
		if errors.Is(ErrNotFound, ErrConflict) {
			t.Error("errors.Is should not match different codes")
		}
	})

	t.Run("matches through fmt.Errorf wrapping", func(t *testing.T) {
		err := fmt.Errorf("looking up user: %w", ErrNotFound.Wrap(errors.New("sql: no rows")))
		if !errors.Is(err, ErrNotFound) {
			t.Error("errors.Is should traverse fmt.Errorf wrapping")
		}
	})
}

func TestAppError_BuildersDoNotMutateSentinels(t *testing.T) {
	originalCode := ErrConflict.Code
	originalMessage := ErrConflict.Message
	originalStatus := ErrConflict.HTTPStatus

	derived := ErrConflict.
		WithMessage("email already taken").
		WithCode("auth.email_taken").
		Wrap(errors.New("unique violation"))

	if ErrConflict.Code != originalCode {
		t.Errorf("sentinel Code mutated: got %q, want %q", ErrConflict.Code, originalCode)
	}
	if ErrConflict.Message != originalMessage {
		t.Errorf("sentinel Message mutated: got %q, want %q", ErrConflict.Message, originalMessage)
	}
	if ErrConflict.HTTPStatus != originalStatus {
		t.Errorf("sentinel HTTPStatus mutated: got %d, want %d", ErrConflict.HTTPStatus, originalStatus)
	}
	if ErrConflict.Err != nil {
		t.Errorf("sentinel Err mutated: got %v, want nil", ErrConflict.Err)
	}

	// Derived value should reflect all changes.
	if derived.Code != "auth.email_taken" {
		t.Errorf("derived Code = %q, want %q", derived.Code, "auth.email_taken")
	}
	if derived.Message != "email already taken" {
		t.Errorf("derived Message = %q, want %q", derived.Message, "email already taken")
	}
	if derived.HTTPStatus != http.StatusConflict {
		t.Errorf("derived HTTPStatus = %d, want %d", derived.HTTPStatus, http.StatusConflict)
	}
	if derived.Err == nil {
		t.Error("derived Err should be set")
	}
}

func TestAs(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := As(nil); got != nil {
			t.Errorf("As(nil) = %v, want nil", got)
		}
	})

	t.Run("non-AppError returns nil", func(t *testing.T) {
		if got := As(errors.New("plain error")); got != nil {
			t.Errorf("As(plain) = %v, want nil", got)
		}
	})

	t.Run("AppError returns it", func(t *testing.T) {
		want := ErrForbidden.WithMessage("team owner only")
		got := As(want)
		if got == nil {
			t.Fatal("As returned nil for AppError")
		}
		if got.Code != want.Code {
			t.Errorf("Code = %q, want %q", got.Code, want.Code)
		}
	})

	t.Run("AppError through fmt.Errorf wrapping", func(t *testing.T) {
		original := ErrForbidden.WithMessage("team owner only")
		wrapped := fmt.Errorf("authorization check: %w", original)
		got := As(wrapped)
		if got == nil {
			t.Fatal("As returned nil for wrapped AppError")
		}
		if got.Code != CodeForbidden {
			t.Errorf("Code = %q, want %q", got.Code, CodeForbidden)
		}
	})
}
