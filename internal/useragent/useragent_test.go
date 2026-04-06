package useragent

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	t.Run("default", func(t *testing.T) {
		SetVersion("dev")
		want := "port-cli/dev (" + platform + ")"
		if got := String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})
	t.Run("with version", func(t *testing.T) {
		SetVersion("1.2.3")
		want := "port-cli/1.2.3 (" + platform + ")"
		if got := String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
		SetVersion("dev")
	})
	t.Run("format", func(t *testing.T) {
		SetVersion("0.1.3")
		got := String()
		if !strings.HasPrefix(got, "port-cli/") {
			t.Errorf("String() = %q, expected prefix port-cli/", got)
		}
		if !strings.Contains(got, "(") || !strings.Contains(got, "/") {
			t.Errorf("String() = %q, expected os/arch in parentheses", got)
		}
		SetVersion("dev")
	})
}
