package templates

import "embed"

//go:embed provision/*.sh
//go:embed software/*.sh
var FS embed.FS
