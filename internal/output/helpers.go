package output

import (
	"fmt"
	"io"
	"os"
)

// Verbosity levels
const (
	QuietLevel = iota
	NormalLevel
	VerboseLevel
)

var (
	verbosity              = NormalLevel
	outputWriter io.Writer = os.Stdout
	errorWriter  io.Writer = os.Stderr
)

// SetVerbosity sets the verbosity level.
func SetVerbosity(level int) {
	verbosity = level
}

// SetWriters sets the output and error writers (useful for testing).
func SetWriters(out, err io.Writer) {
	outputWriter = out
	errorWriter = err
}

// Print prints text if verbosity allows.
func Print(format string, args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, format, args...)
	}
}

// Println prints a line if verbosity allows.
func Println(args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintln(outputWriter, args...)
	}
}

// Printf prints formatted text if verbosity allows.
func Printf(format string, args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, format, args...)
	}
}

// QuietPrint prints text only in quiet mode (errors).
func QuietPrint(format string, args ...interface{}) {
	if verbosity == QuietLevel {
		fmt.Fprintf(outputWriter, format, args...)
	}
}

// VerbosePrint prints text only in verbose mode.
func VerbosePrint(format string, args ...interface{}) {
	if verbosity >= VerboseLevel {
		fmt.Fprintf(outputWriter, format, args...)
	}
}

// VerbosePrintf prints formatted text only in verbose mode.
func VerbosePrintf(format string, args ...interface{}) {
	if verbosity >= VerboseLevel {
		fmt.Fprintf(outputWriter, format, args...)
	}
}

// ErrorPrint always prints (errors should always be shown).
func ErrorPrint(format string, args ...interface{}) {
	fmt.Fprintf(errorWriter, format, args...)
}

// ErrorPrintf always prints formatted text (errors should always be shown).
func ErrorPrintf(format string, args ...interface{}) {
	fmt.Fprintf(errorWriter, format, args...)
}

// SuccessPrint prints success message if verbosity allows.
func SuccessPrint(format string, args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, "%s", Success(fmt.Sprintf(format, args...)))
	}
}

// SuccessPrintln prints success message with newline if verbosity allows.
func SuccessPrintln(text string) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, "%s\n", Success(text))
	}
}

// WarningPrint prints warning message if verbosity allows.
func WarningPrint(format string, args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, "%s", Warning(fmt.Sprintf(format, args...)))
	}
}

// WarningPrintln prints warning message with newline if verbosity allows.
func WarningPrintln(text string) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, "%s\n", Warning(text))
	}
}

// InfoPrint prints info message if verbosity allows.
func InfoPrint(format string, args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, "%s", Info(fmt.Sprintf(format, args...)))
	}
}

// WarningPrintf prints formatted warning message if verbosity allows.
func WarningPrintf(format string, args ...interface{}) {
	if verbosity >= NormalLevel {
		fmt.Fprintf(outputWriter, "%s", Warning(fmt.Sprintf(format, args...)))
	}
}
