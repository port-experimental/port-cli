package plugin

import (
	"testing"
	"time"

	"github.com/port-experimental/port-cli/internal/config"
)

func TestSaveAndLoadPluginConfig(t *testing.T) {
	_, cm, _ := newTestModule(t)
	cfg := &config.PluginConfig{
		Targets:            []string{"/home/user/.cursor", "/home/user/.claude"},
		SelectAllGroups:    true,
		SelectAllUngrouped: false,
		SelectedSkills:     []string{"skill-x"},
		LastSyncedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	writeCfg(t, cm, cfg)

	loaded, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig: %v", err)
	}
	if len(loaded.Targets) != 2 {
		t.Errorf("Targets: got %d", len(loaded.Targets))
	}
	if !loaded.SelectAllGroups {
		t.Error("SelectAllGroups should be true")
	}
	if loaded.SelectAllUngrouped {
		t.Error("SelectAllUngrouped should be false")
	}
	if len(loaded.SelectedSkills) != 1 || loaded.SelectedSkills[0] != "skill-x" {
		t.Errorf("SelectedSkills: got %v", loaded.SelectedSkills)
	}
}

func TestSavePluginConfig_PreservesOtherFields(t *testing.T) {
	_, cm, _ := newTestModule(t)
	if err := cm.SavePluginConfig(&config.PluginConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := cm.SavePluginConfig(&config.PluginConfig{SelectAll: true}); err != nil {
		t.Fatalf("second SavePluginConfig: %v", err)
	}
	loaded, err := cm.LoadPluginConfig()
	if err != nil {
		t.Fatalf("LoadPluginConfig: %v", err)
	}
	if !loaded.SelectAll {
		t.Error("expected SelectAll=true")
	}
}

func TestPluginConfig_HasSelection(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.PluginConfig
		want bool
	}{
		{"empty", config.PluginConfig{}, false},
		{"targets set", config.PluginConfig{Targets: []string{"/foo"}}, true},
		{"select all", config.PluginConfig{SelectAll: true}, true},
		{"select all groups", config.PluginConfig{SelectAllGroups: true}, true},
		{"selected skills", config.PluginConfig{SelectedSkills: []string{"s"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasSelection(); got != tt.want {
				t.Errorf("HasSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
