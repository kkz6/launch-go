package services

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// A successful SSH exit alone is not proof that the restore script completed.
func restoreResultError(result *taskrunner.SSHCommandResult, runErr error, cfg tasks.RestoreBackupConfig) error {
	if runErr == nil && result != nil && result.ExitCode == 0 {
		for _, line := range strings.Split(result.Stdout, "\n") {
			if strings.TrimSpace(line) == "::LAUNCH::restore_step::done" {
				return nil
			}
		}
	}
	detail := "No verified restore completion was received."
	if result != nil {
		detail = fmt.Sprintf("Database restore exited with status %d. %s", result.ExitCode, result.Stderr)
		if result.ExitCode == 0 {
			detail = "The restore completion marker was not received."
		}
	}
	if runErr != nil {
		detail = runErr.Error() + " " + detail
	}
	// Scrub credentials before truncating, including shell/URL forms that a
	// client may include in its connection diagnostics.
	var secrets []string
	for _, secret := range []string{cfg.Password, cfg.AccessKey, cfg.SecretKey} {
		if secret != "" {
			secrets = append(secrets, secret, taskrunner.ShellQuote(secret), url.QueryEscape(secret), url.PathEscape(secret))
		}
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	replacements := make([]string, 0, len(secrets)*2)
	for _, secret := range secrets {
		replacements = append(replacements, secret, "[redacted]")
	}
	detail = strings.NewReplacer(replacements...).Replace(detail)
	return fiberutil.Internal(fmt.Sprintf("Restore did not complete. If ingestion started, the target may contain partial changes; inspect it before retrying. %s", truncate(strings.TrimSpace(detail), 4000)))
}
