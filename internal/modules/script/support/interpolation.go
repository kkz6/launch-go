package support

import (
	"strings"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// InterpolateVariables replaces template variables in script content with server values.
// Supported variables:
//   - {{server_id}} - The server's ULID
//   - {{server_name}} - The server's display name
//   - {{ip_address}} - The server's public IPv4
//   - {{private_ip_address}} - The server's private IPv4
//   - {{username}} - The SSH username
//   - {{db_password}} - The database password
//   - {{server_type}} - The server type (e.g., "web", "database")
func InterpolateVariables(content string, server *servermodels.Server) string {
	replacements := map[string]string{
		"{{server_id}}":          server.ID,
		"{{server_name}}":        server.Name,
		"{{ip_address}}":         getStringOrEmpty(server.PublicIPv4),
		"{{private_ip_address}}": getStringOrEmpty(server.PrivateIPv4),
		"{{username}}":           getStringOrEmpty(server.Username),
		"{{db_password}}":        server.DatabasePassword.String(),
		"{{server_type}}":        getStringOrEmpty(server.Type),
	}

	result := content
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

func getStringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
