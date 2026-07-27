package observability

import (
	"database/sql"
	"runtime"
	"time"
)

type RuntimeStats struct {
	Goroutines    int     `json:"goroutines"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	HeapInuseMB   float64 `json:"heap_inuse_mb"`
	HeapObjectsK  float64 `json:"heap_objects_k"`
	StackInuseMB  float64 `json:"stack_inuse_mb"`
	SysMemMB      float64 `json:"sys_mem_mb"`
	NumGC         uint32  `json:"num_gc"`
	LastGCPauseMS float64 `json:"last_gc_pause_ms"`
	GCCPUPercent  float64 `json:"gc_cpu_percent"`
	NumCPU        int     `json:"num_cpu"`
	Uptime        string  `json:"uptime"`
}
type DBPoolStats struct {
	MaxOpen      int    `json:"max_open"`
	Open         int    `json:"open"`
	InUse        int    `json:"in_use"`
	Idle         int    `json:"idle"`
	WaitCount    int64  `json:"wait_count"`
	WaitDuration string `json:"wait_duration"`
}
type Snapshot struct {
	Runtime   RuntimeStats `json:"runtime"`
	DBPool    DBPoolStats  `json:"db_pool"`
	Queries   QuerySummary `json:"queries"`
	Uptime    string       `json:"uptime"`
	Timestamp time.Time    `json:"timestamp"`
}

var startedAt = time.Now()

func NewSnapshot(db *sql.DB, tracker *Tracker) Snapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	uptime := time.Since(startedAt)
	s := Snapshot{Runtime: RuntimeStats{Goroutines: runtime.NumGoroutine(), HeapAllocMB: mb(mem.HeapAlloc), HeapInuseMB: mb(mem.HeapInuse), HeapObjectsK: float64(mem.HeapObjects) / 1000, StackInuseMB: mb(mem.StackInuse), SysMemMB: mb(mem.Sys), NumGC: mem.NumGC, LastGCPauseMS: float64(mem.PauseNs[(mem.NumGC+255)%256]) / float64(time.Millisecond), GCCPUPercent: mem.GCCPUFraction * 100, NumCPU: runtime.NumCPU(), Uptime: uptime.Round(time.Second).String()}, Uptime: uptime.Round(time.Second).String(), Timestamp: time.Now()}
	if db != nil {
		st := db.Stats()
		s.DBPool = DBPoolStats{st.MaxOpenConnections, st.OpenConnections, st.InUse, st.Idle, st.WaitCount, st.WaitDuration.String()}
	}
	if tracker != nil {
		s.Queries = tracker.Summary()
	}
	return s
}
func mb(v uint64) float64 { return float64(v) / 1024 / 1024 }
