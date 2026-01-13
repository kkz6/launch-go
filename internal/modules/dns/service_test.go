package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/dns/services"
)

func TestServiceErrors(t *testing.T) {
	t.Run("ErrProviderNotFound", func(t *testing.T) {
		assert.EqualError(t, services.ErrProviderNotFound, "Provider not found")
	})

	t.Run("ErrDomainNotFound", func(t *testing.T) {
		assert.EqualError(t, services.ErrDomainNotFound, "Domain not found")
	})

	t.Run("ErrRecordNotFound", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotFound, "Record not found")
	})

	t.Run("ErrRecordNotEditable", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotEditable, "Record cannot be edited")
	})

	t.Run("ErrRecordNotDeletable", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotDeletable, "Record cannot be deleted")
	})

	t.Run("ErrProviderHasActiveDomains", func(t *testing.T) {
		assert.EqualError(t, services.ErrProviderHasActiveDomains, "Provider has active domains")
	})

	t.Run("ErrInvalidCredentials", func(t *testing.T) {
		assert.EqualError(t, services.ErrInvalidCredentials, "Invalid credentials")
	})
}
