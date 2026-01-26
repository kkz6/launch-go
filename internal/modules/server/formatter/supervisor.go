package formatter

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// SupervisorConfig generates the supervisor configuration file contents for a daemon.
func SupervisorConfig(daemon *models.Daemon) string {
	dir := ""
	if daemon.Directory != nil {
		dir = *daemon.Directory
	}

	config := fmt.Sprintf(`[program:%s]
process_name=%%(program_name)s_%%(process_num)02d
command=%s
autostart=true
autorestart=true
stopasgroup=true
killasgroup=true
user=%s
numprocs=%d
redirect_stderr=true
stdout_logfile=%s
stderr_logfile=%s
stopwaitsecs=%d
stopsignal=%s
`, daemon.ProgramName(), daemon.Command, daemon.User, daemon.Processes,
		daemon.GetLogPath(), daemon.GetErrorLogPath(), daemon.StopWaitSeconds, daemon.StopSignal)

	if dir != "" {
		config += fmt.Sprintf("directory=%s\n", dir)
	}

	return config
}
