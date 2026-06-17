package agents

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseAgentFile reads a .md file with YAML frontmatter and returns an AgentFileSpec.
//
// Guards (checked in this order, fail-fast):
//  1. path must be non-empty
//  2. os.Lstat: file must exist; must not be a symlink
//  3. stat.Size() must not exceed 1 MB (1<<20 bytes)
//
// Parsing:
//   - Read full file content via os.ReadFile
//   - Locate frontmatter: first line must be "---"; scan for the next "---" on its own line
//   - Extract frontmatter bytes between the two delimiters
//   - Extract body: everything after the closing "---" line
//   - Unmarshal frontmatter using gopkg.in/yaml.v3 into AgentFileSpec
//   - Assign strings.TrimSpace(body) to spec.Prompt
//   - Validate: spec.Identifier must be non-empty
//
// Returns (*AgentFileSpec, nil) on success.
// Returns (nil, error) on any failure.
func ParseAgentFile(path string) (*AgentFileSpec, error) {
	if path == "" {
		return nil, errors.New("file path is required")
	}

	stat, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s: %w", path, err)
	}

	if stat.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("file must not be a symlink: %s", path)
	}

	if stat.Size() > 1<<20 {
		return nil, fmt.Errorf("file exceeds maximum size of 1 MB: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	lines := strings.Split(string(content), "\n")

	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, fmt.Errorf("no YAML frontmatter found in %s", path)
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return nil, fmt.Errorf("no YAML frontmatter found in %s", path)
	}

	frontmatterLines := lines[1:closeIdx]
	bodyLines := lines[closeIdx+1:]

	frontmatter := strings.Join(frontmatterLines, "\n")
	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))

	var spec AgentFileSpec
	if err := yaml.Unmarshal([]byte(frontmatter), &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter in %s: %w", path, err)
	}

	spec.Prompt = body

	if spec.Identifier == "" {
		return nil, errors.New("identifier is required in frontmatter")
	}

	return &spec, nil
}
