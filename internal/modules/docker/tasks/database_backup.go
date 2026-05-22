package tasks

import (
	"fmt"
	"strings"
	"time"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// BackupRunConfig is the input for the run-backup script. ContainerName
// is the running database container we dump from; engine drives which
// CLI we exec inside it. S3 fields configure the upload target.
type BackupRunConfig struct {
	RunID         string
	ContainerName string
	Engine        dockertypes.DatabaseEngine
	Username      string
	Password      string
	Database      string

	// S3 destination. Endpoint is optional — present for non-AWS S3
	// providers (Backblaze B2, MinIO, Tigris, etc.).
	Endpoint   string
	Region     string
	Bucket     string
	PathPrefix string
	AccessKey  string
	SecretKey  string
}

// RunBackupScript renders the bash that dumps the database inside the
// running container, streams the dump to a local temp file, and uploads
// it to S3 via the aws CLI. AWS CLI is assumed to be installed on the
// docker server — we check at the top of the script and exit early
// with a friendly message if not.
//
// The script emits these LAUNCH markers:
//   - object_key  — the final S3 object key (path inside the bucket)
//   - size_bytes  — the dump file size on disk after compression
//
// On any failure exits non-zero so the calling job marks the run as
// failed; the marker for the size doesn't fire and the row records the
// error.
func RunBackupScript(cfg BackupRunConfig) string {
	dumpCmd, dumpExt := backupDumpCommand(cfg)

	objectKey := composeObjectKey(cfg)

	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")

	fmt.Fprintf(&b, "RUN_ID=%q\n", cfg.RunID)
	fmt.Fprintf(&b, "CONTAINER=%q\n", cfg.ContainerName)
	fmt.Fprintf(&b, "OBJECT_KEY=%q\n", objectKey)
	fmt.Fprintf(&b, "TMP_DIR=/var/lib/launch/backups/${RUN_ID}\n")
	fmt.Fprintf(&b, "TMP_FILE=${TMP_DIR}/dump.%s.gz\n\n", dumpExt)

	// AWS CLI sanity check up-front. Avoids dumping a multi-GB file
	// just to discover the upload tool isn't installed.
	b.WriteString(`if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI not installed on this server — install with 'apt install awscli'" >&2
  exit 1
fi

mkdir -p "${TMP_DIR}"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "::LAUNCH::backup_step::dumping"
`)
	fmt.Fprintf(&b, "%s | gzip > \"${TMP_FILE}\"\n", dumpCmd)
	b.WriteString(`
SIZE=$(wc -c <"${TMP_FILE}" | tr -d ' ')
echo "::LAUNCH::size_bytes::${SIZE}"

echo "::LAUNCH::backup_step::uploading"
`)
	// AWS creds + region come from env vars to keep them off the
	// commandline (ps would expose --access-key flags).
	fmt.Fprintf(&b, "export AWS_ACCESS_KEY_ID=%s\n", shellEscapeArg(cfg.AccessKey))
	fmt.Fprintf(&b, "export AWS_SECRET_ACCESS_KEY=%s\n", shellEscapeArg(cfg.SecretKey))
	if cfg.Region != "" {
		fmt.Fprintf(&b, "export AWS_DEFAULT_REGION=%s\n", shellEscapeArg(cfg.Region))
	}
	endpointArg := ""
	if cfg.Endpoint != "" {
		endpointArg = fmt.Sprintf(" --endpoint-url=%s", shellEscapeArg(cfg.Endpoint))
	}
	fmt.Fprintf(&b, "aws s3 cp \"${TMP_FILE}\" \"s3://%s/${OBJECT_KEY}\"%s\n",
		cfg.Bucket, endpointArg)
	b.WriteString(`
echo "::LAUNCH::object_key::${OBJECT_KEY}"
echo "::LAUNCH::backup_step::done"
`)
	return b.String()
}

// backupDumpCommand returns the engine-specific dump command plus the
// file extension we tag the resulting archive with. Each command is
// run via `docker exec` so passwords never traverse the host's PATH.
//
// Returns the raw command + the bare extension (no leading dot).
func backupDumpCommand(cfg BackupRunConfig) (string, string) {
	switch cfg.Engine {
	case dockertypes.DatabaseEnginePostgres:
		// PGPASSWORD is set inline on the docker exec invocation so it
		// never lands in the container's stored env (pg_dump only needs
		// it for the duration of this command).
		return fmt.Sprintf(
			`docker exec -e PGPASSWORD=%s %s pg_dump -U %s %s`,
			shellEscapeArg(cfg.Password),
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Username),
			shellEscapeArg(cfg.Database),
		), "sql"

	case dockertypes.DatabaseEngineMySQL, dockertypes.DatabaseEngineMariaDB:
		// --password=<value> isn't great from a ps-snoop perspective
		// but mysql-client demands it; the long-form --password=...
		// is at least scrubbed in newer mysql versions.
		return fmt.Sprintf(
			`docker exec %s mysqldump -u %s --password=%s %s`,
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Username),
			shellEscapeArg(cfg.Password),
			shellEscapeArg(cfg.Database),
		), "sql"

	case dockertypes.DatabaseEngineMongo:
		// `mongodump --archive` writes a single binary stream to
		// stdout, perfect for piping to gzip.
		return fmt.Sprintf(
			`docker exec %s mongodump --username=%s --password=%s --authenticationDatabase=admin --archive`,
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Username),
			shellEscapeArg(cfg.Password),
		), "bson"

	case dockertypes.DatabaseEngineRedis:
		// Redis has no native logical dump — we use BGSAVE then copy
		// the dump.rdb. Authentication via --pass is required when
		// the database was created with requirepass (our default).
		return fmt.Sprintf(
			`docker exec %s sh -c 'redis-cli --pass %s SAVE >/dev/null && cat /data/dump.rdb'`,
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Password),
		), "rdb"

	default:
		// Should be unreachable — the service validates engine.
		return fmt.Sprintf(`echo "unsupported engine %s" >&2; exit 1`, cfg.Engine), "bin"
	}
}

