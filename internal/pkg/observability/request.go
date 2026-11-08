package observability

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/oklog/ulid/v2"
)

// RequestContextKey is shared by Fiber's request context and UserContext.
type RequestContextKey struct{}

// Request carries bounded diagnostics, never request paths, bodies or user IDs.
// Its identity is generated locally: a reused external trace must not merge
// unrelated requests into an apparent N+1 pattern.
type Request struct {
	id      string
	traceID string
	method  string
	route   atomic.Value
}

func NewRequest(traceID, method string) *Request {
	return &Request{id: ulid.Make().String(), traceID: bounded(traceID, 128), method: bounded(method, 16)}
}

func WithRequest(ctx context.Context, request *Request) context.Context {
	return context.WithValue(ctx, RequestContextKey{}, request)
}

// RequestFields attributes structured logs without reading raw HTTP data.
func RequestFields(ctx context.Context) map[string]any {
	request, _ := ctx.Value(RequestContextKey{}).(*Request)
	if request == nil {
		return nil
	}
	return map[string]any{"request_id": request.id, "trace_id": request.traceID, "method": request.method, "route": request.routeName()}
}

// Finish receives the matched route template after downstream handlers finish.
// Entries keep this small object, not a pooled Fiber context or a callback into it.
func (r *Request) Finish(route string) {
	r.route.Store(bounded(route, 256))
}

func (r *Request) routeName() string {
	route, _ := r.route.Load().(string)
	return route
}

func bounded(value string, limit int) string {
	if len(value) > limit {
		value = value[:limit]
	}
	return strings.Clone(value)
}
