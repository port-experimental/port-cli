package agents

// @spec-handoff
// @interface ParseAgentFile(path string) (*AgentFileSpec, error)
// @behavior
//   - Rejects empty path before any filesystem access
//   - Calls os.Lstat to reject symlinks and non-existent files
//   - Rejects files larger than 1 MB (1<<20 bytes) based on Lstat size
//   - Splits file content on the first pair of bare "---" lines to extract frontmatter and body
//   - Unmarshals YAML frontmatter into AgentFileSpec using gopkg.in/yaml.v3
//   - Assigns strings.TrimSpace(body) to AgentFileSpec.Prompt
//   - Returns error if Identifier is empty after parse
//   - Returns non-nil *AgentFileSpec on success; nil on any error
// @edge-cases
//   - path == "" → error "file path is required"
//   - File does not exist → wrapped os.ErrNotExist (os.IsNotExist wrappable)
//   - File is a symlink → error containing "symlink"
//   - File > 1 MB → error containing "file too large" or "exceeds maximum size"
//   - No frontmatter delimiters at all → error containing "no YAML frontmatter found"
//   - Frontmatter present but identifier missing/empty → error "identifier is required in frontmatter"
//   - tools key absent → spec.Tools == nil
//   - Body with leading/trailing whitespace → whitespace stripped in Prompt
//   - Empty body after closing --- → spec.Prompt == ""
//   - Multiple tools entries → correctly parsed as []string
//   - Frontmatter-only (no closing ---) → error containing "no YAML frontmatter found"
// @see ./agents.go (AgentFileSpec)
// @see ./parse.go (ParseAgentFile — to be created in E3)

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// writeAgentFile writes content to a temp file and returns its path.
func writeAgentFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp agent file: %v", err)
	}
	return path
}

