package import_module

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/port-labs/port-cli/internal/api"
	"github.com/port-labs/port-cli/internal/modules/export"
)

// Loader loads data from tar.gz or JSON files.
type Loader struct{}

// NewLoader creates a new loader.
func NewLoader() *Loader {
	return &Loader{}
}

// LoadData loads data from a file (tar.gz or JSON).
func (l *Loader) LoadData(inputPath string) (*export.Data, error) {
	// Check if file exists
	if _, err := os.Stat(inputPath); err != nil {
		return nil, fmt.Errorf("input file does not exist: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(inputPath))
	suffixes := strings.Split(strings.ToLower(inputPath), ".")

	isTar := ext == ".gz" || strings.Contains(strings.Join(suffixes, "."), ".tar")
	isJSON := ext == ".json"

	if isTar {
		return l.loadTar(inputPath)
	} else if isJSON {
		return l.loadJSON(inputPath)
	}

	return nil, fmt.Errorf("unsupported file format: %s (expected .json or .tar.gz)", ext)
}

// loadTar loads data from a tar.gz file.
func (l *Loader) loadTar(tarPath string) (*export.Data, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tar file: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	data := &export.Data{
		Blueprints:   []api.Blueprint{},
		Entities:     []api.Entity{},
		Scorecards:   []api.Scorecard{},
		Actions:      []api.Action{},
		Teams:        []api.Team{},
		Users:        []api.User{},
		Pages:        []api.Page{},
		Integrations: []api.Integration{},
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if !strings.HasSuffix(header.Name, ".json") {
			continue
		}

		// Determine data type from filename
		dataType := strings.TrimSuffix(header.Name, ".json")

		// Read file content
		content := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, content); err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read tar file content: %w", err)
		}

		// Parse JSON and assign to appropriate field
		switch dataType {
		case "blueprints":
			var items []api.Blueprint
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse blueprints: %w", err)
			}
			data.Blueprints = items

		case "entities":
			var items []api.Entity
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse entities: %w", err)
			}
			data.Entities = items

		case "scorecards":
			var items []api.Scorecard
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse scorecards: %w", err)
			}
			data.Scorecards = items

		case "actions":
			var items []api.Action
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse actions: %w", err)
			}
			data.Actions = items

		case "teams":
			var items []api.Team
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse teams: %w", err)
			}
			data.Teams = items

		case "users":
			var items []api.User
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse users: %w", err)
			}
			data.Users = items

		case "automations":
			// Backward compatibility: merge automations into actions
			var items []api.Action
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse automations: %w", err)
			}
			data.Actions = append(data.Actions, items...)

		case "pages":
			var items []api.Page
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse pages: %w", err)
			}
			data.Pages = items

		case "integrations":
			var items []api.Integration
			if err := json.Unmarshal(content, &items); err != nil {
				return nil, fmt.Errorf("failed to parse integrations: %w", err)
			}
			data.Integrations = items
		}
	}

	return data, nil
}

// loadJSON loads data from a JSON file.
func (l *Loader) loadJSON(jsonPath string) (*export.Data, error) {
	file, err := os.Open(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON file: %w", err)
	}
	defer file.Close()

	var rawData map[string]interface{}
	if err := json.NewDecoder(file).Decode(&rawData); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	data := &export.Data{
		Blueprints:   []api.Blueprint{},
		Entities:     []api.Entity{},
		Scorecards:   []api.Scorecard{},
		Actions:      []api.Action{},
		Teams:        []api.Team{},
		Users:        []api.User{},
		Pages:        []api.Page{},
		Integrations: []api.Integration{},
	}

	// Convert map[string]interface{} to typed slices
	if blueprints, ok := rawData["blueprints"].([]interface{}); ok {
		for _, bp := range blueprints {
			if bpMap, ok := bp.(map[string]interface{}); ok {
				data.Blueprints = append(data.Blueprints, api.Blueprint(bpMap))
			}
		}
	}

	if entities, ok := rawData["entities"].([]interface{}); ok {
		for _, e := range entities {
			if eMap, ok := e.(map[string]interface{}); ok {
				data.Entities = append(data.Entities, api.Entity(eMap))
			}
		}
	}

	if scorecards, ok := rawData["scorecards"].([]interface{}); ok {
		for _, sc := range scorecards {
			if scMap, ok := sc.(map[string]interface{}); ok {
				data.Scorecards = append(data.Scorecards, api.Scorecard(scMap))
			}
		}
	}

	if actions, ok := rawData["actions"].([]interface{}); ok {
		for _, a := range actions {
			if aMap, ok := a.(map[string]interface{}); ok {
				data.Actions = append(data.Actions, api.Action(aMap))
			}
		}
	}

	if teams, ok := rawData["teams"].([]interface{}); ok {
		for _, t := range teams {
			if tMap, ok := t.(map[string]interface{}); ok {
				data.Teams = append(data.Teams, api.Team(tMap))
			}
		}
	}

	if users, ok := rawData["users"].([]interface{}); ok {
		for _, u := range users {
			if uMap, ok := u.(map[string]interface{}); ok {
				data.Users = append(data.Users, api.User(uMap))
			}
		}
	}

	// Backward compatibility: merge automations into actions
	if automations, ok := rawData["automations"].([]interface{}); ok {
		for _, a := range automations {
			if aMap, ok := a.(map[string]interface{}); ok {
				data.Actions = append(data.Actions, api.Action(aMap))
			}
		}
	}

	if pages, ok := rawData["pages"].([]interface{}); ok {
		for _, p := range pages {
			if pMap, ok := p.(map[string]interface{}); ok {
				data.Pages = append(data.Pages, api.Page(pMap))
			}
		}
	}

	if integrations, ok := rawData["integrations"].([]interface{}); ok {
		for _, i := range integrations {
			if iMap, ok := i.(map[string]interface{}); ok {
				data.Integrations = append(data.Integrations, api.Integration(iMap))
			}
		}
	}

	return data, nil
}

// ValidateData validates the loaded data structure.
func (l *Loader) ValidateData(data *export.Data) error {
	if len(data.Blueprints) == 0 {
		return fmt.Errorf("missing required data: blueprints")
	}
	return nil
}

