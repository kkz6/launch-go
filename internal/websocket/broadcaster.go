package websocket

import "github.com/kkz6/launch-go/internal/pkg/broadcast"

// Broadcaster is an alias for broadcast.Broadcaster for backward compatibility.
// New code should import from internal/pkg/broadcast directly.
//
// Deprecated: Use broadcast.Broadcaster from internal/pkg/broadcast instead.
type Broadcaster = broadcast.Broadcaster

// ModelBroadcaster is an alias for broadcast.ModelBroadcaster for backward compatibility.
// New code should import from internal/pkg/broadcast directly.
//
// Deprecated: Use broadcast.ModelBroadcaster from internal/pkg/broadcast instead.
type ModelBroadcaster = broadcast.ModelBroadcaster

// Ensure Hub implements both interfaces from the canonical package
var _ broadcast.Broadcaster = (*Hub)(nil)
var _ broadcast.ModelBroadcaster = (*Hub)(nil)
