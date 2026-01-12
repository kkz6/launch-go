package site

import "embed"

// TemplateFS embeds all shell template files from the tasks/templates directory
// This provides backward compatibility for code expecting site.TemplateFS
//
//go:embed tasks/templates/*.sh tasks/templates/deployment/*.sh
var TemplateFS embed.FS
