package utils

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

func NewULID() string {
	return strings.ToLower(ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String())
}

func ParseULID(s string) (ulid.ULID, error) {
	return ulid.Parse(s)
}

func IsValidULID(s string) bool {
	_, err := ulid.Parse(s)
	return err == nil
}
