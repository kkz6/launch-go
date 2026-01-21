package fiber

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/oklog/ulid/v2"
)

// GetID extracts and validates the "id" path parameter as a ULID.
// Returns a BadRequest error response if missing or invalid.
//
// Usage:
//
//	func (h *Handler) Show(c *fiber.Ctx) error {
//	    id, err := fiberctx.GetID(c)
//	    if err != nil {
//	        return err
//	    }
//	    // use id...
//	}
func GetID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "id")
}

// GetServerID extracts and validates the "serverId" path parameter as a ULID.
// Common for nested routes under /servers/:serverId
func GetServerID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "serverId")
}

// GetULIDParam extracts and validates a path parameter as a ULID.
// Returns a BadRequest error response if missing or invalid.
//
// Usage:
//
//	deploymentID, err := fiberctx.GetULIDParam(c, "deploymentId")
//	if err != nil {
//	    return err
//	}
func GetULIDParam(c *fiber.Ctx, name string) (string, error) {
	value := c.Params(name)
	if value == "" {
		return "", response.BadRequest(c, "Missing required parameter: "+name)
	}
	if _, err := ulid.Parse(value); err != nil {
		return "", response.BadRequest(c, "Invalid parameter format: "+name)
	}
	return value, nil
}

// GetParam extracts a required path parameter (non-ULID).
// Returns a BadRequest error response if missing.
//
// Usage:
//
//	filename, err := fiberctx.GetParam(c, "filename")
//	if err != nil {
//	    return err
//	}
func GetParam(c *fiber.Ctx) (string, error) {
	return GetRequiredParam(c, "id")
}

// GetRequiredParam extracts a required path parameter by name.
// Returns a BadRequest error response if missing.
func GetRequiredParam(c *fiber.Ctx, name string) (string, error) {
	value := c.Params(name)
	if value == "" {
		return "", response.BadRequest(c, "Missing required parameter: "+name)
	}
	return value, nil
}

// MustGetID extracts the "id" parameter, panicking if invalid.
// Use only when you're certain the middleware has validated the route.
func MustGetID(c *fiber.Ctx) string {
	id, err := GetID(c)
	if err != nil {
		panic(err)
	}
	return id
}

// MustGetServerID extracts the "serverId" parameter, panicking if invalid.
func MustGetServerID(c *fiber.Ctx) string {
	id, err := GetServerID(c)
	if err != nil {
		panic(err)
	}
	return id
}

// MustGetULIDParam extracts a ULID parameter, panicking if invalid.
func MustGetULIDParam(c *fiber.Ctx, name string) string {
	id, err := GetULIDParam(c, name)
	if err != nil {
		panic(err)
	}
	return id
}

// GetSiteID extracts and validates the "id" path parameter as a ULID for site routes.
// This is an alias for GetID, provided for semantic clarity in nested routes
// like /servers/:serverId/sites/:id
func GetSiteID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "id")
}

// GetDeploymentID extracts and validates the "deploymentId" path parameter as a ULID.
func GetDeploymentID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "deploymentId")
}

// GetCronID extracts and validates the "cronId" path parameter as a ULID.
func GetCronID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "cronId")
}

// GetDaemonID extracts and validates the "daemonId" path parameter as a ULID.
func GetDaemonID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "daemonId")
}

// GetKeyID extracts and validates the "keyId" path parameter as a ULID.
func GetKeyID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "keyId")
}

// GetRuleID extracts and validates the "ruleId" path parameter as a ULID.
func GetRuleID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "ruleId")
}

// GetRedirectID extracts and validates the "redirectId" path parameter as a ULID.
func GetRedirectID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "redirectId")
}

// GetRecordID extracts and validates the "recordId" path parameter as a ULID.
func GetRecordID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "recordId")
}

// GetTeamIDParam extracts and validates the "teamId" path parameter as a ULID.
// Note: This is different from GetTeamID in context.go which extracts from Locals.
func GetTeamIDParam(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "teamId")
}

// GetDomainID extracts and validates the "domainId" path parameter as a ULID.
func GetDomainID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "domainId")
}

// GetJobID extracts and validates the "jobId" path parameter as a ULID.
func GetJobID(c *fiber.Ctx) (string, error) {
	return GetULIDParam(c, "jobId")
}
