package aiservice

import "testing"

func TestDeriveAIServiceURL(t *testing.T) {
	tests := []struct {
		api  string
		want string
	}{
		{"https://api.getport.io/v1", "https://ai-service.getport.io/v1"},
		{"https://api.us.getport.io/v1", "https://ai-service.us.getport.io/v1"},
	}
	for _, tt := range tests {
		if got := DeriveAIServiceURL(tt.api); got != tt.want {
			t.Errorf("DeriveAIServiceURL(%q) = %q, want %q", tt.api, got, tt.want)
		}
	}
}
