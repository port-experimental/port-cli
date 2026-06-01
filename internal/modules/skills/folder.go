package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

var skillIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// SkillFolderPack is the parsed content of a local skill directory.
type SkillFolderPack struct {
	Identifier  string
	Title       string
	Description string
	Location    string
	Files       []aiservice.SkillFileInput
}

// PackSkillFolderOptions configures reading a skill directory from disk.
type PackSkillFolderOptions struct {
	Identifier  string
	Title       string
	Description string
	Location    string
}

// PackSkillFolder reads all files under dir into ai-service file inputs.
// The folder must contain SKILL.md at its root. Identifier defaults to the
// directory name when opts.Identifier is empty.
func PackSkillFolder(dir string, opts PackSkillFolderOptions) (*SkillFolderPack, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("skill folder %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	identifier := opts.Identifier
	if identifier == "" {
		identifier = filepath.Base(absDir)
	}
	identifier = SanitizeSkillIdentifier(identifier)
	if !skillIdentifierPattern.MatchString(identifier) {
		return nil, fmt.Errorf("invalid skill identifier %q (use letters, numbers, hyphens, underscores)", identifier)
	}

	var files []aiservice.SkillFileInput
	hasSkillMD := false
	walkErr := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != absDir && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		if rel == "SKILL.md" {
			hasSkillMD = true
		}
		files = append(files, aiservice.SkillFileInput{
			Path:    rel,
			Content: string(content),
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if !hasSkillMD {
		return nil, fmt.Errorf("skill folder must contain SKILL.md at its root")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("skill folder contains no files")
	}

	title := opts.Title
	description := opts.Description
	location := opts.Location
	if meta := parseSkillMDMetadata(findFileContent(files, "SKILL.md")); meta != nil {
		if title == "" && meta.Title != "" {
			title = meta.Title
		}
		if description == "" && meta.Description != "" {
			description = meta.Description
		}
		if location == "" && meta.Location != "" {
			location = meta.Location
		}
		if opts.Identifier == "" && meta.Name != "" {
			identifier = SanitizeSkillIdentifier(meta.Name)
		}
	}
	if title == "" {
		title = identifier
	}
	if location == "" {
		location = "global"
	}
	if location != "global" && location != "project" {
		return nil, fmt.Errorf("location must be global or project, got %q", location)
	}

	return &SkillFolderPack{
		Identifier:  identifier,
		Title:       title,
		Description: description,
		Location:    location,
		Files:       files,
	}, nil
}

// SanitizeSkillIdentifier normalizes a string for use as a Port skill identifier.
func SanitizeSkillIdentifier(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

type skillMDMetadata struct {
	Name        string
	Title       string
	Description string
	Location    string
}

func parseSkillMDMetadata(content string) *skillMDMetadata {
	frontmatterMatch := regexp.MustCompile(`(?s)^---\n(.*?)\n---`).FindStringSubmatch(content)
	if len(frontmatterMatch) < 2 {
		return nil
	}
	block := frontmatterMatch[1]
	meta := &skillMDMetadata{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		switch key {
		case "name":
			meta.Name = val
		case "title":
			meta.Title = val
		case "description":
			meta.Description = val
		case "location":
			meta.Location = val
		}
	}
	if meta.Name == "" && meta.Title == "" && meta.Description == "" && meta.Location == "" {
		return nil
	}
	return meta
}

func findFileContent(files []aiservice.SkillFileInput, path string) string {
	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}
	return ""
}
