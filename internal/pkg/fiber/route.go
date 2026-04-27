package fiber

import (
	"context"

	gofiber "github.com/gofiber/fiber/v2"
)

// Route helpers collapse team-scoped CRUD handler boilerplate into one
// line per route. Each helper extracts request context, calls the service
// function, and either serializes the response or returns the error for
// the global error handler to render.
//
// Service signatures (read = no userID, mutation = always userID):
//
//	Index:        func(ctx, teamID) (Resp, error)
//	Show:         func(ctx, id, teamID) (Resp, error)
//	Create:       func(ctx, teamID, userID, req) (Resp, error)
//	Update:       func(ctx, id, teamID, userID, req) (Resp, error)
//	Delete:       func(ctx, id, teamID, userID) error
//	Action:       func(ctx, id, teamID, userID) error                 -- /:id/sync
//
//	IndexNested:  func(ctx, parentID, teamID) (Resp, error)
//	ShowNested:   func(ctx, id, parentID, teamID) (Resp, error)
//	CreateNested: func(ctx, parentID, teamID, userID, req) (Resp, error)
//	UpdateNested: func(ctx, id, parentID, teamID, userID, req) (Resp, error)
//	DeleteNested: func(ctx, id, parentID, teamID, userID) error
//	ActionNested: func(ctx, parentID, teamID, userID) error           -- /:pid/items/sync
//
// Mutation helpers always thread userID so services have it available
// for activity logging without per-handler plumbing. Services that don't
// need it can ignore the parameter.

// Service-call signatures.

// IndexFunc is the service signature expected by Index.
type IndexFunc[Resp any] func(ctx context.Context, teamID string) (Resp, error)

// ShowFunc is the service signature expected by Show.
type ShowFunc[Resp any] func(ctx context.Context, id, teamID string) (Resp, error)

// CreateFunc is the service signature expected by Create.
type CreateFunc[Req, Resp any] func(ctx context.Context, teamID, userID string, req *Req) (Resp, error)

// UpdateFunc is the service signature expected by Update.
type UpdateFunc[Req, Resp any] func(ctx context.Context, id, teamID, userID string, req *Req) (Resp, error)

// DeleteFunc is the service signature expected by Delete.
type DeleteFunc func(ctx context.Context, id, teamID, userID string) error

// ActionFunc is the service signature expected by Action.
type ActionFunc func(ctx context.Context, id, teamID, userID string) error

// IndexNestedFunc is the service signature expected by IndexNested.
type IndexNestedFunc[Resp any] func(ctx context.Context, parentID, teamID string) (Resp, error)

// ShowNestedFunc is the service signature expected by ShowNested.
type ShowNestedFunc[Resp any] func(ctx context.Context, id, parentID, teamID string) (Resp, error)

// CreateNestedFunc is the service signature expected by CreateNested.
type CreateNestedFunc[Req, Resp any] func(ctx context.Context, parentID, teamID, userID string, req *Req) (Resp, error)

// UpdateNestedFunc is the service signature expected by UpdateNested.
type UpdateNestedFunc[Req, Resp any] func(ctx context.Context, id, parentID, teamID, userID string, req *Req) (Resp, error)

// DeleteNestedFunc is the service signature expected by DeleteNested.
type DeleteNestedFunc func(ctx context.Context, id, parentID, teamID, userID string) error

// ActionNestedFunc is the service signature expected by ActionNested.
type ActionNestedFunc func(ctx context.Context, parentID, teamID, userID string) error

// ActionItemNestedFunc is the service signature expected by ActionItemNested.
type ActionItemNestedFunc func(ctx context.Context, id, parentID, teamID, userID string) error

// Index handles team-scoped list endpoints.
func Index[Resp any](successMsg string, fn IndexFunc[Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, err := MustGetTeamID(c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), teamID)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// Show handles team-scoped fetch-by-id endpoints. The id comes from :id.
func Show[Resp any](successMsg string, fn ShowFunc[Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, err := MustGetTeamID(c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params("id"), teamID)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// Create handles team-scoped create endpoints. The body is parsed and
// validated; on success the response is rendered with status 201.
func Create[Req, Resp any](successMsg string, fn CreateFunc[Req, Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[Req](c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), teamID, userID, req)
		if err != nil {
			return err
		}
		return Created(c, successMsg, result)
	}
}

// Update handles team-scoped update endpoints. The id comes from :id and
// the body is parsed/validated. Renders 200 on success.
func Update[Req, Resp any](successMsg string, fn UpdateFunc[Req, Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[Req](c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params("id"), teamID, userID, req)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// Delete handles team-scoped delete endpoints. Returns 204 on success.
func Delete(fn DeleteFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params("id"), teamID, userID); err != nil {
			return err
		}
		return NoContent(c)
	}
}

// Action handles team-scoped side-effect endpoints (e.g. /sync, /check)
// that take an id but no body and return no payload. Renders 200 with the
// supplied success message.
func Action(successMsg string, fn ActionFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params("id"), teamID, userID); err != nil {
			return err
		}
		return OK(c, successMsg, nil)
	}
}

// IndexNested handles team-scoped list endpoints under a parent resource.
// parentParam is the path parameter holding the parent id (e.g. "id" for
// /domains/:id/records, or "serverId" for /servers/:serverId/databases).
func IndexNested[Resp any](parentParam, successMsg string, fn IndexNestedFunc[Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, err := MustGetTeamID(c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(parentParam), teamID)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// ShowNested handles team-scoped fetch-by-id endpoints under a parent.
// e.g. GET /servers/:serverId/databases/:id.
func ShowNested[Resp any](parentParam, idParam, successMsg string, fn ShowNestedFunc[Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, err := MustGetTeamID(c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), teamID)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// CreateNested handles team-scoped create endpoints under a parent. The
// body is parsed and validated; renders 201 on success.
func CreateNested[Req, Resp any](parentParam, successMsg string, fn CreateNestedFunc[Req, Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[Req](c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(parentParam), teamID, userID, req)
		if err != nil {
			return err
		}
		return Created(c, successMsg, result)
	}
}

// UpdateNested handles team-scoped update endpoints with a parent and
// own id, e.g. PATCH /domains/:domainId/records/:recordId.
func UpdateNested[Req, Resp any](parentParam, idParam, successMsg string, fn UpdateNestedFunc[Req, Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[Req](c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), teamID, userID, req)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// DeleteNested handles team-scoped delete endpoints with a parent and
// own id. Returns 204 on success.
func DeleteNested(parentParam, idParam string, fn DeleteNestedFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), teamID, userID); err != nil {
			return err
		}
		return NoContent(c)
	}
}

// ActionNested handles parent-scoped side-effect endpoints with no own id,
// e.g. POST /servers/:serverId/databases/sync. Renders 200 with the
// supplied success message.
func ActionNested(parentParam, successMsg string, fn ActionNestedFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params(parentParam), teamID, userID); err != nil {
			return err
		}
		return OK(c, successMsg, nil)
	}
}

// ActionItemNested handles per-item side-effect endpoints under a parent,
// e.g. POST /servers/:serverId/backups/:id/run. Renders 200 with the
// supplied success message.
func ActionItemNested(parentParam, idParam, successMsg string, fn ActionItemNestedFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), teamID, userID); err != nil {
			return err
		}
		return OK(c, successMsg, nil)
	}
}
