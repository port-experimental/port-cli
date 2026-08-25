package commands

import (
	"os"
	"strings"
	"testing"
)

func TestHandWrittenAPICommandsUseRuntimeClientHelper(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	body := string(src)
	if strings.Contains(body, "config.NewConfigManager") {
		t.Fatal("api.go must not construct ConfigManager directly; use clientForAPICommand")
	}
	if !strings.Contains(body, "func clientForAPICommand") {
		t.Fatal("api.go must define clientForAPICommand helper")
	}
}