// composeObjectKey builds the S3 object key for a single run. Format:
//
//	[<path_prefix>/]<database>/<yyyy-mm-dd>/<runID>.<ext>.gz
//
// Keeping per-day folders makes browsing the bucket sane after a few
// months of automated backups.
func composeObjectKey(cfg BackupRunConfig) string {
	_, ext := backupDumpCommand(cfg)
	now := time.Now().UTC().Format("2006-01-02")
	parts := []string{}
	if cfg.PathPrefix != "" {
		parts = append(parts, strings.Trim(cfg.PathPrefix, "/"))
	}
	parts = append(parts, cfg.Database, now, cfg.RunID+"."+ext+".gz")
	return strings.Join(parts, "/")
}

// RunBackup is the taskrunner.Task wrapper.
func RunBackup(cfg BackupRunConfig) taskrunner.Task {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Backup Database"),
		taskrunner.WithScript(RunBackupScript(cfg)),
		taskrunner.WithTimeoutSeconds(3600),
	)
}

// RestoreBackupConfig is the input for the restore script. The S3 key
// identifies which past snapshot to fetch; the rest is the same
// authentication info as BackupRunConfig.
type RestoreBackupConfig struct {
	RunID         string
	ContainerName string
	Engine        dockertypes.DatabaseEngine
	Username      string
	Password      string
	Database      string

	Endpoint  string
	Region    string
	Bucket    string
	ObjectKey string
	AccessKey string
	SecretKey string
}

