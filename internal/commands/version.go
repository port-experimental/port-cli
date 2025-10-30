package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// BuildInfo holds build-time information
type BuildInfo struct {
	Version   string
	BuildDate string
	Commit    string
	GoVersion string
	Platform  string
}

var buildInfo = BuildInfo{
	Version:   "dev",
	BuildDate: "unknown",
	Commit:    "unknown",
	GoVersion: runtime.Version(),
	Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
}

// SetBuildInfo sets build information (called from main.go)
func SetBuildInfo(info BuildInfo) {
	buildInfo = info
}

// RegisterVersion registers the version command.
func RegisterVersion(rootCmd *cobra.Command) {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Port CLI version %s\n", buildInfo.Version)
			fmt.Printf("Build date: %s\n", buildInfo.BuildDate)
			fmt.Printf("Git commit: %s\n", buildInfo.Commit)
			fmt.Printf("Go version: %s\n", buildInfo.GoVersion)
			fmt.Printf("Platform: %s\n", buildInfo.Platform)
		},
	}

	rootCmd.AddCommand(versionCmd)
}

