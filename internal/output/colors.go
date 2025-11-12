package output

import (
	"fmt"
	"os"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	cyan   = "\033[36m"
	Gray   = "\033[90m"
)

var (
	enabled     = true
	forceDisable = false
)

// Init initializes color output based on environment and flags.
// Call this early in the application lifecycle.
func Init(noColor bool) {
	forceDisable = noColor
	if forceDisable {
		enabled = false
		return
	}

	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		enabled = false
		return
	}

	// Check if output is a TTY
	if !isTerminal(os.Stdout) {
		enabled = false
		return
	}

	enabled = true
}

// isTerminal checks if the file descriptor is a terminal.
func isTerminal(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// Color wraps text with color codes if colors are enabled.
func Color(text, colorCode string) string {
	if !enabled {
		return text
	}
	return colorCode + text + Reset
}

// Success returns green-colored text.
func Success(text string) string {
	return Color(text, Green)
}

// Error returns red-colored text.
func Error(text string) string {
	return Color(text, Red)
}

// Warning returns yellow-colored text.
func Warning(text string) string {
	return Color(text, Yellow)
}

// Info returns blue-colored text.
func Info(text string) string {
	return Color(text, Blue)
}

// Dim returns gray-colored text.
func Dim(text string) string {
	return Color(text, Gray)
}

// Cyan returns cyan-colored text.
func Cyan(text string) string {
	return Color(text, cyan)
}

// Successf formats and colors text as success.
func Successf(format string, args ...interface{}) string {
	return Success(fmt.Sprintf(format, args...))
}

// Errorf formats and colors text as error.
func Errorf(format string, args ...interface{}) string {
	return Error(fmt.Sprintf(format, args...))
}

// Warningf formats and colors text as warning.
func Warningf(format string, args ...interface{}) string {
	return Warning(fmt.Sprintf(format, args...))
}

// Infof formats and colors text as info.
func Infof(format string, args ...interface{}) string {
	return Info(fmt.Sprintf(format, args...))
}

// Enabled returns whether colors are currently enabled.
func Enabled() bool {
	return enabled
}

