package tasks

import (
	"strconv"
	"strings"
)

// Shared helpers for the backup script's output contract. Both the
// service-layer (synchronous RunNow / Restore) and the worker-job
// layer (scheduled cron) parse the same markers and compose the
// same destination paths, so they live alongside the script
// generators that emit them.

// BackupObjectPath composes the final bucket-prefix used by the upload
// script — the storage provider's "default" path joined with the
// per-backup sub-folder. Either may be empty; leading/trailing slashes
// + spaces are stripped so the script's "<prefix>/<file>" concat
// doesn't double up.
func BackupObjectPath(perBackup *string, providerDefault string) string {
	trim := func(s string) string {
		return strings.Trim(strings.TrimSpace(s), "/")
	}
	out := trim(providerDefault)
	if perBackup == nil {
		return out
	}
	seg := trim(*perBackup)
	switch {
	case seg == "":
		return out
	case out == "":
		return seg
	default:
		return out + "/" + seg
	}
}

// ParseRunMarkers reads `::LAUNCH::object_key::<k>` and
// `::LAUNCH::size_bytes::<n>` from the backup script's stdout. Missing
// values mean the script aborted before emitting them — callers treat
// a zero size or empty key as a failed run.
//
// Size parsing is tolerant of trailing garbage (a stray non-digit
// terminates the value) because the script-side `wc -c` is normally
// clean but we don't want a hypothetical formatting bug to lose the
// recorded size.
func ParseRunMarkers(output string) (objectKey string, sizeBytes int64) {
	const okPrefix = "::LAUNCH::object_key::"
	const szPrefix = "::LAUNCH::size_bytes::"
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, okPrefix):
			objectKey = line[len(okPrefix):]
		case strings.HasPrefix(line, szPrefix):
			rest := line[len(szPrefix):]
			end := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
			if end == -1 {
				end = len(rest)
			}
			if n, err := strconv.ParseInt(rest[:end], 10, 64); err == nil {
				sizeBytes = n
			}
		}
	}
	return objectKey, sizeBytes
}
