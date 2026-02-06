// Package compare provides functionality for comparing two Port organizations.
package compare

import (
	_ "embed"
	"html/template"
	"io"
)

//go:embed templates/report.html
var reportTemplate string

//go:embed templates/simple.html
var simpleTemplate string

// HTMLSection represents a section in the HTML report.
type HTMLSection struct {
	Name          string
	Added         int
	Modified      int
	Removed       int
	HasChanges    bool
	AddedItems    []ResourceChange
	ModifiedItems []ResourceChange
	RemovedItems  []ResourceChange
}

// HTMLData represents the data passed to HTML templates.
type HTMLData struct {
	Source        string
	Target        string
	Timestamp     string
	Identical     bool
	TotalAdded    int
	TotalModified int
	TotalRemoved  int
	Sections      []HTMLSection
}

// HTMLFormatter formats comparison results as HTML.
type HTMLFormatter struct {
	w      io.Writer
	simple bool
}

// NewHTMLFormatter creates a new HTML formatter.
func NewHTMLFormatter(w io.Writer, simple bool) *HTMLFormatter {
	return &HTMLFormatter{w: w, simple: simple}
}

// Format outputs the comparison result as HTML.
func (f *HTMLFormatter) Format(result *CompareResult) error {
	data := HTMLData{
		Source:    result.Source,
		Target:    result.Target,
		Timestamp: result.Timestamp,
		Identical: result.Identical,
	}

	// Build sections
	data.Sections = []HTMLSection{
		f.buildSection("Blueprints", result.Blueprints),
		f.buildSection("Actions", result.Actions),
		f.buildSection("Scorecards", result.Scorecards),
		f.buildSection("Pages", result.Pages),
		f.buildSection("Integrations", result.Integrations),
		f.buildSection("Teams", result.Teams),
		f.buildSection("Users", result.Users),
		f.buildSection("Automations", result.Automations),
	}

	// Calculate totals
	for _, s := range data.Sections {
		data.TotalAdded += s.Added
		data.TotalModified += s.Modified
		data.TotalRemoved += s.Removed
	}

	// Select template
	tmplStr := reportTemplate
	if f.simple {
		tmplStr = simpleTemplate
	}

	tmpl, err := template.New("report").Parse(tmplStr)
	if err != nil {
		return err
	}

	return tmpl.Execute(f.w, data)
}

func (f *HTMLFormatter) buildSection(name string, diff ResourceDiff) HTMLSection {
	return HTMLSection{
		Name:          name,
		Added:         diff.Summary.Added,
		Modified:      diff.Summary.Modified,
		Removed:       diff.Summary.Removed,
		HasChanges:    diff.Summary.Added > 0 || diff.Summary.Modified > 0 || diff.Summary.Removed > 0,
		AddedItems:    diff.Added,
		ModifiedItems: diff.Modified,
		RemovedItems:  diff.Removed,
	}
}
