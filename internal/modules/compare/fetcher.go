package compare

import (
	"context"
	"strings"

	"github.com/port-experimental/port-cli/internal/modules/import_module"
	"github.com/port-experimental/port-cli/internal/snapshot"
)

// detectInputType determines if input is an org name or file path.
func detectInputType(input string) string {
	if strings.HasSuffix(input, ".tar.gz") ||
		strings.HasSuffix(input, ".json") ||
		strings.HasPrefix(input, "/") ||
		strings.HasPrefix(input, "./") ||
		strings.HasPrefix(input, "../") {
		return "file"
	}
	return "org"
}

// Fetcher loads organization data from live orgs or export files.
type Fetcher struct {
	orgClients OrgClientFactory
}

// NewFetcher creates a new fetcher.
func NewFetcher(orgClients OrgClientFactory) *Fetcher {
	return &Fetcher{
		orgClients: orgClients,
	}
}

// FetchOptions contains options for fetching org data.
type FetchOptions struct {
	OrgName          string
	FilePath         string
	ClientID         string
	ClientSecret     string
	APIUrl           string
	IncludeResources []string
}

// Fetch loads organization data from either a live org or export file.
func (f *Fetcher) Fetch(ctx context.Context, opts FetchOptions) (*OrgData, error) {
	var input string
	if opts.FilePath != "" {
		input = opts.FilePath
	} else {
		input = opts.OrgName
	}

	inputType := detectInputType(input)

	if inputType == "file" {
		return f.fetchFromFile(ctx, input)
	}
	return f.fetchFromOrg(ctx, opts)
}

// fetchFromFile loads data from an export file.
func (f *Fetcher) fetchFromFile(ctx context.Context, filePath string) (*OrgData, error) {
	snap, err := snapshot.LoadFromFile(filePath, import_module.NewLoader().LoadData)
	if err != nil {
		return nil, err
	}
	return &OrgData{
		Name: snap.OrgName,
		Data: snap.Data,
	}, nil
}

// fetchFromOrg loads data from a live Port organization.
func (f *Fetcher) fetchFromOrg(ctx context.Context, opts FetchOptions) (*OrgData, error) {
	client, err := f.orgClients.ClientForOrg(ctx, opts.OrgName, opts.ClientID, opts.ClientSecret, opts.APIUrl)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Collect org snapshot
	plan := snapshot.CompareCollectPlan(opts.IncludeResources)
	snapCollector := snapshot.NewCollector(client)
	snap, err := snapCollector.Collect(ctx, opts.OrgName, plan)
	if err != nil {
		return nil, err
	}

	return &OrgData{
		Name: snap.OrgName,
		Data: snap.Data,
	}, nil
}
