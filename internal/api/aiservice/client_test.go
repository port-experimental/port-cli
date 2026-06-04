package aiservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/auth"
)

func TestGetSkillsGrouped_ExcludeFilesQuery(t *testing.T) {
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"groups":[],"ungroupedSkills":[]}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient(ClientOpts{AIServiceURL: srv.URL})
	token := &auth.Token{Token: "test-token"}
	_, err := client.GetSkillsGrouped(context.Background(), token, GetSkillsQuery{
		Exclude: []string{"files"},
	})
	if err != nil {
		t.Fatalf("GetSkillsGrouped: %v", err)
	}
	if !strings.Contains(rawQuery, "exclude=files") {
		t.Fatalf("query %q missing exclude=files", rawQuery)
	}
}

func TestDeriveAIServiceURL(t *testing.T) {
	tests := []struct {
		api  string
		want string
	}{
		{"https://api.getport.io/v1", "https://ai-service.getport.io/v1"},
		{"https://api.us.getport.io/v1", "https://ai-service.us.getport.io/v1"},
		{"http://localhost:3000/v1", "http://localhost:3016/v1"},
		{"http://127.0.0.1:3000/v1", "http://127.0.0.1:3016/v1"},
	}
	for _, tt := range tests {
		if got := DeriveAIServiceURL(tt.api); got != tt.want {
			t.Errorf("DeriveAIServiceURL(%q) = %q, want %q", tt.api, got, tt.want)
		}
	}
}
