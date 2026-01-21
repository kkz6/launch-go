package git

import "github.com/kkz6/launch-go/internal/modules/git/gitref"

// Re-export git ref helpers from the gitref subpackage

// Git ref prefixes
const (
	RefHeadsPrefix = gitref.RefHeadsPrefix
	RefTagsPrefix  = gitref.RefTagsPrefix
)

// ExtractBranchName extracts branch name from git ref
var ExtractBranchName = gitref.ExtractBranchName

// ExtractTagName extracts tag name from git ref
var ExtractTagName = gitref.ExtractTagName

// IsBranchRef checks if ref is a branch reference
var IsBranchRef = gitref.IsBranchRef

// IsTagRef checks if ref is a tag reference
var IsTagRef = gitref.IsTagRef
