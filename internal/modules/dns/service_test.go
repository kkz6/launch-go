package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/dns/services"
)

func TestServiceErrors(t *testing.T) {
	t.Run("ErrProviderNotFound", func(t *testing.T) {
		assert.EqualError(t, services.ErrProviderNotFound, "provider not found")
	})

	t.Run("ErrDomainNotFound", func(t *testing.T) {
		assert.EqualError(t, services.ErrDomainNotFound, "domain not found")
	})

	t.Run("ErrRecordNotFound", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotFound, "record not found")
	})

	t.Run("ErrRecordNotEditable", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotEditable, "record cannot be edited")
	})

	t.Run("ErrRecordNotDeletable", func(t *testing.T) {
		assert.EqualError(t, services.ErrRecordNotDeletable, "record cannot be deleted")
	})

	t.Run("ErrProviderHasActiveDomains", func(t *testing.T) {
		assert.EqualError(t, services.ErrProviderHasActiveDomains, "provider has active domains")
	})

	t.Run("ErrInvalidCredentials", func(t *testing.T) {
		assert.EqualError(t, services.ErrInvalidCredentials, "invalid credentials")
	})
}
