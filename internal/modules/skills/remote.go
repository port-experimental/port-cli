package skills

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/port-experimental/port-cli/internal/api/aiservice"
)

// ListSkills returns a paginated skill catalog from ai-service (GET /v1/skills/summary).
func (m *Module) ListSkills(ctx context.Context, query aiservice.GetSkillsSummaryQuery) (*aiservice.SkillsSummaryResponse, error) {
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
	resp.Skills = entries
	return resp, nil
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

// UploadSkillFromFolder uploads a skill via POST /v1/skills/upload.
func (m *Module) UploadSkillFromFolder(ctx context.Context, folder string, opts PackSkillFolderOptions, writeOpts UploadSkillWriteOptions) (*aiservice.SkillVersionWriteResponse, error) {
	pack, err := PackSkillFolder(folder, opts)
	if err != nil {
		return nil, err
	}
	return m.UploadSkillFromPack(ctx, pack, filepath.Base(folder), writeOpts)
}

// UploadSkillWriteOptions controls version creation when uploading skills.
type UploadSkillWriteOptions struct {
	Publish     bool
	VersionBump aiservice.VersionBump
}

// UploadSkillFromPack uploads a packed skill folder via POST /v1/skills/upload.
func (m *Module) UploadSkillFromPack(ctx context.Context, pack *SkillFolderPack, folderBase string, writeOpts UploadSkillWriteOptions) (*aiservice.SkillVersionWriteResponse, error) {
	if pack == nil {
		return nil, fmt.Errorf("skill pack is required")
	}
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	return m.aiClient.UploadSkill(ctx, m.token, uploadRequestFromPack(pack, folderBase, writeOpts))
}

// SkillPackWithFolder pairs a packed skill with its source folder basename.
type SkillPackWithFolder struct {
	Pack       *SkillFolderPack
	FolderBase string
}

// UploadSkillsBatch uploads multiple skills via POST /v1/skills/upload/batch.
func (m *Module) UploadSkillsBatch(ctx context.Context, packs []SkillPackWithFolder, writeOpts UploadSkillWriteOptions) (*aiservice.BatchUploadSkillsResponse, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	if len(packs) == 0 {
		return nil, fmt.Errorf("at least one skill is required")
	}
	skills := make([]aiservice.UploadSkillRequest, 0, len(packs))
	for _, item := range packs {
		if item.Pack == nil {
			return nil, fmt.Errorf("skill pack is required")
		}
		skills = append(skills, uploadRequestFromPack(item.Pack, item.FolderBase, writeOpts))
	}
	resp, err := m.aiClient.UploadSkillsBatch(ctx, m.token, aiservice.BatchUploadSkillsRequest{Skills: skills})
	if err != nil {
		return nil, fmt.Errorf("failed to batch upload skills: %w", err)
	}
	return resp, nil
}

// FetchSkill loads one published skill from ai-service.
func (m *Module) FetchSkill(ctx context.Context, identifier string) (Skill, error) {
	if m.aiClient == nil {
		return Skill{}, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := m.aiClient.GetSkill(ctx, m.token, identifier)
	if err != nil {
		return Skill{}, fmt.Errorf("failed to fetch skill %q: %w", identifier, err)
	}
	return skillFromAIService(resp.Skill, nil), nil
}

// PublishSkill sets the active version to the latest semver in Port.
func (m *Module) PublishSkill(ctx context.Context, identifier string) (*aiservice.SkillVersionWriteResponse, error) {
	if m.aiClient == nil {
		return nil, fmt.Errorf("ai-service client is not configured")
	}
	resp, err := m.aiClient.PublishSkill(ctx, m.token, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to publish skill %q: %w", identifier, err)
	}
	return resp, nil
}

// UnpublishSkill clears the active version in Port.
func (m *Module) UnpublishSkill(ctx context.Context, identifier string) error {
	if m.aiClient == nil {
		return fmt.Errorf("ai-service client is not configured")
	}
	_, err := m.aiClient.UnpublishSkill(ctx, m.token, identifier)
	if err != nil {
		return fmt.Errorf("failed to unpublish skill %q: %w", identifier, err)
	}
	return nil
}

func uploadRequestFromPack(pack *SkillFolderPack, folderBase string, writeOpts UploadSkillWriteOptions) aiservice.UploadSkillRequest {
	return aiservice.UploadSkillRequest{
		Identifier:     pack.Identifier,
		Title:          pack.Title,
		Description:    pack.Description,
		Location:       pack.Location,
		Publish:        writeOpts.Publish,
		VersionBump:    writeOpts.VersionBump,
		FolderBaseName: folderBase,
		Files:          pack.Files,
	}
}
