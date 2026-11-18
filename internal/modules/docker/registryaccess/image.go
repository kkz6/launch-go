package registryaccess

import (
	"errors"
	"regexp"
	"strings"
)

var imageSuffix = regexp.MustCompile(`^(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})?$`)

func ValidateImage(image, packageName string) error {
	if !strings.HasPrefix(packageName, "ghcr.io/") || !strings.HasPrefix(image, packageName) || !imageSuffix.MatchString(strings.TrimPrefix(image, packageName)) {
		return errors.New("image is not in the configured repository")
	}
	return nil
}
