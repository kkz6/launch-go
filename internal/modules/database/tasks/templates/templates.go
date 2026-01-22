package templates

import "embed"

//go:embed mysql/*.sh
//go:embed postgresql/*.sh
var FS embed.FS