// writeOversizedFile creates a temp file with size just over 1 MB.
func writeOversizedFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "big_agent.md")
	// 1 MB + 1 byte
	data := make([]byte, (1<<20)+1)
	// Fill with valid-looking content so it isn't rejected for any other reason first.
	copy(data, []byte("---\nidentifier: big\n---\n"))
	for i := 23; i < len(data); i++ {
		data[i] = 'x'
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write oversized file: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// P1: empty path
// ---------------------------------------------------------------------------

func TestParseAgentFile_EmptyPath(t *testing.T) {
	spec, err := ParseAgentFile("")
	if err == nil {
		t.Fatal("want error for empty path, got nil")
	}
	const wantMsg = "file path is required"
	if !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("want error containing %q, got %q", wantMsg, err.Error())
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P2: file does not exist
// ---------------------------------------------------------------------------

func TestParseAgentFile_FileNotExist(t *testing.T) {
	spec, err := ParseAgentFile("/tmp/this-file-definitely-does-not-exist-port-cli-test.md")
	if err == nil {
		t.Fatal("want error for non-existent file, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("want error wrapping os.ErrNotExist, got %v (type %T)", err, err)
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P3: file is a symlink
// ---------------------------------------------------------------------------

func TestParseAgentFile_Symlink(t *testing.T) {
	dir := t.TempDir()
	// Create a real target file.
	target := filepath.Join(dir, "real.md")
	if err := os.WriteFile(target, []byte("---\nidentifier: a\n---\n"), 0o644); err != nil {
		t.Fatalf("failed to write target: %v", err)
	}
	// Create a symlink pointing to it.
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	spec, err := ParseAgentFile(link)
	if err == nil {
		t.Fatal("want error for symlink, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("want error containing %q, got %q", "symlink", err.Error())
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P4: file over 1 MB
// ---------------------------------------------------------------------------

func TestParseAgentFile_FileTooLarge(t *testing.T) {
	path := writeOversizedFile(t)

	spec, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("want error for oversized file, got nil")
	}
	// Accept either phrasing: "file too large" OR "exceeds maximum size"
	if !strings.Contains(err.Error(), "too large") && !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("want error containing size-limit message, got %q", err.Error())
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P5: no frontmatter delimiters
// ---------------------------------------------------------------------------

func TestParseAgentFile_NoFrontmatter(t *testing.T) {
	path := writeAgentFile(t, "identifier: triage_agent\nThis is plain text with no --- delimiters.\n")

	spec, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("want error for file with no frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "no YAML frontmatter found") {
		t.Errorf("want error containing %q, got %q", "no YAML frontmatter found", err.Error())
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P5b: frontmatter opened but never closed (no second ---)
// ---------------------------------------------------------------------------

func TestParseAgentFile_UnclosedFrontmatter(t *testing.T) {
	content := "---\nidentifier: triage_agent\ntitle: Triage Agent\n"
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("want error for unclosed frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "no YAML frontmatter found") {
		t.Errorf("want error containing %q, got %q", "no YAML frontmatter found", err.Error())
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P6: identifier missing
// ---------------------------------------------------------------------------

func TestParseAgentFile_MissingIdentifier(t *testing.T) {
	content := "---\ntitle: My Agent\nmodel: gpt-4o\n---\nThis is the prompt body.\n"
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err == nil {
		t.Fatal("want error for missing identifier, got nil")
	}
	const wantMsg = "identifier is required in frontmatter"
	if !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("want error containing %q, got %q", wantMsg, err.Error())
	}
	if spec != nil {
		t.Errorf("want nil *AgentFileSpec on error, got %+v", spec)
	}
}

// ---------------------------------------------------------------------------
// P7: valid full frontmatter + body → all fields populated
// ---------------------------------------------------------------------------

func TestParseAgentFile_ValidFullFrontmatter(t *testing.T) {
	content := strings.Join([]string{
		"---",
		"identifier: triage_agent",
		"title: Triage Agent",
		"description: Triages incoming requests",
		"model: gpt-4o",
		"provider: openai",
		"execution_mode: async",
		"status: active",
		"tools:",
		"  - search",
		"  - summarize",
		"---",
		"You are a triage agent. Prioritize incoming tasks.",
		"",
	}, "\n")
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("want non-nil *AgentFileSpec, got nil")
	}

	checks := []struct {
		field string
		want  string
		got   string
	}{
		{"Identifier", "triage_agent", spec.Identifier},
		{"Title", "Triage Agent", spec.Title},
		{"Description", "Triages incoming requests", spec.Description},
		{"Model", "gpt-4o", spec.Model},
		{"Provider", "openai", spec.Provider},
		{"ExecutionMode", "async", spec.ExecutionMode},
		{"Status", "active", spec.Status},
		{"Prompt", "You are a triage agent. Prioritize incoming tasks.", spec.Prompt},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("spec.%s: want %q, got %q", c.field, c.want, c.got)
		}
	}

	if len(spec.Tools) != 2 {
		t.Fatalf("want 2 tools, got %d: %v", len(spec.Tools), spec.Tools)
	}
	if spec.Tools[0] != "search" {
		t.Errorf("want Tools[0] == %q, got %q", "search", spec.Tools[0])
	}
	if spec.Tools[1] != "summarize" {
		t.Errorf("want Tools[1] == %q, got %q", "summarize", spec.Tools[1])
	}
}

// ---------------------------------------------------------------------------
// P7b: minimal valid frontmatter (identifier only) → other fields are zero values
// ---------------------------------------------------------------------------

func TestParseAgentFile_MinimalFrontmatter(t *testing.T) {
	content := "---\nidentifier: minimal_agent\n---\n"
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("want non-nil *AgentFileSpec, got nil")
	}
	if spec.Identifier != "minimal_agent" {
		t.Errorf("want Identifier %q, got %q", "minimal_agent", spec.Identifier)
	}
	if spec.Title != "" {
		t.Errorf("want empty Title, got %q", spec.Title)
	}
	if spec.Model != "" {
		t.Errorf("want empty Model, got %q", spec.Model)
	}
	if spec.Prompt != "" {
		t.Errorf("want empty Prompt for empty body, got %q", spec.Prompt)
	}
}

// ---------------------------------------------------------------------------
// P8: tools absent → spec.Tools is nil
// ---------------------------------------------------------------------------

func TestParseAgentFile_ToolsAbsent(t *testing.T) {
	content := "---\nidentifier: no_tools_agent\ntitle: No Tools\n---\nPrompt here.\n"
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("want non-nil *AgentFileSpec, got nil")
	}
	if spec.Tools != nil {
		t.Errorf("want spec.Tools == nil when tools key absent, got %v", spec.Tools)
	}
}

// ---------------------------------------------------------------------------
// P9: body with leading/trailing whitespace → stripped
// ---------------------------------------------------------------------------

func TestParseAgentFile_BodyWhitespaceStripped(t *testing.T) {
	content := "---\nidentifier: ws_agent\n---\n\n  \n  You are a whitespace agent.  \n\n"
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("want non-nil *AgentFileSpec, got nil")
	}
	const wantPrompt = "You are a whitespace agent."
	if spec.Prompt != wantPrompt {
		t.Errorf("want Prompt %q (whitespace stripped), got %q", wantPrompt, spec.Prompt)
	}
}

// ---------------------------------------------------------------------------
// P10: empty body → Prompt is ""
// ---------------------------------------------------------------------------

func TestParseAgentFile_EmptyBody(t *testing.T) {
	content := "---\nidentifier: empty_body_agent\n---\n"
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("want non-nil *AgentFileSpec, got nil")
	}
	if spec.Prompt != "" {
		t.Errorf("want empty Prompt for empty body, got %q", spec.Prompt)
	}
}

// ---------------------------------------------------------------------------
// Extra: multiple tools entries parsed correctly
// ---------------------------------------------------------------------------

func TestParseAgentFile_MultipleTools(t *testing.T) {
	content := strings.Join([]string{
		"---",
		"identifier: multi_tool_agent",
		"tools:",
		"  - search",
		"  - code_interpreter",
		"  - browser",
		"---",
		"Do everything.",
	}, "\n")
	path := writeAgentFile(t, content)

	spec, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Tools) != 3 {
		t.Fatalf("want 3 tools, got %d: %v", len(spec.Tools), spec.Tools)
	}
	wantTools := []string{"search", "code_interpreter", "browser"}
	for i, want := range wantTools {
		if spec.Tools[i] != want {
			t.Errorf("Tools[%d]: want %q, got %q", i, want, spec.Tools[i])
		}
	}
}
