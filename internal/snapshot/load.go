package snapshot

import (
	"fmt"

	"github.com/port-experimental/port-cli/internal/modules/export"
)

// LoadFromFile loads a snapshot from an export JSON or tar.gz file.
func LoadFromFile(path string, load func(string) (*export.Data, error)) (*Snapshot, error) {
	data, err := load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot from %s: %w", path, err)
	}
	return FromData(path, "file", data), nil
}
