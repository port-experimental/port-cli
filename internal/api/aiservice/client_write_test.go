package aiservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(SkillVersionWriteResponse{
				OK: true, SkillIdentifier: "skill-a", Version: "1.0.0",
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/skills/skill-a":
			_ = json.NewEncoder(w).Encode(SkillVersionWriteResponse{
				OK: true, SkillIdentifier: "skill-a", Version: "1.0.1",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/skill-a/archive":
			_ = json.NewEncoder(w).Encode(ArchiveSkillResponse{
				OK: true, SkillIdentifier: "skill-a", VersionsArchived: 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(ClientOpts{AIServiceURL: srv.URL + "/v1"})
	token := &auth.Token{
		Token: "tok",
		Claims: auth.Claims{OrgId: "org", UserID: "user"},
	}
	ctx := context.Background()

	summary, err := client.GetSkillsSummary(ctx, token, GetSkillsSummaryQuery{})
	if err != nil || len(summary.Skills) != 1 {
		t.Fatalf("GetSkillsSummary: %v %+v", err, summary)
	}

	created, err := client.CreateSkill(ctx, token, CreateSkillRequest{
		Identifier: "skill-a",
		Files:      []SkillFileInput{{Path: "SKILL.md", Content: "# x"}},
	})
	if err != nil || created.Version != "1.0.0" {
		t.Fatalf("CreateSkill: %v %+v", err, created)
	}

	edited, err := client.EditSkill(ctx, token, "skill-a", EditSkillRequest{
		Files: []SkillFileInput{{Path: "SKILL.md", Content: "# y"}},
	})
	if err != nil || edited.Version != "1.0.1" {
		t.Fatalf("EditSkill: %v %+v", err, edited)
	}

	archived, err := client.ArchiveSkill(ctx, token, "skill-a")
	if err != nil || archived.VersionsArchived != 2 {
		t.Fatalf("ArchiveSkill: %v %+v", err, archived)
	}

	if gotMethod == "" || gotPath == "" {
		t.Fatalf("expected requests, got %s %s", gotMethod, gotPath)
	}
}
