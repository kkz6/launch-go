package fiber

import "github.com/gofiber/fiber/v2"

// Request is the authenticated request context: the team and user that every
// authenticated route resolves, bundled with the fiber context. It saves
// handlers from repeating
//
//	teamID, userID, err := MustGetTeamAndUserID(c)
//	if err != nil { return err }
//
// It embeds *fiber.Ctx, so handlers keep full access to params, query, body,
// and responses (r.Params(...), r.Context(), r.JSON(...), etc.).
type Request struct {
	*fiber.Ctx
	TeamID string
	UserID string
}

// Handler adapts a handler that runs in an authenticated request context (no
// body), injecting a *Request. It must run after the Auth + TeamScope
// middleware; if the team or user is missing the request is rejected and the
// handler is never called.
//
// Usage:
//
//	func (h *H) Reboot(r *fiberutil.Request) error { ... r.TeamID ... }
//	group.Post("/:id/reboot", fiberutil.Handler(h.Reboot))
func Handler(fn func(r *Request) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		return fn(&Request{Ctx: c, TeamID: teamID, UserID: userID})
	}
}

// Bind adapts a handler that runs in an authenticated request context AND needs
// a parsed-and-validated request body, injecting both the *Request and the
// typed DTO. Team/user are resolved first, then the body is parsed, normalized,
// and validated; any failure rejects the request before the handler runs.
//
// For public routes with no auth context (login, register), use Validate
// instead, which injects only the validated body.
//
// Usage:
//
//	func (h *H) Create(r *fiberutil.Request, req *dto.CreateThing) error { ... }
//	group.Post("/", fiberutil.Bind(h.Create))
func Bind[T any](fn func(r *Request, req *T) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[T](c)
		if err != nil {
			return err
		}
		return fn(&Request{Ctx: c, TeamID: teamID, UserID: userID}, req)
	}
}
