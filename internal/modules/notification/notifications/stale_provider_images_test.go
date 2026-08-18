package notifications

import (
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaleProviderImagesEmailUsesTimeline(t *testing.T) {
	templates.Initialize("Launch", "https://launch.io")
	notification := NewStaleProviderImagesNotification([]StaleProviderImageFinding{{
		Provider: "AWS",
		OS:       "Ubuntu 24.04",
		Image:    "ami-retired",
		Reason:   "image not found",
	}})

	message := notification.ToEmail()

	require.True(t, message.IsHTML)
	assert.Contains(t, message.Body, "lctl / provider")
	assert.Contains(t, message.Body, "CONFIGURATION STALE")
	assert.Contains(t, message.Body, "ami-retired")
}
