package skills

import (
	"context"
	"fmt"
	"sort"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

// ListSkillIdentifiers returns _skill entity identifiers from ai-service.
func (m *Module) ListSkillIdentifiers(ctx context.Context) ([]string, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := m.aiClient.GetSkillsSummary(ctx, m.token, aiservice.GetSkillsSummaryQuery{})
	if err != nil {
		return nil, fmt.Errorf("failed to list skills: %w", err)
	}
	ids := make([]string, 0, len(resp.Skills))
	for _, entry := range resp.Skills {
		if entry.Skill.Identifier != "" {
			ids = append(ids, entry.Skill.Identifier)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// CreateSkillFromFolder uploads a new skill via POST /v1/skills.
func (m *Module) CreateSkillFromFolder(ctx context.Context, folder string, opts PackSkillFolderOptions, published bool) (*aiservice.SkillVersionWriteResponse, error) {
	pack, err := PackSkillFolder(folder, opts)
	if err != nil {
		return nil, err
	}
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	return m.aiClient.CreateSkill(ctx, m.token, aiservice.CreateSkillRequest{
		Identifier:  pack.Identifier,
		Title:       pack.Title,
		Description: pack.Description,
		Location:    pack.Location,
		Published:   published,
		Files:       pack.Files,
	})
}

// EditSkillFromFolder uploads a new skill version via PUT /v1/skills/:identifier.
func (m *Module) EditSkillFromFolder(ctx context.Context, skillIdentifier, folder string, opts PackSkillFolderOptions, published bool) (*aiservice.SkillVersionWriteResponse, error) {
	opts.Identifier = skillIdentifier
	pack, err := PackSkillFolder(folder, opts)
	if err != nil {
		return nil, err
	}
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	req := aiservice.EditSkillRequest{
		Published: published,
		Files:     pack.Files,
	}
	if opts.Title != "" {
		req.Title = opts.Title
	} else if pack.Title != "" && pack.Title != skillIdentifier {
		req.Title = pack.Title
	}
	if opts.Description != "" {
		req.Description = opts.Description
	} else if pack.Description != "" {
		req.Description = pack.Description
	}
	if opts.Location != "" {
		req.Location = opts.Location
	} else if pack.Location != "" {
		req.Location = pack.Location
	}
	return m.aiClient.EditSkill(ctx, m.token, skillIdentifier, req)
}

// ArchiveSkill archives all versions of a skill.
func (m *Module) ArchiveSkill(ctx context.Context, skillIdentifier string) (*aiservice.ArchiveSkillResponse, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := m.aiClient.ArchiveSkill(ctx, m.token, skillIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to archive skill: %w", err)
	}
	return resp, nil
}
