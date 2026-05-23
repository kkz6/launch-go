package tasks

import "regexp"

// projectRefPattern matches `${{project.<KEY>}}` references inside an
// env-var value. Whitespace around `project.<KEY>` is tolerated so a
// user typing `${{ project.X }}` still resolves. Key matches the same
// identifier shape we accept on writes ([A-Za-z_][A-Za-z0-9_]*).
//
// Capture group 1 is the key. Package-level so it compiles once at
// startup.
//
// Lives in `tasks` because both `services` (synchronous run-now /
// configure paths) and `jobs` (the asynq deploy / run workers) need
// to call it, and `services → jobs → services` cycles already make
// shared-helper placement tricky here.
var projectRefPattern = regexp.MustCompile(
	`\$\{\{\s*project\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`,
)

// ResolveProjectRefs substitutes every `${{project.<KEY>}}` reference
// in `value` with the matching entry from `projectEnv`. Unknown keys
// are left as-is so the user can see exactly which reference broke
// (rather than the value silently becoming empty) — same policy
// dokploy uses.
//
// Called at deploy/run time, never at save time, so a project-level
// env-var update propagates to existing container rows on the next
// redeploy without rewriting their stored values.
func ResolveProjectRefs(value string, projectEnv map[string]string) string {
	if value == "" || projectEnv == nil {
		return value
	}
	return projectRefPattern.ReplaceAllStringFunc(value, func(match string) string {
		sub := projectRefPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if v, ok := projectEnv[sub[1]]; ok {
			return v
		}
		return match
	})
}

// ResolveProjectRefsInPairs runs ResolveProjectRefs over a slice of
// "KEY=VALUE" strings (the form `docker run -e` consumes). Only the
// value side is rewritten — keys never reference project env, and
// rewriting them would be a footgun.
func ResolveProjectRefsInPairs(pairs []string, projectEnv map[string]string) []string {
	if len(projectEnv) == 0 {
		return pairs
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		// Split on the FIRST `=` only. Env values can carry `=` (JWT
		// tokens, base64 blobs), so a normal split would mangle them.
		eq := -1
		for j := 0; j < len(p); j++ {
			if p[j] == '=' {
				eq = j
				break
			}
		}
		if eq < 0 {
			out[i] = p
			continue
		}
		key, value := p[:eq], p[eq+1:]
		out[i] = key + "=" + ResolveProjectRefs(value, projectEnv)
	}
	return out
}
