package models

import (
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnUser adapts a User model for the go-webauthn library
type WebAuthnUser struct {
	user        *User
	credentials []webauthn.Credential
}

// NewWebAuthnUser creates a WebAuthnUser from a User and their passkeys
func NewWebAuthnUser(user *User, passkeys []Passkey) *WebAuthnUser {
	creds := make([]webauthn.Credential, len(passkeys))
	for i := range passkeys {
		creds[i] = passkeys[i].ToWebAuthnCredential()
	}

	return &WebAuthnUser{
		user:        user,
		credentials: creds,
	}
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.user.ID)
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.user.Email
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.user.Name
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}
