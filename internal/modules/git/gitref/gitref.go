package gitref

import "strings"

// Git ref prefixes
const (
	RefHeadsPrefix = "refs/heads/"
	RefTagsPrefix  = "refs/tags/"
)

// ExtractBranchName extracts branch name from git ref
func ExtractBranchName(ref string) string {
	return strings.TrimPrefix(ref, RefHeadsPrefix)
}

// ExtractTagName extracts tag name from git ref
func ExtractTagName(ref string) string {
	return strings.TrimPrefix(ref, RefTagsPrefix)
}

// IsBranchRef checks if ref is a branch reference
func IsBranchRef(ref string) bool {
	return strings.HasPrefix(ref, RefHeadsPrefix)
}

// IsTagRef checks if ref is a tag reference
func IsTagRef(ref string) bool {
	return strings.HasPrefix(ref, RefTagsPrefix)
}
