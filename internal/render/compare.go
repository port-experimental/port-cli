package render

import (
	"fmt"
	"io"
	"os"

	"github.com/port-experimental/port-cli/internal/modules/compare"
)

// CompareRenderer formats compare command output.
type CompareRenderer struct{}

// Render dispatches compare results to text, JSON, or HTML formatters.
func (CompareRenderer) Render(result *compare.CompareResult, opts compare.Options) error {
	var w io.Writer = os.Stdout

	switch opts.OutputFormat {
	case "json":
		return compare.NewJSONFormatter(w).Format(result)

	case "html":
		filePath := opts.HTMLFile
		if filePath == "" {
			filePath = "comparison-report.html"
		}
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create HTML file: %w", err)
		}
		defer file.Close()

		formatter := compare.NewHTMLFormatter(file, opts.HTMLSimple)
		if err := formatter.Format(result); err != nil {
			return err
		}
		fmt.Printf("HTML report written to %s\n", filePath)
		return nil

	default:
		return compare.NewTextFormatter(w, opts.Verbose, opts.Full, opts.IncludeResources).Format(result)
	}
}
