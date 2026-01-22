# Fiber Error Handling

Simple error handling for Fiber HTTP responses.

## Setup

```go
app := fiber.New(fiber.Config{
    ErrorHandler: fiberpkg.NewErrorHandler(),
})
```

## Usage

### Return default errors

```go
return fiberpkg.NotFound()      // 404 "Resource not found"
return fiberpkg.BadRequest()    // 400 "Bad request"
return fiberpkg.Unauthorized()  // 401 "Unauthorized"
return fiberpkg.Forbidden()     // 403 "Access denied"
return fiberpkg.Conflict()      // 409 "Resource conflict"
return fiberpkg.Validation()    // 422 "Validation failed"
return fiberpkg.Internal()      // 500 "Internal server error"
```

### Return custom message

```go
return fiberpkg.NotFound("Server not found")
return fiberpkg.BadRequest("Invalid server status")
```

### In handlers

```go
func (h *Handler) Show(c *fiber.Ctx) error {
    server, err := h.service.FindByIDOrFail(ctx, id)
    if err != nil {
        return err
    }
    return response.OK(c, "Server retrieved", server)
}
```

### In services (custom validation)

```go
func (s *Service) Deploy(ctx context.Context, serverID string) error {
    server, err := s.repo.FindByIDOrFail(ctx, serverID)
    if err != nil {
        return err
    }
    
    if server.Status != "active" {
        return fiberpkg.BadRequest("Server must be active to deploy")
    }
    
    return nil
}
```

### Check error type

```go
if fiberpkg.IsNotFound(err) {
    // handle not found
}
```
