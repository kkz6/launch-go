package fiber

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// Router wraps a fiber.Router and auto-adapts launch handler shapes so routes
// don't repeat fiberutil.Handler(...) for the common cases. Accepted handlers:
//
//   - func(*fiber.Ctx) error — a plain fiber handler, used as-is
//   - func(*Request) error   — an authenticated no-body handler, auto-wrapped
//   - fiberutil.Bind(...) / fiberutil.Validate(...) results — body handlers
//
// Body handlers (func(*Request, *T) error) are generic over T and cannot be
// auto-detected, so wrap them explicitly with Bind/Validate. The handler is
// passed first and middleware after, but the router registers middleware BEFORE
// the handler so authorization (e.g. middleware.Can) still runs first.
type Router struct {
	r fiber.Router
}

// Wrap returns a Router around an existing fiber.Router (app or group).
func Wrap(r fiber.Router) *Router { return &Router{r: r} }

// Raw returns the underlying fiber.Router for cases the helpers don't cover.
func (rt *Router) Raw() fiber.Router { return rt.r }

// Group creates a sub-group with optional group-level middleware.
func (rt *Router) Group(prefix string, mw ...fiber.Handler) *Router {
	return &Router{r: rt.r.Group(prefix, mw...)}
}

// Get registers a GET route. handler is auto-adapted; mw runs before it.
func (rt *Router) Get(path string, handler any, mw ...fiber.Handler) *Router {
	rt.r.Get(path, append(mw, adapt(handler))...)
	return rt
}

// Post registers a POST route.
func (rt *Router) Post(path string, handler any, mw ...fiber.Handler) *Router {
	rt.r.Post(path, append(mw, adapt(handler))...)
	return rt
}

// Put registers a PUT route.
func (rt *Router) Put(path string, handler any, mw ...fiber.Handler) *Router {
	rt.r.Put(path, append(mw, adapt(handler))...)
	return rt
}

// Patch registers a PATCH route.
func (rt *Router) Patch(path string, handler any, mw ...fiber.Handler) *Router {
	rt.r.Patch(path, append(mw, adapt(handler))...)
	return rt
}

// Delete registers a DELETE route.
func (rt *Router) Delete(path string, handler any, mw ...fiber.Handler) *Router {
	rt.r.Delete(path, append(mw, adapt(handler))...)
	return rt
}

// adapt converts a supported handler shape into a fiber.Handler. fiber.Handler
// is an alias for func(*fiber.Ctx) error, so Bind/Validate results and plain
// handlers both match the first case.
func adapt(h any) fiber.Handler {
	switch fn := h.(type) {
	case func(*fiber.Ctx) error:
		return fn
	case func(*Request) error:
		return Handler(fn)
	default:
		panic(fmt.Sprintf(
			"fiberutil.Router: unsupported handler type %T — body handlers must be wrapped with Bind or Validate",
			h,
		))
	}
}
