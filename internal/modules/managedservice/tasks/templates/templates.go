package templates

import "embed"

//go:embed managedservice/*.sh
var FS embed.FS
