# OpenAPI Client Integration Guide

This guide explains how to integrate the generated OpenAPI client with the existing API client.

## Current State

- ✅ Generation script created (`scripts/generate-api.sh`)
- ✅ Makefile target added (`make generate-api`)
- ⏳ Generated code (run `make generate-api` first)
- ⏳ Adapter integration (follow steps below)

## Integration Steps

### Step 1: Generate the Code

```bash
make generate-api
```

This creates:
- `internal/api/generated/types.go` - All API types
- `internal/api/generated/client.go` - Generated HTTP client

### Step 2: Update go.mod

Add the oapi-codegen dependency:

```bash
go get github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
go mod tidy
```

### Step 3: Integration Pattern

The generated client will have methods like:
- `GetBlueprintsWithResponse(ctx)` - Returns response with status code
- `CreateBlueprintWithResponse(ctx, blueprint)` - Creates with response

We need to:
1. Wrap the generated client in our `Client` struct
2. Use our `TokenManager` for authentication
3. Maintain backward compatibility with existing methods

### Step 4: Update client.go

Add to `Client` struct:

```go
type Client struct {
    httpClient     *http.Client
    tokenMgr       *TokenManager
    apiURL         string
    timeout        time.Duration
    generatedClient *generated.ClientWithResponses  // NEW
}
```

Update `NewClient()`:

```go
func NewClient(clientID, clientSecret, apiURL string, timeout time.Duration) *Client {
    // ... existing setup ...
    
    // Create HTTP client with auth transport
    httpClient := &http.Client{
        Timeout: timeout,
        Transport: &authRoundTripper{
            tokenMgr: tokenMgr,
            base:     http.DefaultTransport,
        },
    }
    
    // Create generated client
    generatedClient, _ := generated.NewClientWithResponses(
        apiURL,
        generated.WithHTTPClient(httpClient),
    )
    
    return &Client{
        httpClient:     httpClient,
        tokenMgr:       tokenMgr,
        apiURL:         apiURL,
        timeout:        timeout,
        generatedClient: generatedClient,  // NEW
    }
}
```

### Step 5: Create Auth RoundTripper

Add to `client.go`:

```go
// authRoundTripper adds authentication to requests
type authRoundTripper struct {
    tokenMgr *TokenManager
    base     http.RoundTripper
}

func (a *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    token, err := a.tokenMgr.GetToken()
    if err != nil {
        return nil, fmt.Errorf("failed to get token: %w", err)
    }
    
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
    req.Header.Set("Content-Type", "application/json")
    
    return a.base.RoundTrip(req)
}
```

### Step 6: Update Methods in requests.go

Example for `GetBlueprints()`:

```go
func (c *Client) GetBlueprints(ctx context.Context) ([]Blueprint, error) {
    // Use generated client
    response, err := c.generatedClient.GetBlueprintsWithResponse(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get blueprints: %w", err)
    }
    
    // Check status
    if response.StatusCode() >= 400 {
        return nil, fmt.Errorf("API error: %d - %s", 
            response.StatusCode(), string(response.Body))
    }
    
    // Convert generated types to our interface
    if response.JSON200 == nil || response.JSON200.Blueprints == nil {
        return []Blueprint{}, nil
    }
    
    var blueprints []Blueprint
    for _, bp := range *response.JSON200.Blueprints {
        // Convert generated.Blueprint to map[string]interface{}
        bpMap := convertGeneratedToMap(bp)
        blueprints = append(blueprints, Blueprint(bpMap))
    }
    
    return blueprints, nil
}
```

### Step 7: Add Conversion Helper

Add to `client.go`:

```go
// convertGeneratedToMap converts generated types to map for backward compatibility
func convertGeneratedToMap(v interface{}) map[string]interface{} {
    data, _ := json.Marshal(v)
    var result map[string]interface{}
    json.Unmarshal(data, &result)
    return result
}
```

## Benefits

After integration:
- ✅ Type safety from generated code
- ✅ All OpenAPI endpoints available
- ✅ Backward compatible API
- ✅ Automatic validation
- ✅ Easy to add new endpoints

## Testing

After integration, test:

```bash
go test ./internal/api/...
```

Verify existing commands still work:

```bash
port api blueprints list
port api entities get <blueprint> <entity>
```

## Rollback Plan

If issues occur:
1. The existing `request()` method is still available
2. Can revert methods to use `request()` instead of generated client
3. Generated code doesn't break existing functionality

