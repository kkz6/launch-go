package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/dns/services"
)

// Sentinel errors carry their final HTTP status and user-facing message
// so the global error handler can render them without per-handler
// branching. Keep these assertions in sync with the messages the
// frontend expects on the wire.
func TestServiceErrors(t *testing.T) {
	t.Run("ErrRecordNotEditable", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotEditable, "This record type cannot be edited")
	})

	t.Run("ErrRecordNotDeletable", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotDeletable, "This record type cannot be deleted")
	})

	t.Run("ErrProviderHasActiveDomains", func(t *testing.T) {
		assert.EqualError(t, services.ErrProviderHasActiveDomains, "Cannot delete provider with active domains")
	})

	t.Run("ErrInvalidCredentials", func(t *testing.T) {
		assert.EqualError(t, services.ErrInvalidCredentials, "Invalid credentials")
	})
}
