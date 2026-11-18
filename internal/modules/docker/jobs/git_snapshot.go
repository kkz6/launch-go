package jobs

import "github.com/kkz6/launch-go/internal/modules/git/access"

func firstSnapshot(snapshots []*access.Snapshot) *access.Snapshot {
	if len(snapshots) == 0 {
		return nil
	}
	return snapshots[0]
}

func dependencySnapshots(snapshots []*access.Snapshot) []access.Snapshot {
	var out []access.Snapshot
	if len(snapshots) < 2 {
		return out
	}
	for _, snapshot := range snapshots[1:] {
		if snapshot != nil {
			out = append(out, *snapshot)
		}
	}
	return out
}
