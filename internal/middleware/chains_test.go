package middleware

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// mockHandler creates a simple handler for testing
func mockHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

func TestChain(t *testing.T) {
	h1 := mockHandler()
	h2 := mockHandler()
	h3 := mockHandler()

	chain := Chain(h1, h2, h3)

	assert.Len(t, chain, 3)
}

func TestChainEmpty(t *testing.T) {
	chain := Chain()
	assert.Len(t, chain, 0)
}

func TestAuthenticatedChain(t *testing.T) {
	authMiddleware := mockHandler()
	chain := AuthenticatedChain(authMiddleware)

	// Should have auth, team scope, and subscription verification
	assert.Len(t, chain, 3)
}

func TestAuthenticatedChainWithRole(t *testing.T) {
	authMiddleware := mockHandler()
	chain := AuthenticatedChainWithRole(authMiddleware, "admin")

	// Should have auth, team scope, subscription verification, and role check
	assert.Len(t, chain, 4)
}

func TestTeamScopeChain(t *testing.T) {
	authMiddleware := mockHandler()
	chain := TeamScopeChain(authMiddleware)

	// Should have auth and team scope only (no subscription)
	assert.Len(t, chain, 2)
}

func TestOptionalAuthChain(t *testing.T) {
	chain := OptionalAuthChain("test-secret")

	// Should have optional auth and optional team scope
	assert.Len(t, chain, 2)
}

func TestWebhookChain(t *testing.T) {
	chain := WebhookChain()

	// Should have signed URL verification
	assert.Len(t, chain, 1)
}

func TestWebhookChainWithSigner(t *testing.T) {
	chain := WebhookChainWithSigner(nil)

	// Should have signed URL verification with custom signer
	assert.Len(t, chain, 1)
}

func TestAPIChain(t *testing.T) {
	chain := APIChain(100, 60)

	// Should have rate limiting
	assert.Len(t, chain, 1)
}

func TestAuthenticatedAPIChain(t *testing.T) {
	authMiddleware := mockHandler()
	chain := AuthenticatedAPIChain(authMiddleware, 100, 60)

	// Should have auth, team scope, subscription, and rate limiting
	assert.Len(t, chain, 4)
}

func TestAppend(t *testing.T) {
	h1 := mockHandler()
	h2 := mockHandler()
	h3 := mockHandler()

	baseChain := []fiber.Handler{h1, h2}
	extended := Append(baseChain, h3)

	assert.Len(t, extended, 3)
	// Original chain should be unchanged
	assert.Len(t, baseChain, 2)
}

func TestAppendMultiple(t *testing.T) {
	h1 := mockHandler()
	h2 := mockHandler()
	h3 := mockHandler()
	h4 := mockHandler()

	baseChain := []fiber.Handler{h1}
	extended := Append(baseChain, h2, h3, h4)

	assert.Len(t, extended, 4)
}

func TestPrepend(t *testing.T) {
	h1 := mockHandler()
	h2 := mockHandler()
	h3 := mockHandler()

	baseChain := []fiber.Handler{h2, h3}
	extended := Prepend(baseChain, h1)

	assert.Len(t, extended, 3)
	// Original chain should be unchanged
	assert.Len(t, baseChain, 2)
}

func TestPrependMultiple(t *testing.T) {
	h1 := mockHandler()
	h2 := mockHandler()
	h3 := mockHandler()
	h4 := mockHandler()

	baseChain := []fiber.Handler{h4}
	extended := Prepend(baseChain, h1, h2, h3)

	assert.Len(t, extended, 4)
}

func TestChainImmutability(t *testing.T) {
	h1 := mockHandler()
	h2 := mockHandler()
	h3 := mockHandler()

	original := []fiber.Handler{h1, h2}
	appended := Append(original, h3)
	prepended := Prepend(original, h3)

	// Original should remain unchanged
	assert.Len(t, original, 2)
	assert.Len(t, appended, 3)
	assert.Len(t, prepended, 3)
}
