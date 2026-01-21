package services

import (
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// verifyPassword verifies a password against a hash
func verifyPassword(hashedPassword, password string) bool {
	return security.VerifyPassword(hashedPassword, password)
}
