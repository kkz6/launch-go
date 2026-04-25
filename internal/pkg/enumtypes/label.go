package enumtypes

// Label looks v up in labels and returns the matching display string, or
// fallback if v isn't in the map. Use it to collapse the boilerplate that
// every enum's Label() method otherwise repeats:
//
//	// Before: rebuilt map on every call, 6 lines per enum
//	func (s ServerStatus) Label() string {
//	    labels := map[ServerStatus]string{
//	        ServerStatusNew: "Connecting",
//	        ServerStatusRunning: "Running",
//	        // ...
//	    }
//	    if l, ok := labels[s]; ok {
//	        return l
//	    }
//	    return "Unknown"
//	}
//
//	// After: map declared once at package scope, method is one line
//	var serverStatusLabels = map[ServerStatus]string{
//	    ServerStatusNew: "Connecting",
//	    ServerStatusRunning: "Running",
//	    // ...
//	}
//
//	func (s ServerStatus) Label() string {
//	    return enumtypes.Label(s, serverStatusLabels, "Unknown")
//	}
//
// Lifting the map to package scope also avoids re-allocating on every call —
// a small but real win for any code that loops over a list of enum values.
//
// Use a fallback of `string(v)` when the desired behaviour for unknown
// values is to surface the raw enum string (handy when forwarding values
// from external systems through a label-only UI).
func Label[T comparable](v T, labels map[T]string, fallback string) string {
	if l, ok := labels[v]; ok {
		return l
	}
	return fallback
}
