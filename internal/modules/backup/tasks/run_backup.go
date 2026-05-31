// Package tasks renders the SSH scripts that execute server backups on
// the customer's box. Mirrors the docker-module backup task shape:
// shell-out → launch-agent upload, no aws CLI dependency.
package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunBackupConfig carries everything the rendered script needs to dump
// + upload one server backup. Credentials travel via env vars to keep
// them off the commandline; bucket/region/endpoint/path-style are
// non-secret and ride as flags.
type RunBackupConfig struct {
	JobID        string   // backup_jobs row id (ULID)
	BackupID     string   // parent backup row id (ULID) — drives the S3 key
	IncludeFiles []string // server-side paths to archive
	ExcludeFiles []string // glob-style exclusions passed to tar

	Endpoint       string
	Region         string
	Bucket         string
	PathPrefix     string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

// RunBackupScript renders the bash script that:
//
//  1. Verifies launch-agent is installed (the only required tool).
//  2. Tars the include paths (with excludes), gzipped, into a temp file.
//  3. Uploads via `launch-agent upload`.
//  4. Cleans up.
//
// Emits ::LAUNCH:: markers (backup_step, size_bytes, object_key) so the
// dispatching job can parse the final object key + size from the output
// and update the backup_jobs row.
func RunBackupScript(cfg RunBackupConfig) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")

	fmt.Fprintf(&b, "JOB_ID=%q\n", cfg.JobID)
	fmt.Fprintf(&b, "BACKUP_ID=%q\n", cfg.BackupID)
	// Build the object key on the box so the path-prefix logic stays
	// in one place; we still parse it back from the marker.
	objectKey := composeObjectKey(cfg)
	fmt.Fprintf(&b, "OBJECT_KEY=%q\n", objectKey)
	b.WriteString("TMP_DIR=/var/lib/launch/backups/${JOB_ID}\n")
	b.WriteString("TMP_FILE=${TMP_DIR}/backup.tar.gz\n\n")

	b.WriteString(`if ! command -v launch-agent >/dev/null 2>&1; then
  echo "launch-agent not installed on this server — backups require the Launch agent" >&2
  exit 1
fi

mkdir -p "${TMP_DIR}"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "==> Server backup starting (job ${JOB_ID})"
echo "==> [1/2] Archiving include paths..."
echo "::LAUNCH::backup_step::archiving"
`)

	// Build the tar command. tar's --exclude can repeat; include paths
	// come positionally. -C / so the archive stores absolute paths
	// rooted at /, matching how operators expect to restore them.
	var tarArgs strings.Builder
	tarArgs.WriteString("tar --warning=no-file-changed -czf \"${TMP_FILE}\"")
	for _, ex := range cfg.ExcludeFiles {
		if strings.TrimSpace(ex) == "" {
			continue
		}
		fmt.Fprintf(&tarArgs, " --exclude=%s", shellEscape(ex))
	}
	tarArgs.WriteString(" -C /")
	includesEmpty := true
	for _, inc := range cfg.IncludeFiles {
		inc = strings.TrimPrefix(strings.TrimSpace(inc), "/")
		if inc == "" {
			continue
		}
		fmt.Fprintf(&tarArgs, " %s", shellEscape(inc))
		includesEmpty = false
	}
	if includesEmpty {
		// Defensive — the configure-time form requires at least one
		// include path, but a misconfigured row would silently archive
		// the whole filesystem rooted at /. Fail loudly.
		b.WriteString("echo \"backup has no include paths configured — refusing to run\" >&2\nexit 1\n")
	} else {
		fmt.Fprintf(&b, "%s\n", tarArgs.String())
	}

	b.WriteString(`if [ ! -s "${TMP_FILE}" ]; then
  echo "archive is empty — refusing to upload" >&2
  exit 1
fi
SIZE=$(wc -c <"${TMP_FILE}" | tr -d ' ')
echo "::LAUNCH::size_bytes::${SIZE}"
echo "==> Archive complete — ${SIZE} bytes"

echo "==> [2/2] Uploading to S3..."
echo "::LAUNCH::backup_step::uploading"
`)

	fmt.Fprintf(&b, "export AWS_ACCESS_KEY_ID=%s\n", shellEscape(cfg.AccessKey))
	fmt.Fprintf(&b, "export AWS_SECRET_ACCESS_KEY=%s\n", shellEscape(cfg.SecretKey))
	fmt.Fprintf(&b, "launch-agent upload --file \"${TMP_FILE}\" --key \"${OBJECT_KEY}\"%s\n",
		s3Flags(cfg.Bucket, cfg.Region, cfg.Endpoint, cfg.ForcePathStyle))

	b.WriteString(`echo "==> Upload complete"
echo "::LAUNCH::object_key::${OBJECT_KEY}"
echo "::LAUNCH::backup_step::done"
echo "==> Server backup finished successfully"
`)
	return b.String()
}

// RunBackup is the taskrunner.Task wrapper.
func RunBackup(cfg RunBackupConfig) taskrunner.Task {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Run Server Backup"),
		taskrunner.WithScript(RunBackupScript(cfg)),
		taskrunner.WithTimeoutSeconds(3600),
	)
}

// composeObjectKey builds the S3 object key for a server backup:
//
//	[<path>/]server/<backup_id>/<job_id>.tar.gz
func composeObjectKey(cfg RunBackupConfig) string {
	parts := []string{}
	if p := strings.Trim(cfg.PathPrefix, "/"); p != "" {
		parts = append(parts, p)
	}
	parts = append(parts, "server", cfg.BackupID, cfg.JobID+".tar.gz")
	return strings.Join(parts, "/")
}

func s3Flags(bucket, region, endpoint string, forcePath bool) string {
	out := " --bucket " + shellEscape(bucket)
	if region != "" {
		out += " --region " + shellEscape(region)
	}
	if endpoint != "" {
		out += " --endpoint " + shellEscape(endpoint)
	}
	if forcePath {
		out += " --force-path-style"
	}
	return out
}

// shellEscape single-quotes a value safely for bash. ' inside the value
// is closed-then-reopened around an escaped literal.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ParseRunMarkers pulls the final object_key + size_bytes out of the
// script's stdout. Returns ("", 0) if neither was emitted (failed
// before upload).
func ParseRunMarkers(out string) (objectKey string, sizeBytes int64) {
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "::LAUNCH::") {
			continue
		}
		body := strings.TrimPrefix(line, "::LAUNCH::")
		parts := strings.SplitN(body, "::", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "object_key":
			objectKey = parts[1]
		case "size_bytes":
			var n int64
			_, _ = fmt.Sscanf(parts[1], "%d", &n)
			sizeBytes = n
		}
	}
	return objectKey, sizeBytes
}
