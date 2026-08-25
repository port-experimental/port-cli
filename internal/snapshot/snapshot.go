package snapshot

import (
	"time"

	"github.com/port-experimental/port-cli/internal/modules/export"
)

// Metadata describes how and when a snapshot was produced.
type Metadata struct {
	CollectedAt time.Time
	Source      string // "live" or "file"
}

// Snapshot is a point-in-time view of organization data.
type Snapshot struct {
	OrgName  string
	Data     *export.Data
	Metadata Metadata
}

// FromData wraps existing export data as a snapshot.
func FromData(orgName, source string, data *export.Data) *Snapshot {
	return &Snapshot{
		OrgName: orgName,
		Data:    data,
		Metadata: Metadata{
			CollectedAt: time.Now().UTC(),
			Source:      source,
		},
	}
}
