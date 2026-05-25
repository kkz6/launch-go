package notifications

import "fmt"

// Helpers shared by the backup success / failure notifications. Kept
// minimal — the two notification types stay separate so the fluent
// builder pattern (`NewDatabaseBackup*Notification(...).WithX(...)`)
// continues to return concrete types for caller chaining.

// fallback returns v when non-empty, else def — small helper used by
// the email body so missing optional fields read "—" instead of a
// blank line that confuses Outlook's quoting.
func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// humanSize returns a compact "1.2 GB" style label. Inlined here
// instead of pulling in a dependency because backup sizes span a few
// orders of magnitude and stdlib has nothing for it.
func humanSize(n int64) string {
	if n <= 0 {
		return "—"
	}
	const k = 1024.0
	v := float64(n)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for v >= k && i < len(units)-1 {
		v /= k
		i++
	}
	if v < 10 && i > 0 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.0f %s", v, units[i])
}
