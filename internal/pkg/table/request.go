package table

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// Request is the parsed query string for a /data call.
type Request struct {
	Page    int
	PerPage int
	Sort    string
	Search  string
	// Filters is a key → clause → value map. Values are deliberately `any` so
	// each filter implementation can validate them with its own rules
	// (string for text, []string for in, two-element array for between, etc.).
	Filters map[string]map[Clause]any
	// Columns is the user-toggled visible column list (CSV).
	Columns []string
}

// ParseRequest reads the bracket-notation Fiber query params into a Request.
// The frontend sends:
//
//	?page=1&limit=15&sort=email:desc&search=foo
//	&filters[email][contains]=bar
//	&filters[provider][in]=google,apple
type RequestSource interface {
	Query(key string, defaultValue ...string) string
	Queries() map[string]string
}

func ParseRequest(src RequestSource) Request {
	r := Request{
		Filters: map[string]map[Clause]any{},
	}
	page, _ := strconv.Atoi(src.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(src.Query("limit", "0"))
	r.Page = page
	r.PerPage = limit
	r.Sort = src.Query("sort")
	r.Search = src.Query("search")
	if cols := src.Query("columns"); cols != "" {
		r.Columns = strings.Split(cols, ",")
	}

	// Try a JSON-encoded `filters` first.
	if raw := src.Query("filters"); raw != "" {
		if parsed, ok := parseJSONFilters(raw); ok {
			r.Filters = parsed
			return r
		}
	}

	bracket := regexp.MustCompile(`^filters\[(\w+)]\[(\w+)]$`)
	for k, v := range src.Queries() {
		m := bracket.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		key := m[1]
		clause := Clause(m[2])
		if r.Filters[key] == nil {
			r.Filters[key] = map[Clause]any{}
		}
		// For "in" / "not_in" we accept CSV in the URL too.
		if clause == ClauseIn || clause == ClauseNotIn {
			r.Filters[key][clause] = splitCSV(v)
		} else if clause == ClauseBetween || clause == ClauseNotBetween {
			parts := splitCSV(v)
			if len(parts) == 2 {
				r.Filters[key][clause] = []any{parts[0], parts[1]}
			}
		} else {
			r.Filters[key][clause] = v
		}
	}
	return r
}

func parseJSONFilters(raw string) (map[string]map[Clause]any, bool) {
	var tmp map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return nil, false
	}
	out := make(map[string]map[Clause]any, len(tmp))
	for k, cm := range tmp {
		inner := make(map[Clause]any, len(cm))
		for c, v := range cm {
			inner[Clause(c)] = v
		}
		out[k] = inner
	}
	return out, true
}

func splitCSV(s string) []any {
	parts := strings.Split(s, ",")
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
