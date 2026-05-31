// Package tasks renders the SSH scripts that execute server backups on
// the customer's box. Mirrors the docker-module backup task shape:
// shell-out → launch-agent upload, no aws CLI dependency.
package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DatabaseDump describes one database to dump as part of the backup.
// Engine selects the dump command (mysqldump vs pg_dump). Password
// travels inline; we keep it off the commandline via env vars in the
// generated script (PGPASSWORD / MYSQL_PWD).
type DatabaseDump struct {
	Name     string // database name on the server
	Engine   string // "mysql", "postgres" (normalised from server-software keys)
	Username string
	Password string
}

// RunBackupConfig carries everything the rendered script needs to dump
// + upload one server backup. Credentials travel via env vars to keep
// them off the commandline; bucket/region/endpoint/path-style are
// non-secret and ride as flags.
type RunBackupConfig struct {
	JobID        string         // backup_jobs row id (ULID)
	BackupID     string         // parent backup row id (ULID) — drives the S3 key
	Databases    []DatabaseDump // logical databases to dump before tarring
	IncludeFiles []string       // server-side filesystem paths to archive
	ExcludeFiles []string       // glob-style exclusions passed to tar

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
DUMP_DIR="${TMP_DIR}/databases"
mkdir -p "${DUMP_DIR}"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "==> Server backup starting (job ${JOB_ID})"
`)

	// Step 1 — database dumps. Each linked database becomes a
	// per-engine dump file under $DUMP_DIR; tar in step 2 picks them
	// up. Credentials ride via env vars (MYSQL_PWD / PGPASSWORD) so
	// they don't appear in `ps`.
	hasDumps := false
	for _, d := range cfg.Databases {
		if d.Name == "" {
			continue
		}
		hasDumps = true
		engine := strings.ToLower(d.Engine)
		safe := safeFilename(d.Name)
		out := fmt.Sprintf("${DUMP_DIR}/%s.sql.gz", safe)
		fmt.Fprintf(&b, "echo \"==> Dumping %s database %q...\"\n", engine, d.Name)
		switch {
		case strings.HasPrefix(engine, "mysql") || engine == "mariadb":
			// `MYSQL_PWD` is honoured by the mysql client. Use
			// --single-transaction for crash-consistent InnoDB
			// dumps without locking tables.
			fmt.Fprintf(&b, "MYSQL_PWD=%s mysqldump --single-transaction --quick -u %s %s | gzip > %s\n",
				shellEscape(d.Password), shellEscape(d.Username), shellEscape(d.Name), out)
		case strings.HasPrefix(engine, "postgres"):
			fmt.Fprintf(&b, "PGPASSWORD=%s pg_dump -U %s %s | gzip > %s\n",
				shellEscape(d.Password), shellEscape(d.Username), shellEscape(d.Name), out)
		default:
			fmt.Fprintf(&b, "echo \"unsupported database engine %s for %s — skipping\" >&2\n",
				shellEscape(engine), shellEscape(d.Name))
			continue
		}
	}

	b.WriteString("\necho \"==> Archiving backup...\"\n")
	b.WriteString("echo \"::LAUNCH::backup_step::archiving\"\n")

	// Step 2 — tar everything (include paths + the dumps directory).
	// tar's --exclude can repeat; positional args come last. -C / so
	// the archive stores absolute paths rooted at /, matching how
	// operators expect to restore them. The dumps directory lives
	// under TMP_DIR so we add it via a second `-C` block.
	var tarArgs strings.Builder
	tarArgs.WriteString("tar --warning=no-file-changed -czf \"${TMP_FILE}\"")
	for _, ex := range cfg.ExcludeFiles {
		if strings.TrimSpace(ex) == "" {
			continue
		}
		fmt.Fprintf(&tarArgs, " --exclude=%s", shellEscape(ex))
	}
	hasIncludes := false
	if len(cfg.IncludeFiles) > 0 {
		tarArgs.WriteString(" -C /")
		for _, inc := range cfg.IncludeFiles {
			inc = strings.TrimPrefix(strings.TrimSpace(inc), "/")
			if inc == "" {
				continue
			}
			fmt.Fprintf(&tarArgs, " %s", shellEscape(inc))
			hasIncludes = true
		}
	}
	if hasDumps {
		// Add the dumps directory contents (databases/ subfolder in
		// the archive). Switching -C mid-command applies to following
		// positional args.
		tarArgs.WriteString(" -C \"${TMP_DIR}\" databases")
	}

	if !hasDumps && !hasIncludes {
		// Truly nothing to back up — fail loudly. (Refusing to run is
		// safer than tar'ing an empty archive into S3.)
		b.WriteString("echo \"backup has no databases selected and no include paths — nothing to back up\" >&2\nexit 1\n")
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

// safeFilename strips characters that are awkward in a dump filename
// (path separators, NULs) so a database with a colourful name doesn't
// blow up the tar archive layout. Conservative — anything not in
// [A-Za-z0-9._-] becomes "_".
func safeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "db"
	}
	return out
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
