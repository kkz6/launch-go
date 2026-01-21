package services

import (
	"github.com/kkz6/launch-go/internal/pkg/cryptoutil"
)

// verifyPassword verifies a password against a hash
func verifyPassword(hashedPassword, password string) bool {
	return cryptoutil.VerifyPassword(hashedPassword, password)
}
