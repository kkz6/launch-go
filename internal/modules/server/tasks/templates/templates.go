package templates

import "embed"

//go:embed provision/*.sh
//go:embed software/*.sh
//go:embed lb/*.sh
var FS embed.FS
