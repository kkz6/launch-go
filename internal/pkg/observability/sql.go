package observability

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

const maxSQLBytes = 4096

// RedactSQL preserves query structure without values or comments. PostgreSQL
// double quotes identify columns; MySQL and SQLite can use them for literals.
// Bound the input before scanning, including unterminated strings/comments.
func RedactSQL(sql, dialect string) string {
	truncated := len(sql) > 4*maxSQLBytes
	if truncated {
		sql = sql[:4*maxSQLBytes]
	}
	var out strings.Builder
	for i := 0; i < len(sql); {
		if out.Len() >= maxSQLBytes {
			truncated = true
			break
		}
		switch {
		case sql[i] == '\'' || (sql[i] == '"' && dialect != "postgres"):
			// PostgreSQL ordinary strings and SQLite strings use doubled
			// quotes, not backslash escapes. PostgreSQL E'...' is explicit.
			escaped := dialect == "postgres" && i > 0 && (sql[i-1] == 'E' || sql[i-1] == 'e') && (i == 1 || !identifierByte(sql[i-2]))
			end := quotedEnd(sql, i, escaped)
			if dialect == "mysql" || dialect == "" || dialect == "postgres" && !escaped {
				// MySQL's NO_BACKSLASH_ESCAPES and PostgreSQL's
				// standard_conforming_strings are connection-specific.
				// If quote boundaries differ, omit the rest rather than risk
				// treating the next private literal as unquoted SQL.
				if alternate := quotedEnd(sql, i, true); alternate != end {
					end = len(sql)
				}
			}
			i = end
			out.WriteByte('?')
		case strings.HasPrefix(sql[i:], "--") || (sql[i] == '#' && dialect == "mysql"):
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			out.WriteByte(' ')
		case strings.HasPrefix(sql[i:], "/*"):
			i += 2
			depth := 1
			for i < len(sql) && depth > 0 {
				switch {
				case strings.HasPrefix(sql[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(sql[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			out.WriteByte(' ')
		case sql[i] == '$':
			end := i + 1
			for end < len(sql) && identifierByte(sql[end]) {
				end++
			}
			if end < len(sql) && sql[end] == '$' {
				tag := sql[i : end+1]
				closing := strings.Index(sql[end+1:], tag)
				i = len(sql)
				if closing >= 0 {
					i = end + 1 + closing + len(tag)
				}
				out.WriteByte('?')
			} else if i+1 < len(sql) && digit(sql[i+1]) {
				i++
				for i < len(sql) && digit(sql[i]) {
					i++
				}
				out.WriteByte('?')
			} else {
				out.WriteByte(sql[i])
				i++
			}
		case sql[i] == '"' || sql[i] == '`':
			end := quotedEnd(sql, i, false)
			out.WriteString(bounded(sql[i:end], maxSQLBytes-out.Len()))
			i = end
		case digit(sql[i]) && (i == 0 || !identifierByte(sql[i-1])):
			i++
			for i < len(sql) && (digit(sql[i]) || strings.ContainsRune(".eExXaAbBcCdDfF+-", rune(sql[i]))) {
				i++
			}
			out.WriteByte('?')
		default:
			out.WriteByte(sql[i])
			i++
		}
	}
	result := strings.Join(strings.Fields(out.String()), " ")
	if truncated {
		result = bounded(result, maxSQLBytes-3) + "..."
	}
	return result
}

func quotedEnd(sql string, start int, backslashEscapes bool) int {
	quote := sql[start]
	for i := start + 1; i < len(sql); i++ {
		if sql[i] == '\\' && backslashEscapes {
			i++
		} else if sql[i] == quote {
			if i+1 < len(sql) && sql[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(sql)
}

func digit(b byte) bool { return b >= '0' && b <= '9' }

func identifierByte(b byte) bool {
	return digit(b) || b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// QueryError keeps useful categories without driver messages, which may quote
// credentials, row values or the original SQL. SQLSTATE is safe structured data.
func QueryError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "record not found"
	case errors.Is(err, context.Canceled):
		return "context canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline exceeded"
	default:
		var state interface{ SQLState() string }
		if errors.As(err, &state) && validSQLState(state.SQLState()) {
			return "query failed (SQLSTATE " + state.SQLState() + ")"
		}
		return "query failed"
	}
}

func validSQLState(state string) bool {
	if len(state) != 5 {
		return false
	}
	for _, b := range state {
		if !(b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			return false
		}
	}
	return true
}

func safeErrorCategory(value string) string {
	switch value {
	case "", "record not found", "context canceled", "deadline exceeded":
		return value
	}
	if strings.HasPrefix(value, "query failed (SQLSTATE ") && strings.HasSuffix(value, ")") && validSQLState(strings.TrimSuffix(strings.TrimPrefix(value, "query failed (SQLSTATE "), ")")) {
		return value
	}
	return "query failed"
}
