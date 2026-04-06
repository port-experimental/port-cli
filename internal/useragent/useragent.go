package useragent

import (
	"fmt"
	"runtime"
)

// Name is the product identifier in User-Agent strings.
const Name = "port-cli"

var version = "dev"

// SetVersion sets the CLI version used in User-Agent (e.g. from build / main).
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

// String returns the User-Agent value, e.g. "port-cli/0.1.3 (darwin/arm64)".
func String() string {
	return fmt.Sprintf("%s/%s (%s/%s)", Name, version, runtime.GOOS, runtime.GOARCH)
}