// RestoreBackupScript downloads the snapshot from S3 and restores it
// into the running container. Each engine has its own ingest command;
// the wrapping pattern (download → gunzip → exec into container) is
// identical to the backup script in reverse.
func RestoreBackupScript(cfg RestoreBackupConfig) string {
	ingest := restoreIngestCommand(cfg)

	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")
	fmt.Fprintf(&b, "RUN_ID=%q\n", cfg.RunID)
	fmt.Fprintf(&b, "CONTAINER=%q\n", cfg.ContainerName)
	fmt.Fprintf(&b, "OBJECT_KEY=%q\n", cfg.ObjectKey)
	fmt.Fprintf(&b, "TMP_DIR=/var/lib/launch/restore/${RUN_ID}\n")
	b.WriteString(`TMP_FILE="${TMP_DIR}/dump.gz"

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI not installed on this server" >&2
  exit 1
fi

mkdir -p "${TMP_DIR}"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "::LAUNCH::restore_step::downloading"
`)
	fmt.Fprintf(&b, "export AWS_ACCESS_KEY_ID=%s\n", shellEscapeArg(cfg.AccessKey))
	fmt.Fprintf(&b, "export AWS_SECRET_ACCESS_KEY=%s\n", shellEscapeArg(cfg.SecretKey))
	if cfg.Region != "" {
		fmt.Fprintf(&b, "export AWS_DEFAULT_REGION=%s\n", shellEscapeArg(cfg.Region))
	}
	endpointArg := ""
	if cfg.Endpoint != "" {
		endpointArg = fmt.Sprintf(" --endpoint-url=%s", shellEscapeArg(cfg.Endpoint))
	}
	fmt.Fprintf(&b, "aws s3 cp \"s3://%s/${OBJECT_KEY}\" \"${TMP_FILE}\"%s\n",
		cfg.Bucket, endpointArg)

	b.WriteString(`
echo "::LAUNCH::restore_step::restoring"
`)
	b.WriteString("gunzip -c \"${TMP_FILE}\" | ")
	b.WriteString(ingest)
	b.WriteString("\necho \"::LAUNCH::restore_step::done\"\n")

	return b.String()
}

// restoreIngestCommand returns the per-engine ingest command. Each one
// reads from stdin (so we can pipe gunzip output straight in) and runs
// inside the running container via `docker exec -i`.
func restoreIngestCommand(cfg RestoreBackupConfig) string {
	switch cfg.Engine {
	case dockertypes.DatabaseEnginePostgres:
		return fmt.Sprintf(
			`docker exec -i -e PGPASSWORD=%s %s psql -U %s -d %s`,
			shellEscapeArg(cfg.Password),
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Username),
			shellEscapeArg(cfg.Database),
		)
	case dockertypes.DatabaseEngineMySQL, dockertypes.DatabaseEngineMariaDB:
		return fmt.Sprintf(
			`docker exec -i %s mysql -u %s --password=%s %s`,
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Username),
			shellEscapeArg(cfg.Password),
			shellEscapeArg(cfg.Database),
		)
	case dockertypes.DatabaseEngineMongo:
		return fmt.Sprintf(
			`docker exec -i %s mongorestore --username=%s --password=%s --authenticationDatabase=admin --archive --drop`,
			shellEscapeArg(cfg.ContainerName),
			shellEscapeArg(cfg.Username),
			shellEscapeArg(cfg.Password),
		)
	case dockertypes.DatabaseEngineRedis:
		// Restoring a Redis RDB requires stopping the container, dropping
		// the new file in place, and restarting. That's a bigger flow
		// than docker exec — explicitly out of scope here.
		return `echo "redis restore is not yet supported — copy the .rdb manually" >&2; exit 1`
	default:
		return fmt.Sprintf(`echo "unsupported engine %s" >&2; exit 1`, cfg.Engine)
	}
}

// Restore is the taskrunner.Task wrapper for the restore script.
func Restore(cfg RestoreBackupConfig) taskrunner.Task {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restore Database"),
		taskrunner.WithScript(RestoreBackupScript(cfg)),
		taskrunner.WithTimeoutSeconds(3600),
	)
}
