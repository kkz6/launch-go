package templates

import "embed"

//go:embed docker/*.sh
var FS embed.FS
