package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	"github.com/spf13/cobra"
)

func writeTestConfig(t *testing.T, dir string, content string) string {
	t.Helper()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return configPath
}

func runtimeWithConfig(t *testing.T, configPath string, flags GlobalFlags) *Runtime {
	t.Helper()
	flags.ConfigFile = configPath
	ctx := WithGlobalFlags(context.Background(), flags)
	return NewRuntime(ctx)
}

func TestRuntime_LoadOrg_resolvesNamedOrg(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: staging
organizations:
  staging:
    client_id: staging-id
    client_secret: staging-secret
    api_url: https://api.staging.example/v1
  production:
    client_id: prod-id
    client_secret: prod-secret
    api_url: https://api.prod.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	_, orgConfig, useOrg, err := rt.LoadOrg(context.Background(), "production")
	if err != nil {
		t.Fatalf("LoadOrg failed: %v", err)
	}
	if useOrg != "production" {
		t.Errorf("expected org production, got %q", useOrg)
	}
	if orgConfig.ClientID != "prod-id" {
		t.Errorf("expected client_id prod-id, got %q", orgConfig.ClientID)
	}
	if orgConfig.APIURL != "https://api.prod.example/v1" {
		t.Errorf("expected production api_url, got %q", orgConfig.APIURL)
	}
}

func TestRuntime_LoadOrg_usesDefaultOrg(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: staging
organizations:
  staging:
    client_id: staging-id
    client_secret: staging-secret
    api_url: https://api.staging.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	_, orgConfig, useOrg, err := rt.LoadOrg(context.Background(), "")
	if err != nil {
		t.Fatalf("LoadOrg failed: %v", err)
	}
	if useOrg != "staging" {
		t.Errorf("expected default org staging, got %q", useOrg)
	}
	if orgConfig.ClientID != "staging-id" {
		t.Errorf("expected client_id staging-id, got %q", orgConfig.ClientID)
	}
}

func TestRuntime_LoadOrg_appliesCLIOverrides(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: default
organizations:
  default:
    client_id: default-id
    client_secret: default-secret
    api_url: https://api.default.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{
		ClientID:     "override-id",
		ClientSecret: "override-secret",
	})
	_, orgConfig, useOrg, err := rt.LoadOrg(context.Background(), "override")
	if err != nil {
		t.Fatalf("LoadOrg failed: %v", err)
	}
	if useOrg != "override" {
		t.Errorf("expected org override, got %q", useOrg)
	}
	if orgConfig.ClientID != "override-id" {
		t.Errorf("expected override client_id, got %q", orgConfig.ClientID)
	}
}

func TestRuntime_LoadOrg_missingOrgFails(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: staging
organizations:
  staging:
    client_id: staging-id
    client_secret: staging-secret
    api_url: https://api.staging.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	_, _, _, err := rt.LoadOrg(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing org")
	}
}

func TestRuntime_ClientForOrg_usesOrgAPIURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, t.TempDir(), `default_org: test
organizations:
  test:
    client_id: test-id
    client_secret: test-secret
    api_url: `+server.URL+`
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	client, useOrg, err := rt.ClientForOrg(context.Background(), "test")
	if err != nil {
		t.Fatalf("ClientForOrg failed: %v", err)
	}
	defer client.Close()

	if useOrg != "test" {
		t.Errorf("expected org test, got %q", useOrg)
	}

	_, err = client.Request(context.Background(), api.RequestParams{Method: "GET", Endpoint: "/health"})
	if err != nil {
		t.Fatalf("client request failed: %v", err)
	}
}

func TestRuntime_ClientForOrg_ignoresMissingStoredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"blueprints":[]}`))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, t.TempDir(), `default_org: test
organizations:
  test:
    client_id: test-id
    client_secret: test-secret
    api_url: `+server.URL+`
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	client, _, err := rt.ClientForOrg(context.Background(), "test")
	if err != nil {
		t.Fatalf("ClientForOrg failed: %v", err)
	}
	defer client.Close()

	_, err = client.GetBlueprints(context.Background())
	if err != nil {
		t.Fatalf("GetBlueprints failed without stored token: %v", err)
	}
}

func TestRuntime_SourceTargetClients_resolvesBothOrgs(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer sourceServer.Close()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer targetServer.Close()

	configPath := writeTestConfig(t, t.TempDir(), `default_org: source
organizations:
  source:
    client_id: source-id
    client_secret: source-secret
    api_url: `+sourceServer.URL+`
  target:
    client_id: target-id
    client_secret: target-secret
    api_url: `+targetServer.URL+`
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	sourceClient, targetClient, err := rt.SourceTargetClients(context.Background(), "source", "target")
	if err != nil {
		t.Fatalf("SourceTargetClients failed: %v", err)
	}
	defer sourceClient.Close()
	defer targetClient.Close()

	if _, err := sourceClient.Request(context.Background(), api.RequestParams{Method: "GET", Endpoint: "/source"}); err != nil {
		t.Fatalf("source client request failed: %v", err)
	}
	if _, err := targetClient.Request(context.Background(), api.RequestParams{Method: "GET", Endpoint: "/target"}); err != nil {
		t.Fatalf("target client request failed: %v", err)
	}
}

func TestRuntime_SourceTargetClients_missingTargetFails(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: source
organizations:
  source:
    client_id: source-id
    client_secret: source-secret
    api_url: https://api.source.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	_, _, err := rt.SourceTargetClients(context.Background(), "source", "missing")
	if err == nil {
		t.Fatal("expected error for missing target org")
	}
}

