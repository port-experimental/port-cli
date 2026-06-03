package skills

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

// ListSkills returns skill catalog entries from ai-service (GET /v1/skills/summary).
func (m *Module) ListSkills(ctx context.Context, query aiservice.GetSkillsSummaryQuery) ([]aiservice.SkillCatalogEntry, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := m.aiClient.GetSkillsSummary(ctx, m.token, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list skills: %w", err)
	}
	entries := append([]aiservice.SkillCatalogEntry(nil), resp.Skills...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Skill.Identifier < entries[j].Skill.Identifier
	})
	return entries, nil
}

// SearchSkills finds skills by identifier or title (GET /v1/skills/search).
func (m *Module) SearchSkills(ctx context.Context, query aiservice.SearchSkillsQuery) ([]aiservice.SkillCatalogEntry, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	if strings.TrimSpace(query.Query) == "" {
		return nil, fmt.Errorf("search query is required")
	}
	resp, err := m.aiClient.SearchSkills(ctx, m.token, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search skills: %w", err)
	}
	entries := append([]aiservice.SkillCatalogEntry(nil), resp.Skills...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Skill.Identifier < entries[j].Skill.Identifier
	})
	return entries, nil
}

// CreateSkillFromFolder uploads a new skill via POST /v1/skills.
func (m *Module) CreateSkillFromFolder(ctx context.Context, folder string, opts PackSkillFolderOptions, publish bool) (*aiservice.SkillVersionWriteResponse, error) {
	pack, err := PackSkillFolder(folder, opts)
	if err != nil {
		return nil, err
	}
	return m.CreateSkillFromPack(ctx, pack, publish)
}

// CreateSkillFromPack uploads a packed skill folder via POST /v1/skills.
func (m *Module) CreateSkillFromPack(ctx context.Context, pack *SkillFolderPack, publish bool) (*aiservice.SkillVersionWriteResponse, error) {
	if pack == nil {
		return nil, fmt.Errorf("skill pack is required")
	}
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	return m.aiClient.CreateSkill(ctx, m.token, aiservice.CreateSkillRequest{
		Identifier:  pack.Identifier,
		Title:       pack.Title,
		Description: pack.Description,
		Location:    pack.Location,
		Publish:     publish,
		Files:       pack.Files,
	})
}

// CreateSkillsBatch uploads multiple new skills via POST /v1/skills/batch.
func (m *Module) CreateSkillsBatch(ctx context.Context, packs []*SkillFolderPack, publish bool) (*aiservice.BatchCreateSkillsResponse, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	if len(packs) == 0 {
		return nil, fmt.Errorf("at least one skill is required")
	}
	skills := make([]aiservice.CreateSkillRequest, 0, len(packs))
	for _, pack := range packs {
		if pack == nil {
			return nil, fmt.Errorf("skill pack is required")
		}
		skills = append(skills, aiservice.CreateSkillRequest{
			Identifier:  pack.Identifier,
			Title:       pack.Title,
			Description: pack.Description,
			Location:    pack.Location,
			Publish:     publish,
			Files:       pack.Files,
		})
	}
	resp, err := m.aiClient.CreateSkillsBatch(ctx, m.token, aiservice.BatchCreateSkillsRequest{Skills: skills})
	if err != nil {
		return nil, fmt.Errorf("failed to batch create skills: %w", err)
	}
	return resp, nil
}

// EditSkillFromFolder uploads a new skill version via PUT /v1/skills/:identifier.
func (m *Module) EditSkillFromFolder(ctx context.Context, skillIdentifier, folder string, opts PackSkillFolderOptions, publish bool) (*aiservice.SkillVersionWriteResponse, error) {
	opts.Identifier = skillIdentifier
	pack, err := PackSkillFolder(folder, opts)
	if err != nil {
		return nil, err
	}
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	req := aiservice.EditSkillRequest{
		Publish: publish,
		Files:   pack.Files,
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
