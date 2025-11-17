# Port API Authentication Guide

## Endpoints That Don't Require Authorization

### `/v1/auth/access_token` (POST)

**This is the ONLY endpoint that doesn't require authorization.**

This endpoint is used to obtain an access token using client credentials. It requires:
- `clientId` in the request body
- `clientSecret` in the request body
- **NO Authorization header needed**

#### Request Example

```bash
curl -X POST https://api.getport.io/v1/auth/access_token \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "your-client-id",
    "clientSecret": "your-client-secret"
  }'
```

#### Response Example

```json
{
  "ok": true,
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600,
  "tokenType": "Bearer"
}
```

#### Test Results

✅ **Endpoint is accessible without authorization**
- Status: 401 (expected with invalid credentials)
- Response: `{"ok":false,"error":"invalid_credentials","message":"Invalid credentials supplied to the generate token API"}`
- This confirms the endpoint structure is correct and doesn't require a Bearer token

## All Other Endpoints Require Authorization

**Every other endpoint in the Port API requires a Bearer token** in the Authorization header:

```
Authorization: Bearer <access_token>
```

### Examples of Protected Endpoints

- `GET /v1/blueprints` - Requires `read:blueprints` permission
- `POST /v1/blueprints` - Requires `create:blueprints` permission
- `GET /v1/blueprints/{id}/entities` - Requires `read:entities` permission
- `GET /v1/teams` - Requires `read:teams` permission
- `GET /v1/users` - Requires `read:users` permission
- And all other endpoints...

## Authentication Flow

1. **Get Token** (no auth required):
   ```bash
   POST /v1/auth/access_token
   Body: { "clientId": "...", "clientSecret": "..." }
   ```

2. **Use Token** (auth required):
   ```bash
   GET /v1/blueprints
   Header: Authorization: Bearer <token>
   ```

## In the CLI

The CLI handles this automatically:

1. When you run any command, it first calls `/v1/auth/access_token` (if token expired)
2. Then uses the token for all subsequent API calls
3. All commands except the auth endpoint itself require valid credentials

## Summary

| Endpoint | Requires Auth? | Notes |
|----------|---------------|-------|
| `POST /v1/auth/access_token` | ❌ No | Uses client credentials in body |
| All other endpoints | ✅ Yes | Requires Bearer token in header |

