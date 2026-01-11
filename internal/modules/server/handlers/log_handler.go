package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// LogInfo represents available log information
type LogInfo struct {
	Name      string `json:"name"`
	Software  string `json:"software"`
	ShowRoute string `json:"show_route"`
}

// softwareLogMap maps software types to their log information
var softwareLogMap = map[enums.Software]LogInfo{
	enums.SoftwareMySql80: {Name: "MySQL Error Log", Software: "mysql80", ShowRoute: "/var/log/mysql/error.log"},
	enums.SoftwareRedis:   {Name: "Redis Server Log", Software: "redis", ShowRoute: "/var/log/redis/redis-server.log"},
	enums.SoftwarePhp56:   {Name: "PHP 5.6 FPM Log", Software: "php56", ShowRoute: "/var/log/php5.6-fpm.log"},
	enums.SoftwarePhp70:   {Name: "PHP 7.0 FPM Log", Software: "php70", ShowRoute: "/var/log/php7.0-fpm.log"},
	enums.SoftwarePhp71:   {Name: "PHP 7.1 FPM Log", Software: "php71", ShowRoute: "/var/log/php7.1-fpm.log"},
	enums.SoftwarePhp72:   {Name: "PHP 7.2 FPM Log", Software: "php72", ShowRoute: "/var/log/php7.2-fpm.log"},
	enums.SoftwarePhp73:   {Name: "PHP 7.3 FPM Log", Software: "php73", ShowRoute: "/var/log/php7.3-fpm.log"},
	enums.SoftwarePhp74:   {Name: "PHP 7.4 FPM Log", Software: "php74", ShowRoute: "/var/log/php7.4-fpm.log"},
	enums.SoftwarePhp80:   {Name: "PHP 8.0 FPM Log", Software: "php80", ShowRoute: "/var/log/php8.0-fpm.log"},
	enums.SoftwarePhp81:   {Name: "PHP 8.1 FPM Log", Software: "php81", ShowRoute: "/var/log/php8.1-fpm.log"},
	enums.SoftwarePhp82:   {Name: "PHP 8.2 FPM Log", Software: "php82", ShowRoute: "/var/log/php8.2-fpm.log"},
	enums.SoftwarePhp83:   {Name: "PHP 8.3 FPM Log", Software: "php83", ShowRoute: "/var/log/php8.3-fpm.log"},
	enums.SoftwarePhp84:   {Name: "PHP 8.4 FPM Log", Software: "php84", ShowRoute: "/var/log/php8.4-fpm.log"},
	enums.SoftwareCaddy2:  {Name: "Caddy Access Log", Software: "caddy2", ShowRoute: "/var/log/caddy/access.log"},
}

// ListLogs returns available logs for a server based on installed services
func (h *Handler) ListLogs(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	serverID := c.Params("id")

	// Get server with services
	server, err := h.service.GetServerWithRelations(c.Context(), serverID, teamID)
	if err != nil {
		return response.HandleErrorOrInternalErr(c, err, "Failed to fetch server")
	}

	// Build list of available logs based on installed services
	var logs []LogInfo

	for _, service := range server.Services {
		software := enums.Software(service.Software)
		if logInfo, ok := softwareLogMap[software]; ok {
			logs = append(logs, logInfo)
		}
	}

	// If no logs found, return empty array (not null)
	if logs == nil {
		logs = []LogInfo{}
	}

	return response.OK(c, "Logs retrieved", logs)
}
