//go:build e2e

package skills

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
	"github.com/port-experimental/port-cli/internal/auth"
	"github.com/port-experimental/port-cli/internal/config"
	skillmod "github.com/port-experimental/port-cli/internal/modules/skills"
	"gopkg.in/yaml.v3"
)

var (
	e2ePkgDir = func() string {
		_, f, _, _ := runtime.Caller(0)
		return filepath.Dir(f)
	}()
	e2eRepoRoot = filepath.Join(e2ePkgDir, "..", "..")
)

type env struct {
	Org            string
	APIURL         string
	AIServiceURL   string
	AdminURL       string
	ConfigDir      string
	WorkDir        string
	CursorDir      string
	PortSkillsRoot string
	RunID          string
	FixturesDir    string
	PortBin        string
}

type harness struct {
	t      *testing.T
	env    env
	cm     *config.ConfigManager
	token  *auth.Token
	orgCfg *config.OrganizationConfig
	ai     *aiservice.Client
	mod    *skillmod.Module
	admin  *AdminClient
}

func loadEnv(t *testing.T) env {
	t.Helper()
	repoRoot, err := filepath.Abs(e2eRepoRoot)
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	runID := os.Getenv("E2E_RUN_ID")
	if runID == "" {
		runID = strconv.FormatInt(time.Now().Unix(), 10)
	}
	configDir := filepath.Join(os.TempDir(), "port-cli-e2e-"+runID)
	workDir := filepath.Join(configDir, "workdir")
	cursorDir := filepath.Join(workDir, ".cursor")
	adminURL := os.Getenv("E2E_ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:3002/v0.1"
	}
	return env{
		Org:            envOr("ORG", "demo"),
		APIURL:         envOr("PORT_API_URL", "http://localhost:3000/v1"),
		AIServiceURL:   envOr("PORT_AI_SERVICE_URL", "http://localhost:3016/v1"),
		AdminURL:       adminURL,
		ConfigDir:      configDir,
		WorkDir:        workDir,
		CursorDir:      cursorDir,
		PortSkillsRoot: portSkillsRoot(cursorDir),
		RunID:          runID,
		FixturesDir:    filepath.Join(e2ePkgDir, "testdata"),
		PortBin:        envOr("PORT_BIN", filepath.Join(repoRoot, "bin", "port")),
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	e := loadEnv(t)
	if err := os.MkdirAll(e.CursorDir, 0o755); err != nil {
		t.Fatalf("mkdir cursor: %v", err)
	}
	if err := os.MkdirAll(e.WorkDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	userConfig := config.DefaultConfigPath()
	if _, err := os.Stat(userConfig); err != nil {
		t.Fatalf("missing %s — run: port auth login --org %s", userConfig, e.Org)
	}
	apiURL, err := readOrgAPIURL(userConfig, e.Org)
	if err != nil {
		t.Fatalf("read org api_url: %v", err)
	}
	if apiURL != "" {
		e.APIURL = apiURL
	}

	h := &harness{
		t:     t,
		env:   e,
		admin: newAdminClient(e.AdminURL),
	}
	if err := h.writeFreshConfig(nil); err != nil {
		t.Fatalf("write config: %v", err)
	}
	h.cm = config.NewConfigManager(filepath.Join(e.ConfigDir, "config.yaml"))

	ctx := context.Background()
	token, err := h.cm.GetOrRefreshToken(ctx, e.Org)
	if err != nil {
		t.Fatalf("auth token for org %q: %v — run: port auth login --org %s", e.Org, err, e.Org)
	}
	cfg, err := h.cm.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	orgCfg, err := cfg.GetOrgConfig(e.Org)
	if err != nil {
		t.Fatalf("org config: %v", err)
	}
	h.token = token
	h.orgCfg = orgCfg
	h.ai = aiservice.NewClient(aiservice.ClientOpts{
		APIURL:       orgCfg.APIURL,
		AIServiceURL: e.AIServiceURL,
		Timeout:      2 * time.Minute,
	})
	h.mod = skillmod.NewModule(token, orgCfg, h.ai, h.cm)
	return h
}

type skillsSelection struct {
	SelectedGroups     []string
	IncludeGroups      []string
	ExcludeGroups      []string
	SelectAllUngrouped bool
	TeamGroupDefaults  bool
}

func (h *harness) beginScenario(sel *skillsSelection) {
	h.t.Helper()
	if err := resetPortSkillsDir(h.env.CursorDir); err != nil {
		h.t.Fatalf("reset port skills: %v", err)
	}
	if err := h.writeFreshConfig(sel); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

func (h *harness) writeFreshConfig(sel *skillsSelection) error {
	if err := os.MkdirAll(h.env.ConfigDir, 0o700); err != nil {
		return err
	}
	userCreds := filepath.Join(os.Getenv("HOME"), ".port", "creds.json")
	if _, err := os.Stat(userCreds); err != nil {
		return fmt.Errorf("missing %s", userCreds)
	}
	dstCreds := filepath.Join(h.env.ConfigDir, "creds.json")
	_ = os.Remove(dstCreds)
	if err := os.Symlink(userCreds, dstCreds); err != nil {
		return fmt.Errorf("link creds: %w", err)
	}

	skillsBlock := map[string]any{
		"targets":              []string{h.env.CursorDir},
		"project_dirs":         []string{h.env.WorkDir},
		"select_all":           false,
		"select_all_groups":    false,
		"select_all_ungrouped": false,
		"team_group_defaults":  false,
		"selected_groups":      []string{},
		"selected_skills":      []string{},
		"include_groups":       []string{},
		"exclude_groups":       []string{},
	}
	if sel != nil {
		skillsBlock["select_all_ungrouped"] = sel.SelectAllUngrouped
		skillsBlock["team_group_defaults"] = sel.TeamGroupDefaults
		if len(sel.SelectedGroups) > 0 {
			skillsBlock["selected_groups"] = sel.SelectedGroups
		}
		if len(sel.IncludeGroups) > 0 {
			skillsBlock["include_groups"] = sel.IncludeGroups
		}
		if len(sel.ExcludeGroups) > 0 {
			skillsBlock["exclude_groups"] = sel.ExcludeGroups
		}
	}

	root := map[string]any{
		"default_org": h.env.Org,
		"organizations": map[string]any{
			h.env.Org: map[string]any{
				"api_url":       h.env.APIURL,
				"client_id":     "",
				"client_secret": "",
			},
		},
		"skills": skillsBlock,
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(h.env.ConfigDir, "config.yaml"), data, 0o600)
}

func (h *harness) writeConfigOrgOnly() error {
	if err := os.MkdirAll(h.env.ConfigDir, 0o700); err != nil {
		return err
	}
	userCreds := filepath.Join(os.Getenv("HOME"), ".port", "creds.json")
	dstCreds := filepath.Join(h.env.ConfigDir, "creds.json")
	_ = os.Remove(dstCreds)
	if err := os.Symlink(userCreds, dstCreds); err != nil {
		return fmt.Errorf("link creds: %w", err)
	}
	root := map[string]any{
		"default_org": h.env.Org,
		"organizations": map[string]any{
			h.env.Org: map[string]any{
				"api_url":       h.env.APIURL,
				"client_id":     "",
				"client_secret": "",
			},
		},
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(h.env.ConfigDir, "config.yaml"), data, 0o600)
}

func portSkillsRootForBase(base string) string {
	return filepath.Join(base, "skills", "port")
}

func (h *harness) syncWithoutInit(ctx context.Context, homeDir string) error {
	h.t.Helper()
	if err := h.writeConfigOrgOnly(); err != nil {
		return err
	}
	h.cm = config.NewConfigManager(filepath.Join(h.env.ConfigDir, "config.yaml"))
	h.mod = skillmod.NewModule(h.token, h.orgCfg, h.ai, h.cm)
	if homeDir != "" {
		h.t.Setenv("HOME", homeDir)
	}
	_, err := h.mod.LoadSkills(ctx, skillmod.LoadSkillsOptions{})
	return err
}

func (h *harness) sync(ctx context.Context, sel skillsSelection) error {
	h.t.Helper()
	if err := h.writeFreshConfig(&sel); err != nil {
		return err
	}
	_, err := h.mod.LoadSkills(ctx, skillmod.LoadSkillsOptions{
		SelectedGroups:     append([]string(nil), sel.SelectedGroups...),
		IncludeGroups:      append([]string(nil), sel.IncludeGroups...),
		ExcludeGroups:      append([]string(nil), sel.ExcludeGroups...),
		SelectAllUngrouped: sel.SelectAllUngrouped,
		TeamGroupDefaults:  sel.TeamGroupDefaults,
		ReplaceSelection:   true,
	})
	return err
}

func (h *harness) activeCatalog(ctx context.Context) map[string]ActiveSkillExpect {
	h.t.Helper()
	catalog, err := buildActiveCatalog(ctx, h.ai, h.token)
	if err != nil {
		h.t.Fatalf("fetch active catalog: %v", err)
	}
	return catalog
}

func curlHealth(base string) error {
	base = strings.TrimSuffix(base, "/")
	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{"/v1/health", "/health"} {
		resp, err := client.Get(base + path)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 300 {
			return nil
		}
	}
	return fmt.Errorf("health check failed for %s", base)
}

func readOrgAPIURL(userConfigPath, org string) (string, error) {
	data, err := os.ReadFile(userConfigPath)
	if err != nil {
		return "", err
	}
	var file struct {
		Organizations map[string]struct {
			APIURL string `yaml:"api_url"`
		} `yaml:"organizations"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return "", err
	}
	if o, ok := file.Organizations[org]; ok && strings.TrimSpace(o.APIURL) != "" {
		return strings.TrimSpace(o.APIURL), nil
	}
	return "", nil
}
