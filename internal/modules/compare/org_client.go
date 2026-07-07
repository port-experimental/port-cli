package compare

import (
	"context"

	"github.com/port-experimental/port-cli/internal/api"
)

// OrgClientFactory creates API clients for live organization fetches.
type OrgClientFactory interface {
	ClientForOrg(ctx context.Context, orgName, clientID, clientSecret, apiURL string) (*api.Client, error)
}
