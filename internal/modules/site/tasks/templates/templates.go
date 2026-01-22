package templates

import "embed"

//go:embed *.sh
//go:embed deployment/*.sh
//go:embed deployment/prepare_fresh_installation/*.sh
var FS embed.FS
