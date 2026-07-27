// Package observability collects lightweight, in-process diagnostics for staff
// users. It deliberately keeps data bounded and never persists query text.
package observability

import (
	"context"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const defaultCapacity = 200

type QueryEntry struct {
	SQL        string    `json:"sql"`
	DurationNS int64     `json:"duration_ns"`
	Rows       int64     `json:"rows"`
	Caller     string    `json:"caller"`
	TraceID    string    `json:"trace_id,omitempty"`
	Slow       bool      `json:"slow"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type N1Pattern struct {
	SQL       string    `json:"sql"`
	Count     int       `json:"count"`
	TraceID   string    `json:"trace_id,omitempty"`
	Caller    string    `json:"caller"`
	TotalMS   float64   `json:"total_ms"`
	Timestamp time.Time `json:"timestamp"`
}

type QuerySummary struct {
	Total       int          `json:"total"`
	SlowCount   int          `json:"slow_count"`
	N1Count     int          `json:"n1_count"`
	AvgMS       float64      `json:"avg_ms"`
	Recent      []QueryEntry `json:"recent"`
	SlowQueries []QueryEntry `json:"slow_queries"`
	N1Patterns  []N1Pattern  `json:"n1_patterns"`
}

type Tracker struct {
	mu        sync.RWMutex
	entries   []QueryEntry
	capacity  int
	threshold time.Duration
}

func NewTracker(threshold time.Duration) *Tracker {
	if threshold <= 0 {
		threshold = 200 * time.Millisecond
	}
	return &Tracker{capacity: defaultCapacity, threshold: threshold}
}

// Register attaches one callback after every GORM query/create/update/delete.
func (t *Tracker) Register(db *gorm.DB) {
	start := func(tx *gorm.DB) { tx.InstanceSet("observability:started_at", time.Now()) }
	track := func(tx *gorm.DB) { t.Record(statementEntry(tx)) }
	_ = db.Callback().Query().Before("gorm:query").Register("observability:query-start", start)
	_ = db.Callback().Query().After("gorm:query").Register("observability:query", track)
	_ = db.Callback().Create().Before("gorm:create").Register("observability:create-start", start)
	_ = db.Callback().Create().After("gorm:create").Register("observability:create", track)
	_ = db.Callback().Update().Before("gorm:update").Register("observability:update-start", start)
	_ = db.Callback().Update().After("gorm:update").Register("observability:update", track)
	_ = db.Callback().Delete().Before("gorm:delete").Register("observability:delete-start", start)
	_ = db.Callback().Delete().After("gorm:delete").Register("observability:delete", track)
	_ = db.Callback().Raw().Before("gorm:raw").Register("observability:raw-start", start)
	_ = db.Callback().Raw().After("gorm:raw").Register("observability:raw", track)
}

func statementEntry(tx *gorm.DB) QueryEntry {
	if tx == nil || tx.Statement == nil {
		return QueryEntry{}
	}
	sql := tx.Statement.SQL.String()
	if sql == "" {
		return QueryEntry{}
	}
	startedAt, _ := tx.InstanceGet("observability:started_at")
	started, _ := startedAt.(time.Time)
	duration := time.Since(started)
	e := QueryEntry{SQL: sql, DurationNS: duration.Nanoseconds(), Rows: tx.Statement.RowsAffected, Slow: duration >= 200*time.Millisecond, Timestamp: time.Now()}
	if traceID, _ := tx.Statement.Context.Value(traceIDKey{}).(string); traceID != "" {
		e.TraceID = traceID
	}
	if tx.Error != nil {
		e.Error = tx.Error.Error()
	}
	e.Caller = caller()
	return e
}

func caller() string {
	pcs := make([]uintptr, 12)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.Function, "gorm.io/") && !strings.Contains(frame.Function, "/observability.") {
			return frame.Function
		}
		if !more {
			return ""
		}
	}
}

func (t *Tracker) Record(entry QueryEntry) {
	if entry.SQL == "" {
		return
	}
	entry.Slow = entry.DurationNS >= t.threshold.Nanoseconds()
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.entries) == t.capacity {
		copy(t.entries, t.entries[1:])
		t.entries[len(t.entries)-1] = entry
		return
	}
	t.entries = append(t.entries, entry)
}

func (t *Tracker) Queries(limit int, slowOnly bool) []QueryEntry {
	if limit <= 0 || limit > defaultCapacity {
		limit = 50
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]QueryEntry, 0, limit)
	for i := len(t.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if !slowOnly || t.entries[i].Slow {
			result = append(result, t.entries[i])
		}
	}
	return result
}

func (t *Tracker) Summary() QuerySummary {
	t.mu.RLock()
	entries := append([]QueryEntry(nil), t.entries...)
	t.mu.RUnlock()
	s := QuerySummary{Recent: newest(entries, 20), SlowQueries: newestSlow(entries, 20), N1Patterns: n1(entries)}
	s.Total = len(entries)
	var total int64
	for _, entry := range entries {
		total += entry.DurationNS
		if entry.Slow {
			s.SlowCount++
		}
	}
	if s.Total > 0 {
		s.AvgMS = float64(total) / float64(s.Total) / float64(time.Millisecond)
	}
	s.N1Count = len(s.N1Patterns)
	return s
}

func newest(entries []QueryEntry, limit int) []QueryEntry {
	out := make([]QueryEntry, 0, limit)
	for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, entries[i])
	}
	return out
}
func newestSlow(entries []QueryEntry, limit int) []QueryEntry {
	out := make([]QueryEntry, 0, limit)
	for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
		if entries[i].Slow {
			out = append(out, entries[i])
		}
	}
	return out
}

func n1(entries []QueryEntry) []N1Pattern {
	type aggregate struct {
		count   int
		totalNS int64
		latest  QueryEntry
	}
	groups := map[string]aggregate{}
	for _, e := range entries {
		key := normalizeSQL(e.SQL)
		a := groups[key]
		a.count++
		a.totalNS += e.DurationNS
		a.latest = e
		groups[key] = a
	}
	out := make([]N1Pattern, 0)
	for sql, a := range groups {
		if a.count >= 3 {
			out = append(out, N1Pattern{SQL: sql, Count: a.count, TraceID: a.latest.TraceID, Caller: a.latest.Caller, TotalMS: float64(a.totalNS) / float64(time.Millisecond), Timestamp: a.latest.Timestamp})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalMS > out[j].TotalMS })
	return out
}

func normalizeSQL(sql string) string { return strings.Join(strings.Fields(sql), " ") }

// WithTraceID is available to callers that attach a trace ID to GORM contexts.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

type traceIDKey struct{}
