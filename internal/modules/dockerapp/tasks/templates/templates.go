package templates

import "embed"

//go:embed dockerapp/*.sh
var FS embed.FS
