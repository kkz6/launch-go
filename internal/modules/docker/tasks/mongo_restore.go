package tasks

import (
	"fmt"
	"strings"
)

// ValidateMongoRestoreDatabases rejects names that can broaden Mongo's
// namespace patterns or restore instance-wide authentication/system state.
func ValidateMongoRestoreDatabases(source, target string) error {
	for _, name := range []string{source, target} {
		if name == "" || len(name) > 63 || strings.ContainsAny(name, `/\. "$*?[]`+"\x00\r\n\t`") {
			return fmt.Errorf("MongoDB restore requires explicit source and target database names without namespace pattern characters")
		}
		switch strings.ToLower(name) {
		case "admin", "config", "local":
			return fmt.Errorf("MongoDB system databases cannot be restored through application database restore")
		}
	}
	return nil
}

func mongoRestoreCommand(cfg RestoreBackupConfig) string {
	return fmt.Sprintf(
		`docker exec -i %s mongorestore --username=%s --password=%s --authenticationDatabase=admin --nsInclude=%s --nsFrom=%s --nsTo=%s --archive`,
		shellEscapeArg(cfg.ContainerName), shellEscapeArg(cfg.Username), shellEscapeArg(cfg.Password),
		shellEscapeArg(cfg.SourceDatabase+".*"), shellEscapeArg(cfg.SourceDatabase+".*"), shellEscapeArg(cfg.Database+".*"),
	)
}

func mongoRestorePreflight(cfg RestoreBackupConfig) string {
	if err := ValidateMongoRestoreDatabases(cfg.SourceDatabase, cfg.Database); err != nil {
		return fmt.Sprintf("printf '%%s\\n' %s >&2\nexit 1\n", shellEscapeArg(err.Error()))
	}
	// mongorestore otherwise exits successfully when nsInclude matches nothing.
	// Its documented dry-run mode examines archive metadata without importing.
	// Require an actual selected collection in the verbose plan, not merely an
	// archive prelude (which also lists namespaces excluded by nsInclude). If a
	// tool version cannot provide this evidence, fail closed before --drop.
	return fmt.Sprintf(`MONGO_PLAN="${TMP_DIR}/mongo-restore-plan.log"
if ! %s --dryRun --verbose < "${TMP_UNZIP}" > "${MONGO_PLAN}" 2>&1; then
  cat "${MONGO_PLAN}" >&2
  echo 'MongoDB archive namespace validation failed; no data was imported.' >&2
  exit 1
fi
if ! grep -F -- %s "${MONGO_PLAN}" >/dev/null; then
  echo 'MongoDB archive has no verified collections for the recorded source database; no data was imported.' >&2
  exit 1
fi
`, mongoRestoreCommand(cfg), shellEscapeArg("found collection `"+cfg.SourceDatabase+"."))
}

func mongoRestoreIngestCommand(cfg RestoreBackupConfig) string {
	if err := ValidateMongoRestoreDatabases(cfg.SourceDatabase, cfg.Database); err != nil {
		return fmt.Sprintf("printf '%%s\\n' %s >&2; exit 1", shellEscapeArg(err.Error()))
	}
	return mongoRestoreCommand(cfg) + " --stopOnError --drop"
}
