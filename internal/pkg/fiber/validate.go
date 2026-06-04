package fiber

import "github.com/gofiber/fiber/v2"

// Validate adapts a handler that needs a parsed-and-validated request body,
// injecting the typed DTO so the handler never repeats the
//
//	req, err := MustParseAndValidate[T](c)
//	if err != nil { return err }
//
// boilerplate. Parsing, Normalize() (if the DTO implements dto.Normalizable),
// and validation run before the handler; on failure the request is rejected and
// the handler is never called. The handler keeps full *fiber.Ctx access for
// team/user/params and custom responses.
//
// Usage:
//
//	func (h *Handler) Create(c *fiber.Ctx, req *dto.CreateThingRequest) error { ... }
//	group.Post("/things", fiberutil.Validate(h.Create))
func Validate[T any](fn func(c *fiber.Ctx, req *T) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		req, err := MustParseAndValidate[T](c)
		if err != nil {
			return err
		}
		return fn(c, req)
	}
}
