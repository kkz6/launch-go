package server

import "embed"

// TemplateFS embeds all shell template files from the tasks/templates directory
// This provides backward compatibility for code expecting server.TemplateFS
//
//go:embed tasks/templates/provision/*.sh tasks/templates/software/*.sh
var TemplateFS embed.FS
