package aiservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/auth"
)

func TestGetSkillsSummary_andWriteEndpoints(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/summary":
			_ = json.NewEncoder(w).Encode(SkillsSummaryResponse{
				OK: true,
				Skills: []SkillCatalogEntry{{
					Skill: CatalogEntitySnapshot{Identifier: "skill-a", Title: "A"},
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/search":
			if r.URL.Query().Get("q") != "api" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(SkillsSummaryResponse{
				OK: true,
				Skills: []SkillCatalogEntry{{
					Skill: CatalogEntitySnapshot{Identifier: "demo-api-guide", Title: "Demo API Guide"},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/upload":
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				http.Error(w, "expected multipart/form-data", http.StatusUnsupportedMediaType)
				return
			}
			_ = json.NewEncoder(w).Encode(SkillVersionWriteResponse{
				OK: true, SkillIdentifier: "skill-a", Version: "1.0.0", ActiveVersionSet: false,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/upload/batch":
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				http.Error(w, "expected multipart/form-data", http.StatusUnsupportedMediaType)
				return
			}
			_ = json.NewEncoder(w).Encode(BatchUploadSkillsResponse{
				OK: true,
				Results: []BatchUploadSkillResultItem{
					{Identifier: "skill-a", OK: true, Result: &SkillVersionWriteResponse{SkillIdentifier: "skill-a", Version: "1.0.0"}},
					{Identifier: "skill-b", OK: false, Error: &BatchSkillError{Name: "validation_error", Message: "invalid", StatusCode: 400}},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/skill-a/unpublish":
			_ = json.NewEncoder(w).Encode(UnpublishSkillResponse{OK: true, SkillIdentifier: "skill-a"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(ClientOpts{AIServiceURL: srv.URL + "/v1"})
	token := &auth.Token{
		Token:  "tok",
		Claims: auth.Claims{OrgId: "org", UserID: "user"},
	}
	ctx := context.Background()

	summary, err := client.GetSkillsSummary(ctx, token, GetSkillsSummaryQuery{})
	if err != nil || len(summary.Skills) != 1 {
		t.Fatalf("GetSkillsSummary: %v %+v", err, summary)
	}

	search, err := client.SearchSkills(ctx, token, SearchSkillsQuery{Query: "api"})
	if err != nil || len(search.Skills) != 1 || search.Skills[0].Skill.Identifier != "demo-api-guide" {
		t.Fatalf("SearchSkills: %v %+v", err, search)
	}

	uploaded, err := client.UploadSkill(ctx, token, UploadSkillRequest{
		Identifier: "skill-a",
		Files:      []SkillFileInput{{Path: "SKILL.md", Content: "# x"}},
	})
	if err != nil || uploaded.Version != "1.0.0" {
		t.Fatalf("UploadSkill: %v %+v", err, uploaded)
	}

	batch, err := client.UploadSkillsBatch(ctx, token, BatchUploadSkillsRequest{
		Skills: []UploadSkillRequest{
			{Identifier: "skill-a", Files: []SkillFileInput{{Path: "SKILL.md", Content: "# a"}}},
			{Identifier: "skill-b", Files: []SkillFileInput{{Path: "SKILL.md", Content: "# b"}}},
		},
	})
	if err != nil || len(batch.Results) != 2 || !batch.Results[0].OK || batch.Results[1].OK {
		t.Fatalf("UploadSkillsBatch: %v %+v", err, batch)
	}

	unpublished, err := client.UnpublishSkill(ctx, token, "skill-a")
	if err != nil || unpublished.SkillIdentifier != "skill-a" {
		t.Fatalf("UnpublishSkill: %v %+v", err, unpublished)
	}

	if gotMethod == "" || gotPath == "" {
		t.Fatalf("expected requests, got %s %s", gotMethod, gotPath)
	}
}
