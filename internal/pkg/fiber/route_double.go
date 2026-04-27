package fiber

import (
	"context"

	gofiber "github.com/gofiber/fiber/v2"
)

// Doubly-nested route helpers cover URL patterns with two levels of
// parent context, e.g.:
//
//	GET    /servers/:serverId/sites/:id/queues
//	POST   /servers/:serverId/sites/:id/queues
//	PATCH  /servers/:serverId/sites/:id/queues/:queueId
//	DELETE /servers/:serverId/sites/:id/queues/:queueId
//	POST   /servers/:serverId/sites/:id/queues/sync
//	POST   /servers/:serverId/sites/:id/queues/:queueId/restart
//
// In service signatures the leaf-most owner is the "parent"
// (siteID/upstreamID), and the outer scope is the "grandparent"
// (serverID). Service functions accept the ids in (id, parentID,
// grandparentID, teamID, userID, ...) order — closest-first — so they
// read like the URL.

// IndexDoubleNestedFunc is the service signature expected by IndexDoubleNested.
type IndexDoubleNestedFunc[Resp any] func(ctx context.Context, parentID, grandparentID, teamID string) (Resp, error)

// ShowDoubleNestedFunc is the service signature expected by ShowDoubleNested.
type ShowDoubleNestedFunc[Resp any] func(ctx context.Context, id, parentID, grandparentID, teamID string) (Resp, error)

// CreateDoubleNestedFunc is the service signature expected by CreateDoubleNested.
type CreateDoubleNestedFunc[Req, Resp any] func(ctx context.Context, parentID, grandparentID, teamID, userID string, req *Req) (Resp, error)

// UpdateDoubleNestedFunc is the service signature expected by UpdateDoubleNested.
type UpdateDoubleNestedFunc[Req, Resp any] func(ctx context.Context, id, parentID, grandparentID, teamID, userID string, req *Req) (Resp, error)

// DeleteDoubleNestedFunc is the service signature expected by DeleteDoubleNested.
type DeleteDoubleNestedFunc func(ctx context.Context, id, parentID, grandparentID, teamID, userID string) error

// ActionDoubleNestedFunc is the service signature expected by ActionDoubleNested.
type ActionDoubleNestedFunc func(ctx context.Context, parentID, grandparentID, teamID, userID string) error

// ActionItemDoubleNestedFunc is the service signature expected by ActionItemDoubleNested.
type ActionItemDoubleNestedFunc func(ctx context.Context, id, parentID, grandparentID, teamID, userID string) error

// IndexDoubleNested handles team-scoped list endpoints under a parent
// nested in a grandparent.
func IndexDoubleNested[Resp any](grandparentParam, parentParam, successMsg string, fn IndexDoubleNestedFunc[Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, err := MustGetTeamID(c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(parentParam), c.Params(grandparentParam), teamID)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// ShowDoubleNested handles team-scoped fetch-by-id under a parent nested
// in a grandparent.
func ShowDoubleNested[Resp any](grandparentParam, parentParam, idParam, successMsg string, fn ShowDoubleNestedFunc[Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, err := MustGetTeamID(c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), c.Params(grandparentParam), teamID)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// CreateDoubleNested handles team-scoped create endpoints under a
// doubly-nested parent. Returns 201.
func CreateDoubleNested[Req, Resp any](grandparentParam, parentParam, successMsg string, fn CreateDoubleNestedFunc[Req, Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[Req](c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(parentParam), c.Params(grandparentParam), teamID, userID, req)
		if err != nil {
			return err
		}
		return Created(c, successMsg, result)
	}
}

// UpdateDoubleNested handles team-scoped update endpoints under a
// doubly-nested parent. Returns 200.
func UpdateDoubleNested[Req, Resp any](grandparentParam, parentParam, idParam, successMsg string, fn UpdateDoubleNestedFunc[Req, Resp]) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		req, err := MustParseAndValidate[Req](c)
		if err != nil {
			return err
		}
		result, err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), c.Params(grandparentParam), teamID, userID, req)
		if err != nil {
			return err
		}
		return OK(c, successMsg, result)
	}
}

// DeleteDoubleNested handles team-scoped delete endpoints under a
// doubly-nested parent. Returns 204.
func DeleteDoubleNested(grandparentParam, parentParam, idParam string, fn DeleteDoubleNestedFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), c.Params(grandparentParam), teamID, userID); err != nil {
			return err
		}
		return NoContent(c)
	}
}

// ActionDoubleNested handles parent-collection actions under a
// doubly-nested parent (e.g. POST /servers/:sid/sites/:id/queues/sync).
func ActionDoubleNested(grandparentParam, parentParam, successMsg string, fn ActionDoubleNestedFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params(parentParam), c.Params(grandparentParam), teamID, userID); err != nil {
			return err
		}
		return OK(c, successMsg, nil)
	}
}

// ActionItemDoubleNested handles per-item side-effect endpoints under a
// doubly-nested parent (e.g. POST /servers/:sid/sites/:id/queues/:qid/restart).
func ActionItemDoubleNested(grandparentParam, parentParam, idParam, successMsg string, fn ActionItemDoubleNestedFunc) gofiber.Handler {
	return func(c *gofiber.Ctx) error {
		teamID, userID, err := MustGetTeamAndUserID(c)
		if err != nil {
			return err
		}
		if err := fn(c.Context(), c.Params(idParam), c.Params(parentParam), c.Params(grandparentParam), teamID, userID); err != nil {
			return err
		}
		return OK(c, successMsg, nil)
	}
}
