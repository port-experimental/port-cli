package commands

import (
	"context"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/modules/compare"
)

type runtimeOrgClientFactory struct {
	rt *Runtime
}

// OrgClientFactory returns a compare.OrgClientFactory backed by this runtime.
func (r *Runtime) OrgClientFactory() compare.OrgClientFactory {
	return runtimeOrgClientFactory{rt: r}
}

func (f runtimeOrgClientFactory) ClientForOrg(ctx context.Context, orgName, clientID, clientSecret, apiURL string) (*api.Client, error) {
	if clientID == "" && clientSecret == "" && apiURL == "" {
		clientID = f.rt.Flags.ClientID
		clientSecret = f.rt.Flags.ClientSecret
		apiURL = f.rt.Flags.APIURL
	}
	return f.rt.ClientForOrgWithOverrides(ctx, orgName, clientID, clientSecret, apiURL)
}
