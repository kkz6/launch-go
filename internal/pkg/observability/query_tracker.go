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

const (
	defaultCapacity = 200
	n1Threshold     = 3
)

type QueryEntry struct {
	SQL        string    `json:"sql"`
	DurationNS int64     `json:"duration_ns"`
	Rows       int64     `json:"rows"`
	Caller     string    `json:"caller"`
	TraceID    string    `json:"trace_id,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	Method     string    `json:"method,omitempty"`
	Route      string    `json:"route,omitempty"`
	Operation  string    `json:"operation"`
	Slow       bool      `json:"slow"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	request    *Request
	dialect    string
}

type N1Pattern struct {
	SQL       string    `json:"sql"`
	Count     int       `json:"count"`
	TraceID   string    `json:"trace_id,omitempty"`
	RequestID string    `json:"request_id"`
	Method    string    `json:"method,omitempty"`
	Route     string    `json:"route,omitempty"`
	Operation string    `json:"operation"`
	Caller    string    `json:"caller"`
	TotalMS   float64   `json:"total_ms"`
	AvgMS     float64   `json:"avg_ms"`
	Timestamp time.Time `json:"timestamp"`
}

type QuerySummary struct {
	Total           int          `json:"total"`
	SlowCount       int          `json:"slow_count"`
	N1Count         int          `json:"n1_count"`
	AvgMS           float64      `json:"avg_ms"`
	Recent          []QueryEntry `json:"recent"`
	SlowQueries     []QueryEntry `json:"slow_queries"`
	N1Patterns      []N1Pattern  `json:"n1_patterns"`
	Capacity        int          `json:"capacity"`
	N1Threshold     int          `json:"n1_threshold"`
	SlowThresholdMS float64      `json:"slow_threshold_ms"`
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
	_ = db.Callback().Row().Before("gorm:row").Register("observability:row-start", start)
	_ = db.Callback().Row().After("gorm:row").Register("observability:row", track)
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
	var duration time.Duration
	if !started.IsZero() {
		duration = time.Since(started)
	}
	e := QueryEntry{SQL: sql, DurationNS: duration.Nanoseconds(), Rows: tx.Statement.RowsAffected, Timestamp: time.Now(), dialect: tx.Dialector.Name()}
	if request, ok := tx.Statement.Context.Value(RequestContextKey{}).(*Request); ok {
		e.request = request
		e.RequestID, e.TraceID, e.Method = request.id, request.traceID, request.method
	}
	if traceID, _ := tx.Statement.Context.Value(traceIDKey{}).(string); traceID != "" {
		e.TraceID = traceID
	}
	if tx.Error != nil {
		e.Error = QueryError(tx.Error)
	}
	e.Caller = caller()
	return e
}

func caller() string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.Function, "gorm.io/") && !strings.Contains(frame.Function, "/observability.") && !strings.Contains(frame.Function, "/pkg/repository.") {
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
	entry.SQL = RedactSQL(entry.SQL, entry.dialect)
	if entry.SQL == "" {
		return
	}
	entry.Operation = strings.ToUpper(strings.Fields(entry.SQL)[0])
	entry.TraceID = bounded(entry.TraceID, 128)
	entry.RequestID = bounded(entry.RequestID, 128)
	entry.Method = bounded(entry.Method, 16)
	entry.Route = bounded(entry.Route, 256)
	entry.Caller = bounded(entry.Caller, 512)
	entry.Error = safeErrorCategory(entry.Error)
	entry.DurationNS = max(entry.DurationNS, 0)
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
			result = append(result, withRequestRoute(t.entries[i]))
		}
	}
	return result
}

func (t *Tracker) Summary() QuerySummary {
	t.mu.RLock()
	entries := append([]QueryEntry(nil), t.entries...)
	t.mu.RUnlock()
	for i := range entries {
		entries[i] = withRequestRoute(entries[i])
	}
	s := QuerySummary{Recent: newest(entries, 20), SlowQueries: newestSlow(entries, 20), N1Patterns: n1(entries), Capacity: t.capacity, N1Threshold: n1Threshold, SlowThresholdMS: float64(t.threshold) / float64(time.Millisecond)}
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
	type patternKey struct {
		requestID string
		sql       string
		caller    string
	}
	type aggregate struct {
		count   int
		totalNS int64
		latest  QueryEntry
	}
	groups := map[patternKey]aggregate{}
	for _, e := range entries {
		// Repeated writes, failures and unrelated/background work are not N+1
		// evidence. Keep separate call sites separate even within one request.
		if e.RequestID == "" || e.Operation != "SELECT" || e.Error != "" {
			continue
		}
		key := patternKey{requestID: e.RequestID, sql: e.SQL, caller: e.Caller}
		a := groups[key]
		a.count++
		a.totalNS += e.DurationNS
		a.latest = e
		groups[key] = a
	}
	out := make([]N1Pattern, 0)
	for key, a := range groups {
		if a.count >= n1Threshold {
			totalMS := float64(a.totalNS) / float64(time.Millisecond)
			out = append(out, N1Pattern{SQL: key.sql, Count: a.count, TraceID: a.latest.TraceID, RequestID: key.requestID, Method: a.latest.Method, Route: a.latest.Route, Operation: a.latest.Operation, Caller: a.latest.Caller, TotalMS: totalMS, AvgMS: totalMS / float64(a.count), Timestamp: a.latest.Timestamp})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalMS != out[j].TotalMS {
			return out[i].TotalMS > out[j].TotalMS
		}
		if out[i].RequestID != out[j].RequestID {
			return out[i].RequestID < out[j].RequestID
		}
		if out[i].SQL != out[j].SQL {
			return out[i].SQL < out[j].SQL
		}
		return out[i].Caller < out[j].Caller
	})
	return out
}

func withRequestRoute(entry QueryEntry) QueryEntry {
	if entry.request != nil {
		entry.Route = entry.request.routeName()
	}
	return entry
}

// WithTraceID is available to callers that attach a trace ID to GORM contexts.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

type traceIDKey struct{}
