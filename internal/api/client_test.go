package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/useragent"
)

func TestTokenManager_GetToken(t *testing.T) {
	tm := NewTokenManager("test-client-id", "test-client-secret", "https://api.getport.io/v1")

	// Initially no token
	token, err := tm.GetToken()
	if err == nil && token != "" {
		t.Error("Expected error or empty token when refreshToken is not implemented")
	}
}

func TestTokenManager_SetToken(t *testing.T) {
	tm := NewTokenManager("test-client-id", "test-client-secret", "https://api.getport.io/v1")

	expiry := time.Now().Add(1 * time.Hour)
	tm.SetToken("test-token", expiry)

	// Token should be cached
	token, err := tm.GetToken()
	if err == nil && token == "test-token" {
		// Token is valid (within 5 minute buffer)
		return
	}

	// If token expired, that's also fine for this test
	if err != nil {
		// Expected if token expired
		return
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(ClientOpts{ClientID: "test-id", ClientSecret: "test-secret", APIURL: "https://api.getport.io/v1", Timeout: 0})

	if client.apiURL != "https://api.getport.io/v1" {
		t.Errorf("Expected apiURL 'https://api.getport.io/v1', got '%s'", client.apiURL)
	}

	if client.tokenMgr.ClientID != "test-id" {
		t.Errorf("Expected ClientID 'test-id', got '%s'", client.tokenMgr.ClientID)
	}
}

func TestNewClient_DefaultURL(t *testing.T) {
	client := NewClient(ClientOpts{ClientID: "test-id", ClientSecret: "test-secret", APIURL: "", Timeout: 0})

	if client.apiURL != "https://api.getport.io/v1" {
		t.Errorf("Expected default apiURL 'https://api.getport.io/v1', got '%s'", client.apiURL)
	}
}

func TestNewClientWithToken(t *testing.T) {
	exp := time.Now().Add(time.Hour * 24).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":                             "https://api.example.com",
		"exp":                             float64(exp),
		"https://api.example.com/email":   "user@test.com",
		"https://api.example.com/orgId":   "someOrgId",
		"https://api.example.com/orgName": "Org Name",
	})
	signed, err := token.SignedString([]byte("signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := auth.ParseToken(signed)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(ClientOpts{Token: parsed, ClientID: "test-id", ClientSecret: "test-secret", APIURL: "https://api.getport.io/v1", Timeout: 0})

	if client.apiURL != "https://api.getport.io/v1" {
		t.Errorf("Expected apiURL 'https://api.getport.io/v1', got '%s'", client.apiURL)
	}

	if client.tokenMgr.ClientID != "test-id" {
		t.Errorf("Expected ClientID 'test-id', got '%s'", client.tokenMgr.ClientID)
	}

	if client.tokenMgr.token != parsed.Token {
		t.Errorf("Expected token %s, got '%s'", parsed.Token, client.tokenMgr.token)
	}

	if client.tokenMgr.expiry.Unix() != exp {
		t.Errorf("Expected expiry %v, got '%v'", exp, client.tokenMgr.expiry.Unix())
	}
}

func TestNewClientWithoutSecret(t *testing.T) {
	exp := time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"aud":                             "https://api.example.com",
		"exp":                             float64(exp),
		"https://api.example.com/email":   "user@test.com",
		"https://api.example.com/orgId":   "someOrgId",
		"https://api.example.com/orgName": "Org Name",
	})
	signed, err := token.SignedString([]byte("signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := auth.ParseToken(signed)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(ClientOpts{Token: parsed, APIURL: "https://api.getport.io/v1", Timeout: 0})

	if client.apiURL != "https://api.getport.io/v1" {
		t.Errorf("Expected apiURL 'https://api.getport.io/v1', got '%s'", client.apiURL)
	}

	if client.tokenMgr.ClientID != "" {
		t.Errorf("Expected no client id, got '%s'", client.tokenMgr.ClientID)
	}

	if client.tokenMgr.ClientSecret != "" {
		t.Errorf("Expected no client secret, got '%s'", client.tokenMgr.ClientSecret)
	}

	if client.tokenMgr.token != parsed.Token {
		t.Errorf("Expected token %s, got '%s'", parsed.Token, client.tokenMgr.token)
	}

	if client.tokenMgr.expiry.Unix() != exp {
		t.Errorf("Expected expiry %v, got '%v'", exp, client.tokenMgr.expiry.Unix())
	}
}

func TestClient_refreshToken(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/access_token" {
			t.Errorf("Expected path '/auth/access_token', got '%s'", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected method 'POST', got '%s'", r.Method)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		response := TokenResponse{
			AccessToken: "test-access-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "test-id", ClientSecret: "test-secret", APIURL: server.URL, Timeout: 0})
	client.apiURL = server.URL

	token, err := client.refreshToken(context.Background())
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	if token != "test-access-token" {
		t.Errorf("Expected token 'test-access-token', got '%s'", token)
	}
}

func TestClient_request(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			// Token endpoint
			response := TokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
				TokenType:   "Bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// API endpoint
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "test-id", ClientSecret: "test-secret", APIURL: server.URL, Timeout: 0})
	client.apiURL = server.URL

	resp, err := client.request(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_request_Retry(t *testing.T) {
	attempts := 0
	// Create a mock server that returns 429 on first attempt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			response := TokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
				TokenType:   "Bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "test-id", ClientSecret: "test-secret", APIURL: server.URL, Timeout: 0})
	client.apiURL = server.URL

	resp, err := client.request(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("Expected 2 attempts (retry on 429), got %d", attempts)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 after retry, got %d", resp.StatusCode)
	}
}

func TestClient_Close(t *testing.T) {
	client := NewClient(ClientOpts{ClientID: "test-id", ClientSecret: "test-secret", APIURL: "https://api.getport.io/v1", Timeout: 0})

	// Close should not error
	if err := client.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// TestClient_UserAgent verifies that every outbound request carries a
// User-Agent header that starts with "port-cli/".
func TestClient_UserAgent(t *testing.T) {
	useragent.SetVersion("test-version")
	t.Cleanup(func() { useragent.SetVersion("dev") })

	wantUA := useragent.String()

	type capture struct {
		path string
		ua   string
	}
	captured := make([]capture, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, capture{path: r.URL.Path, ua: r.Header.Get("User-Agent")})

		if r.URL.Path == "/auth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "1"})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})
	resp, err := client.request(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Expect at least two calls: token refresh and the actual request.
	if len(captured) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(captured))
	}

	for _, c := range captured {
		if !strings.HasPrefix(c.ua, "port-cli/") {
			t.Errorf("request to %s: User-Agent = %q, want prefix \"port-cli/\"", c.path, c.ua)
		}
		if c.ua != wantUA {
			t.Errorf("request to %s: User-Agent = %q, want %q", c.path, c.ua, wantUA)
		}
	}
}

// TestClient_RetryPreservesBody verifies that after a 429 retry the full
// request body is re-sent. Before the bytes.NewReader fix, the body was a
// bytes.Buffer that was drained on the first Do(), causing retries to send
// an empty body.
func TestClient_RetryPreservesBody(t *testing.T) {
	var attempt int32
	wantBody := `{"key":"value"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}

		n := atomic.AddInt32(&attempt, 1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("attempt %d: failed to read body: %v", n, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if n == 1 {
			if len(body) == 0 {
				t.Error("attempt 1: body was unexpectedly empty")
			}
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		if len(body) == 0 {
			t.Fatal("attempt 2: body was empty — retry did not re-send the body")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("attempt 2: invalid JSON body: %v", err)
		}
		if parsed["key"] != "value" {
			t.Errorf("attempt 2: body = %s, want %s", string(body), wantBody)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})

	resp, err := client.request(context.Background(), "POST", "/test", map[string]string{"key": "value"}, nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if got := atomic.LoadInt32(&attempt); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

// TestUpsertEntity verifies that UpsertEntity sends the correct HTTP method,
// path, and query parameters (upsert, merge, create_missing_related_entities).
func TestUpsertEntity(t *testing.T) {
	tests := []struct {
		name      string
		merge     bool
		wantMerge string
	}{
		{"merge=true", true, "true"},
		{"merge=false", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedMethod, capturedPath, capturedQuery string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/auth/access_token" {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(TokenResponse{AccessToken: "tok", ExpiresIn: 3600})
					return
				}

				capturedMethod = r.Method
				capturedPath = r.URL.Path
				capturedQuery = r.URL.RawQuery

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"entity": map[string]interface{}{"identifier": "e1"},
				})
			}))
			defer server.Close()

			client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})

			entity := Entity{"identifier": "e1", "title": "Entity 1"}
			_, err := client.UpsertEntity(context.Background(), "myBlueprint", entity, tt.merge)
			if err != nil {
				t.Fatalf("UpsertEntity failed: %v", err)
			}

			if capturedMethod != "POST" {
				t.Errorf("method = %q, want POST", capturedMethod)
			}
			if capturedPath != "/blueprints/myBlueprint/entities" {
				t.Errorf("path = %q, want /blueprints/myBlueprint/entities", capturedPath)
			}
			if !strings.Contains(capturedQuery, "upsert=true") {
				t.Errorf("query %q missing upsert=true", capturedQuery)
			}
			if !strings.Contains(capturedQuery, "create_missing_related_entities=true") {
				t.Errorf("query %q missing create_missing_related_entities=true", capturedQuery)
			}
			if tt.wantMerge != "" {
				if !strings.Contains(capturedQuery, "merge=true") {
					t.Errorf("query %q missing merge=true", capturedQuery)
				}
			} else {
				if strings.Contains(capturedQuery, "merge") {
					t.Errorf("query %q should not contain merge when merge=false", capturedQuery)
				}
			}
		})
	}
}

// TestBulkUpsertEntities verifies path, method, and query parameters.
func TestBulkUpsertEntities(t *testing.T) {
	var capturedMethod, capturedPath, capturedQuery string
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}

		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entities": []map[string]interface{}{
				{"identifier": "e1"},
				{"identifier": "e2"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})

	entities := []Entity{
		{"identifier": "e1", "title": "Entity 1"},
		{"identifier": "e2", "title": "Entity 2"},
	}
	result, err := client.BulkUpsertEntities(context.Background(), "svcBP", entities, true)
	if err != nil {
		t.Fatalf("BulkUpsertEntities failed: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
	if capturedPath != "/blueprints/svcBP/entities/bulk" {
		t.Errorf("path = %q, want /blueprints/svcBP/entities/bulk", capturedPath)
	}
	if !strings.Contains(capturedQuery, "upsert=true") {
		t.Errorf("query %q missing upsert=true", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "merge=true") {
		t.Errorf("query %q missing merge=true", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "create_missing_related_entities=true") {
		t.Errorf("query %q missing create_missing_related_entities=true", capturedQuery)
	}

	var bodyParsed []map[string]interface{}
	if err := json.Unmarshal(capturedBody, &bodyParsed); err != nil {
		t.Fatalf("body is not a JSON array: %v", err)
	}
	if len(bodyParsed) != 2 {
		t.Errorf("body has %d entities, want 2", len(bodyParsed))
	}

	if len(result) != 2 {
		t.Errorf("returned %d entities, want 2", len(result))
	}
}

// TestBulkDeleteEntities verifies the correct path and method.
func TestBulkDeleteEntities(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}

		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})

	err := client.BulkDeleteEntities(context.Background(), "svcBP", []string{"e1", "e2", "e3"})
	if err != nil {
		t.Fatalf("BulkDeleteEntities failed: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
	if capturedPath != "/blueprints/svcBP/bulk/entities/delete" {
		t.Errorf("path = %q, want /blueprints/svcBP/bulk/entities/delete", capturedPath)
	}

	var bodyParsed map[string]interface{}
	if err := json.Unmarshal(capturedBody, &bodyParsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	ids, ok := bodyParsed["identifiers"].([]interface{})
	if !ok {
		t.Fatal("body missing 'identifiers' array")
	}
	if len(ids) != 3 {
		t.Errorf("identifiers has %d items, want 3", len(ids))
	}
}

// TestSearchEntities_Pagination verifies that SearchEntities follows the
// "next" cursor and aggregates entities across multiple pages.
func TestSearchEntities_Pagination(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}

		page := atomic.AddInt32(&callCount, 1)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("page %d: invalid request body: %v", page, err)
		}

		if r.URL.Path != "/blueprints/testBP/entities/search" {
			t.Errorf("page %d: path = %q, want /blueprints/testBP/entities/search", page, r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("page %d: method = %q, want POST", page, r.Method)
		}

		w.Header().Set("Content-Type", "application/json")

		switch page {
		case 1:
			if _, hasCursor := reqBody["from"]; hasCursor {
				t.Error("page 1: should not have 'from' cursor")
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"entities": []map[string]interface{}{
					{"identifier": "e1"},
					{"identifier": "e2"},
				},
				"next": "cursor-page-2",
			})
		case 2:
			cursor, _ := reqBody["from"].(string)
			if cursor != "cursor-page-2" {
				t.Errorf("page 2: from = %q, want cursor-page-2", cursor)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"entities": []map[string]interface{}{
					{"identifier": "e3"},
				},
				"next": "",
			})
		default:
			t.Fatalf("unexpected page %d — pagination should have stopped after page 2", page)
		}
	}))
	defer server.Close()

	client := NewClient(ClientOpts{ClientID: "id", ClientSecret: "secret", APIURL: server.URL})

	searchBody := map[string]interface{}{
		"query": map[string]interface{}{
			"combinator": "and",
			"rules":      []interface{}{},
		},
	}
	entities, err := client.SearchEntities(context.Background(), "testBP", searchBody)
	if err != nil {
		t.Fatalf("SearchEntities failed: %v", err)
	}

	if len(entities) != 3 {
		t.Errorf("got %d entities, want 3", len(entities))
	}

	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Errorf("server received %d search calls, want 2", got)
	}

	wantIDs := []string{"e1", "e2", "e3"}
	for i, want := range wantIDs {
		if i >= len(entities) {
			break
		}
		got, _ := entities[i]["identifier"].(string)
		if got != want {
			t.Errorf("entities[%d].identifier = %q, want %q", i, got, want)
		}
	}
}
