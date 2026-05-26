package tasks

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomHeredocSentinel returns a sentinel safe to use as the closing
// delimiter of a bash heredoc, e.g.
//
//	cat <<'SENTINEL' > /path
//	  …body…
//	SENTINEL
//
// Using a fixed sentinel for caller-supplied content is a latent
// shell-injection bug: a body containing exactly the sentinel line
// terminates the heredoc early and lets trailing bytes run as shell
// commands. The randomised suffix makes that collision impossible
// for any practical body (16 hex chars = 64 bits of entropy).
//
// The sentinel always starts with `LAUNCH_EOF_` so the same shape
// shows up in logs and is easy to grep for if anyone is debugging a
// stuck script.
func RandomHeredocSentinel() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "LAUNCH_EOF_" + hex.EncodeToString(b[:])
}
