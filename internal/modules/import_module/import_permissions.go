package import_module

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

// Permissions apply helpers (extracted from import.go): invalid-field parsing,
// sanitization retries, and blueprint/action/page permission updates.

// isInvalidPermissionsError returns true when the Port API rejects a permissions
// PATCH because the payload references relations or properties that don't exist
// on the target blueprint (e.g., orphaned scorecard relations on _rule_result).
func isInvalidPermissionsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "invalid_permissions")
}

// IsInvalidPermissionsError is the exported form for use by the migrate package.
func IsInvalidPermissionsError(err error) bool {
	return isInvalidPermissionsError(err)
}

// invalidPermBodyPattern extracts the JSON body from the API error string.
// The error format is: "API request to ... failed: 422 ... Body: {JSON}"
var invalidPermBodyPattern = regexp.MustCompile(`(?s)Body: (\{.*\})`)

// ParseInvalidPermissionFields extracts the invalidRelations and
// invalidProperties arrays from an invalid_permissions API error.
// Returns nil slices when the error is not parseable or not an
// invalid_permissions error.
func ParseInvalidPermissionFields(err error) (relations, properties []string) {
	if err == nil {
		return nil, nil
	}
	matches := invalidPermBodyPattern.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return nil, nil
	}
	var body struct {
		Error   string `json:"error"`
		Details struct {
			InvalidRelations  []string `json:"invalidRelations"`
			InvalidProperties []string `json:"invalidProperties"`
		} `json:"details"`
	}
	if json.Unmarshal([]byte(matches[1]), &body) != nil {
		return nil, nil
	}
	if body.Error != "invalid_permissions" {
		return nil, nil
	}
	return body.Details.InvalidRelations, body.Details.InvalidProperties
}

// SanitizePermissions returns a deep copy of perms with the named relation
// and property keys removed. Invalid relations are stripped from top-level
// keys and from entities.updateRelations; invalid properties are stripped
// from top-level keys and from entities.updateProperties.
func SanitizePermissions(perms api.Permissions, invalidRelations, invalidProperties []string) api.Permissions {
	relSet := make(map[string]bool, len(invalidRelations))
	for _, r := range invalidRelations {
		relSet[r] = true
	}
	propSet := make(map[string]bool, len(invalidProperties))
	for _, p := range invalidProperties {
		propSet[p] = true
	}

	// Deep-copy and strip top-level keys
	cleaned := make(api.Permissions, len(perms))
	for k, v := range perms {
		if relSet[k] || propSet[k] {
			continue
		}
		cleaned[k] = v
	}

	// Strip nested relation/property keys inside entities.updateRelations
	// and entities.updateProperties where the API actually validates them.
	entities, ok := cleaned["entities"].(map[string]interface{})
	if !ok {
		return cleaned
	}
	entitiesCopy := make(map[string]interface{}, len(entities))
	for k, v := range entities {
		entitiesCopy[k] = v
	}

	if ur, ok := entitiesCopy["updateRelations"].(map[string]interface{}); ok && len(relSet) > 0 {
		urCopy := make(map[string]interface{}, len(ur))
		for k, v := range ur {
			if !relSet[k] {
				urCopy[k] = v
			}
		}
		entitiesCopy["updateRelations"] = urCopy
	}

	if up, ok := entitiesCopy["updateProperties"].(map[string]interface{}); ok && len(propSet) > 0 {
		upCopy := make(map[string]interface{}, len(up))
		for k, v := range up {
			if !propSet[k] {
				upCopy[k] = v
			}
		}
		entitiesCopy["updateProperties"] = upCopy
	}

	cleaned["entities"] = entitiesCopy
	return cleaned
}

func (i *Importer) importPermissions(ctx context.Context, applyCtx ApplyContext) (bpUpdated, actionUpdated, pageUpdated int, warnings []string) {
	// Import blueprint permissions
	for _, change := range applyCtx.BlueprintPermissions {
		perms := change.Permissions
		_, err := i.client.UpdateBlueprintPermissions(ctx, change.Identifier, perms)
		if err != nil && isInvalidPermissionsError(err) {
			relations, properties := ParseInvalidPermissionFields(err)
			if len(relations) > 0 || len(properties) > 0 {
				warnings = append(warnings, fmt.Sprintf("Stripped orphaned fields from %s permissions: relations=%v properties=%v", change.Identifier, relations, properties))
				perms = SanitizePermissions(perms, relations, properties)
				_, err = i.client.UpdateBlueprintPermissions(ctx, change.Identifier, perms)
			}
		}
		if err != nil {
			i.errors.Add(fmt.Errorf("failed to update blueprint permissions for %s: %w", change.Identifier, err), "blueprint_permissions", change.Identifier)
		} else {
			bpUpdated++
		}
	}

	// Import action permissions
	for _, change := range applyCtx.ActionPermissions {
		perms := change.Permissions
		_, err := i.client.UpdateActionPermissions(ctx, change.Identifier, perms)
		if err != nil && isInvalidPermissionsError(err) {
			relations, properties := ParseInvalidPermissionFields(err)
			if len(relations) > 0 || len(properties) > 0 {
				warnings = append(warnings, fmt.Sprintf("Stripped orphaned fields from %s action permissions: relations=%v properties=%v", change.Identifier, relations, properties))
				perms = SanitizePermissions(perms, relations, properties)
				_, err = i.client.UpdateActionPermissions(ctx, change.Identifier, perms)
			}
		}
		if err != nil {
			i.errors.Add(fmt.Errorf("failed to update action permissions for %s: %w", change.Identifier, err), "action_permissions", change.Identifier)
		} else {
			actionUpdated++
		}
	}

	// Import page permissions
	for _, change := range applyCtx.PagePermissions {
		perms := change.Permissions
		_, err := i.client.UpdatePagePermissions(ctx, change.Identifier, perms)
		if err != nil && isInvalidPermissionsError(err) {
			relations, properties := ParseInvalidPermissionFields(err)
			if len(relations) > 0 || len(properties) > 0 {
				warnings = append(warnings, fmt.Sprintf("Stripped orphaned fields from %s page permissions: relations=%v properties=%v", change.Identifier, relations, properties))
				perms = SanitizePermissions(perms, relations, properties)
				_, err = i.client.UpdatePagePermissions(ctx, change.Identifier, perms)
			}
		}
		if err != nil {
			i.errors.Add(fmt.Errorf("failed to update page permissions for %s: %w", change.Identifier, err), "page_permissions", change.Identifier)
		} else {
			pageUpdated++
		}
	}
	return
}
