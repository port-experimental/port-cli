package export

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/port-labs/port-cli/internal/api"
	"github.com/port-labs/port-cli/internal/config"
)

// Module handles exporting data from Port.
type Module struct {
	client *api.Client
}

// NewModule creates a new export module.
func NewModule(orgConfig *config.OrganizationConfig) *Module {
	client := api.NewClient(orgConfig.ClientID, orgConfig.ClientSecret, orgConfig.APIURL, 0)
	return &Module{
		client: client,
	}
}

// Result represents the result of an export operation.
type Result struct {
	Success        bool
	Message        string
	OutputPath     string
	BlueprintsCount int
	EntitiesCount   int
	ActionsCount    int
	PagesCount      int
	IntegrationsCount int
	UsersCount      int
	Error          error
}

// Execute performs the export operation.
func (m *Module) Execute(ctx context.Context, opts Options) (*Result, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Collect data concurrently
	collector := NewCollector(m.client)
	data, err := collector.Collect(ctx, opts)
	if err != nil {
		return &Result{
			Success: false,
			Message: "Export failed",
			Error:   err,
		}, nil
	}

	// Write output
	formatType := opts.Format
	if formatType == "" {
		// Determine format from file extension
		ext := strings.ToLower(filepath.Ext(opts.OutputPath))
		if ext == ".json" {
			formatType = "json"
		} else {
			formatType = "tar"
		}
	}

	var writeErr error
	if formatType == "tar" {
		writeErr = writeTar(data, opts.OutputPath)
	} else {
		writeErr = writeJSON(data, opts.OutputPath)
	}

	if writeErr != nil {
		return &Result{
			Success: false,
			Message: "Export failed",
			Error:   fmt.Errorf("failed to write output: %w", writeErr),
		}, nil
	}

	return &Result{
		Success:         true,
		Message:         fmt.Sprintf("Successfully exported data to %s", opts.OutputPath),
		OutputPath:       opts.OutputPath,
		BlueprintsCount:  len(data.Blueprints),
		EntitiesCount:    len(data.Entities),
		ActionsCount:     len(data.Actions),
		PagesCount:       len(data.Pages),
		IntegrationsCount: len(data.Integrations),
		UsersCount:       len(data.Users),
	}, nil
}

// Close closes the API client.
func (m *Module) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// writeTar writes data to a tar.gz file.
func writeTar(data *Data, outputPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Write each data type to separate JSON files in the tar
	dataTypes := map[string]interface{}{
		"blueprints":   data.Blueprints,
		"entities":     data.Entities,
		"scorecards":   data.Scorecards,
		"actions":      data.Actions,
		"teams":        data.Teams,
		"users":        data.Users,
		"pages":        data.Pages,
		"integrations":  data.Integrations,
	}

	for dataType, items := range dataTypes {
		jsonData, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", dataType, err)
		}

		// Create tar header
		header := &tar.Header{
			Name: fmt.Sprintf("%s.json", dataType),
			Size: int64(len(jsonData)),
			Mode: 0644,
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", dataType, err)
		}

		if _, err := tw.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write %s to tar: %w", dataType, err)
		}
	}

	return nil
}

// writeJSON writes data to a JSON file.
func writeJSON(data *Data, outputPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	output := map[string]interface{}{
		"blueprints":   data.Blueprints,
		"entities":     data.Entities,
		"scorecards":   data.Scorecards,
		"actions":      data.Actions,
		"teams":        data.Teams,
		"users":        data.Users,
		"pages":        data.Pages,
		"integrations":  data.Integrations,
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

