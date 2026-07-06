package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/export"
	"github.com/port-experimental/port-cli/internal/modules/import_module"
)

// Collector gathers live organization snapshots via the export collector.
type Collector struct {
	client *api.Client
}

// NewCollector creates a snapshot collector backed by an API client.
func NewCollector(client *api.Client) *Collector {
	return &Collector{client: client}
}

// Collect fetches a snapshot for orgName using the given plan.
func (c *Collector) Collect(ctx context.Context, orgName string, plan CollectPlan) (*Snapshot, error) {
	collector := export.NewCollector(c.client)
	data, err := collector.Collect(ctx, plan.ExportOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to collect snapshot for org %s: %w", orgName, err)
	}
	return &Snapshot{
		OrgName: orgName,
		Data:    data,
		Metadata: Metadata{
			CollectedAt: time.Now().UTC(),
			Source:      "live",
		},
	}, nil
}

// LoadFromFile loads a snapshot from an export archive or JSON file.
func LoadFromFile(path string) (*Snapshot, error) {
	loader := import_module.NewLoader()
	data, err := loader.LoadData(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot from %s: %w", path, err)
	}
	return &Snapshot{
		OrgName: path,
		Data:    data,
		Metadata: Metadata{
			CollectedAt: time.Now().UTC(),
			Source:      "file",
		},
	}, nil
}
