package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/config"
)

// ---------------------------------------------------------------------------
// Module / config helpers
// ---------------------------------------------------------------------------

func newTestModule(t *testing.T) (*Module, *config.ConfigManager, string) {
	t.Helper()
	dir := t.TempDir()
	cm := config.NewConfigManager(filepath.Join(dir, "config.yaml"))
	orgCfg := &config.OrganizationConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIURL:       "https://api.getport.io/v1",
	}
	return NewModule(nil, orgCfg, cm), cm, dir
}

func writeCfg(t *testing.T, cm *config.ConfigManager, cfg *config.PluginConfig) {
	t.Helper()
	if err := cm.SavePluginConfig(cfg); err != nil {
		t.Fatalf("SavePluginConfig: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

// assertFileExists fails if path does not exist.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// assertFileAbsent fails if path exists.
func assertFileAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be absent: %s", path)
	}
}

// ---------------------------------------------------------------------------
// Skill / path helpers
// ---------------------------------------------------------------------------

// skillMDPath returns the expected SKILL.md path inside a target directory.
func skillMDPath(targetDir, groupID, skillID string) string {
	if groupID == "" {
		groupID = NoGroupDir
	}
	return filepath.Join(targetDir, "skills", PortSkillsDir, groupID, skillID, "SKILL.md")
}

func identifiers(skills []Skill) []string {
	ids := make([]string, len(skills))
	for i, s := range skills {
		ids[i] = s.Identifier
	}
	return ids
}

// ---------------------------------------------------------------------------
// Generic string slice helpers
// ---------------------------------------------------------------------------

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsStr(body, substr string) bool {
	return strings.Contains(body, substr)
}
