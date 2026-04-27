package templates

import "embed"

//go:embed dockerservice/*.sh
var FS embed.FS
