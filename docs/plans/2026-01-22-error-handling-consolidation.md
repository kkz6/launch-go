# Error Handling Consolidation Plan

Simplify error handling by removing module-specific errors and using Fiber's built-in error pattern with a global error handler.

## Goals

- Remove all specific NotFound errors (ErrServerNotFound, ErrSiteNotFound, etc.)
- Use `fiber.NewError()` for all HTTP errors
- Global error handler converts errors to JSON responses
- Handlers just `return err` - no manual response building
- Generic messages by default, custom messages when needed

## Implementation Steps

### Step 1: Create Fiber Error Package

Create `internal/pkg/fiber/errors.go`:

```go
package fiberpkg

import (
    "errors"
    "github.com/gofiber/fiber/v2"
)

// Default messages
const (
    MsgNotFound     = "Resource not found"
    MsgUnauthorized = "Unauthorized"
    MsgForbidden    = "Access denied"
    MsgBadRequest   = "Bad request"
    MsgConflict     = "Resource conflict"
    MsgInternal     = "Internal server error"
    MsgValidation   = "Validation failed"
)

// Sentinel errors for errors.Is() checks
var (
    ErrNotFound   = errors.New("not found")
    ErrValidation = errors.New("validation error")
)

// Error helpers with optional custom message

func NotFound(message ...string) error {
    return fiber.NewError(fiber.StatusNotFound, msgOrDefault(message, MsgNotFound))
}

func Unauthorized(message ...string) error {
    return fiber.NewError(fiber.StatusUnauthorized, msgOrDefault(message, MsgUnauthorized))
}

func Forbidden(message ...string) error {
    return fiber.NewError(fiber.StatusForbidden, msgOrDefault(message, MsgForbidden))
}

func BadRequest(message ...string) error {
    return fiber.NewError(fiber.StatusBadRequest, msgOrDefault(message, MsgBadRequest))
}

func Conflict(message ...string) error {
    return fiber.NewError(fiber.StatusConflict, msgOrDefault(message, MsgConflict))
}

func Internal(message ...string) error {
    return fiber.NewError(fiber.StatusInternalServerError, msgOrDefault(message, MsgInternal))
}

func Validation(message ...string) error {
    return fiber.NewError(fiber.StatusUnprocessableEntity, msgOrDefault(message, MsgValidation))
}

func msgOrDefault(message []string, defaultMsg string) string {
    if len(message) > 0 && message[0] != "" {
        return message[0]
    }
    return defaultMsg
}

// IsNotFound checks if error is a not found condition
func IsNotFound(err error) bool {
    if errors.Is(err, ErrNotFound) {
        return true
    }
    var e *fiber.Error
    if errors.As(err, &e) && e.Code == fiber.StatusNotFound {
        return true
    }
    return false
}
```

### Step 2: Create Error Handler

Create `internal/pkg/fiber/error_handler.go`:

```go
package fiberpkg

import (
    "errors"
    "github.com/gofiber/fiber/v2"
)

type ErrorResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

func NewErrorHandler() fiber.ErrorHandler {
    return func(c *fiber.Ctx, err error) error {
        code := fiber.StatusInternalServerError
        message := MsgInternal

        var e *fiber.Error
        if errors.As(err, &e) {
            code = e.Code
            message = e.Message
        }

        return c.Status(code).JSON(ErrorResponse{
            Success: false,
            Message: message,
        })
    }
}
```

### Step 3: Create README

Create `internal/pkg/fiber/README.md` with usage examples.

### Step 4: Update Base Repository

Update `internal/pkg/repository/base.go`:

- Import `fiberpkg`
- Update `FindByIDOrFail` to return `fiberpkg.NotFound()`
- Update `FindByIDAndTeamOrFail` to return `fiberpkg.NotFound()`
- Update `FindByIDAndServerOrFail` to return `fiberpkg.NotFound()`
- Update `FirstOrFail` to return `fiberpkg.NotFound()`
- Return `fiberpkg.Internal()` for non-NotFound errors

### Step 5: Delete Error Package

Delete entire `internal/pkg/errors/` directory:
- `common.go`
- `types.go`
- `wrap.go`

### Step 6: Delete Repository Errors

Delete `internal/pkg/repository/errors.go`

### Step 7: Update Response Package

Update `internal/pkg/response/`:
- Remove `messages.go` (error messages now in fiberpkg)
- Update `errors.go` - remove `HandleError`, `HandleErrorOrInternalErr` (no longer needed)
- Keep success response helpers (`OK`, `Created`, etc.)

### Step 8: Update Module Repositories

For each module, update repositories to:
- Remove error variable declarations
- Use base repository `OrFail` methods or `fiberpkg` directly

Modules to update:
- `internal/modules/server/repositories/`
- `internal/modules/site/repositories/`
- `internal/modules/database/repositories/`
- `internal/modules/auth/repositories/`
- `internal/modules/backup/repositories/`
- `internal/modules/dns/repositories/`
- `internal/modules/git/repositories/`
- `internal/modules/notification/repositories/`
- `internal/modules/billing/repositories/`
- `internal/modules/script/repositories/`

### Step 9: Update Module Services

For each module, update services to:
- Remove error re-exports from `base.go`
- Use `fiberpkg` for custom errors
- Let repository errors bubble up

### Step 10: Update Handlers

For each module, update handlers to:
- Remove specific error checking
- Just `return err` after service calls
- ErrorHandler handles response formatting

### Step 11: Configure Error Handler

Update `cmd/api/main.go` (or wherever Fiber is initialized):

```go
app := fiber.New(fiber.Config{
    ErrorHandler: fiberpkg.NewErrorHandler(),
})
```

### Step 12: Update Imports

Search and replace across codebase:
- Remove imports of `internal/pkg/errors`
- Add imports of `internal/pkg/fiber` where needed

## Files to Create

- `internal/pkg/fiber/errors.go`
- `internal/pkg/fiber/error_handler.go`
- `internal/pkg/fiber/README.md`

## Files to Delete

- `internal/pkg/errors/common.go`
- `internal/pkg/errors/types.go`
- `internal/pkg/errors/wrap.go`
- `internal/pkg/repository/errors.go`
- `internal/pkg/response/messages.go`

## Files to Update

- `internal/pkg/repository/base.go`
- `internal/pkg/response/errors.go`
- `cmd/api/main.go`
- All module repositories
- All module services
- All module handlers

## Testing

After implementation:
1. Run `go build ./...` to check compilation
2. Run existing tests
3. Test API endpoints return correct error format:
   ```json
   {"success": false, "message": "Resource not found"}
   ```