func TestRuntime_ClientForOrg_usesStoredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer stored-token" {
			t.Errorf("expected stored token, got %q", authHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, `default_org: test
organizations:
  test:
    client_id: test-id
    client_secret: test-secret
    api_url: `+server.URL+`
`)

	manager := config.NewConfigManager(configPath)
	token := &auth.Token{
		Token: "stored-token",
		Claims: auth.Claims{
			Expiry: time.Now().Add(time.Hour),
		},
	}
	if err := manager.StoreToken("test", token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	client, _, err := rt.ClientForOrg(context.Background(), "test")
	if err != nil {
		t.Fatalf("ClientForOrg failed: %v", err)
	}
	defer client.Close()

	if _, err := client.Request(context.Background(), api.RequestParams{Method: "GET", Endpoint: "/auth-check"}); err != nil {
		t.Fatalf("request failed: %v", err)
	}
}

func TestRuntime_CredentialsForBaseOrg_resolvesDefaultOrg(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: staging
organizations:
  staging:
    client_id: staging-id
    client_secret: staging-secret
    api_url: https://api.staging.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	token, orgConfig, useOrg, err := rt.CredentialsForBaseOrg(context.Background(), "")
	if err != nil {
		t.Fatalf("CredentialsForBaseOrg failed: %v", err)
	}
	_ = token
	if useOrg != "" {
		t.Errorf("expected empty org name passthrough, got %q", useOrg)
	}
	if orgConfig.ClientID != "staging-id" {
		t.Errorf("expected staging-id, got %q", orgConfig.ClientID)
	}
	// Token may be nil when auth cannot reach the configured API URL; org config
	// resolution is the contract under test here.
}

func TestRuntime_CredentialsForTargetOrg_fallsBackToBaseFlags(t *testing.T) {
	configPath := writeTestConfig(t, t.TempDir(), `default_org: target
organizations:
  target:
    client_id: target-id
    client_secret: target-secret
    api_url: https://api.target.example/v1
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{
		ClientID:     "base-id",
		ClientSecret: "base-secret",
		APIURL:       "https://api.base.example/v1",
	})
	_, orgConfig, _, err := rt.CredentialsForTargetOrg(context.Background(), "target")
	if err != nil {
		t.Fatalf("CredentialsForTargetOrg failed: %v", err)
	}
	// When target credential flags are empty, base flags are used as the target
	// override fallback (see loadTargetOrgConfig).
	if orgConfig.ClientID != "base-id" {
		t.Errorf("expected base-id fallback, got %q", orgConfig.ClientID)
	}

	rtOverride := runtimeWithConfig(t, configPath, GlobalFlags{
		TargetClientID:     "override-id",
		TargetClientSecret: "override-secret",
	})
	_, orgConfig, _, err = rtOverride.CredentialsForTargetOrg(context.Background(), "")
	if err != nil {
		t.Fatalf("CredentialsForTargetOrg with overrides failed: %v", err)
	}
	if orgConfig.ClientID != "override-id" {
		t.Errorf("expected override-id, got %q", orgConfig.ClientID)
	}
}

func TestRuntime_ClientForOrgWithOverrides_usesExplicitCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, t.TempDir(), `default_org: test
organizations:
  test:
    client_id: file-id
    client_secret: file-secret
    api_url: `+server.URL+`
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	client, err := rt.ClientForOrgWithOverrides(context.Background(), "test", "override-id", "override-secret", server.URL)
	if err != nil {
		t.Fatalf("ClientForOrgWithOverrides failed: %v", err)
	}
	defer client.Close()

	if _, err := client.Request(context.Background(), api.RequestParams{Method: "GET", Endpoint: "/health"}); err != nil {
		t.Fatalf("client request failed: %v", err)
	}
}

func TestRuntime_OrgClientFactory_delegatesToRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, t.TempDir(), `default_org: test
organizations:
  test:
    client_id: test-id
    client_secret: test-secret
    api_url: `+server.URL+`
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{})
	factory := rt.OrgClientFactory()
	client, err := factory.ClientForOrg(context.Background(), "test", "", "", "")
	if err != nil {
		t.Fatalf("OrgClientFactory.ClientForOrg failed: %v", err)
	}
	defer client.Close()
}

func TestRuntime_OrgClientFactory_fallsBackToGlobalFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, t.TempDir(), `default_org: test
organizations:
  test:
    client_id: file-id
    client_secret: file-secret
    api_url: `+server.URL+`
`)

	rt := runtimeWithConfig(t, configPath, GlobalFlags{
		ClientID:     "global-id",
		ClientSecret: "global-secret",
		APIURL:       server.URL,
	})
	factory := rt.OrgClientFactory()
	client, err := factory.ClientForOrg(context.Background(), "test", "", "", "")
	if err != nil {
		t.Fatalf("OrgClientFactory.ClientForOrg failed: %v", err)
	}
	defer client.Close()

	if _, err := client.Request(context.Background(), api.RequestParams{Method: "GET", Endpoint: "/health"}); err != nil {
		t.Fatalf("client request failed: %v", err)
	}
}

func TestGetOrRefreshCommandToken_delegatesToSharedHelper(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTestConfig(t, dir, `default_org: test
organizations:
  test:
    client_id: test-id
    client_secret: test-secret
    api_url: https://api.example/v1
`)

	manager := config.NewConfigManager(configPath)
	token := &auth.Token{
		Token: "compat-token",
		Claims: auth.Claims{
			Expiry: time.Now().Add(time.Hour),
		},
	}
	if err := manager.StoreToken("test", token); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	got, err := getOrRefreshCommandToken(cmd, manager, "test")
	if err != nil {
		t.Fatalf("getOrRefreshCommandToken failed: %v", err)
	}
	if got == nil || got.Token != "compat-token" {
		t.Fatalf("expected compat-token, got %#v", got)
	}
}
