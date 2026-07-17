package import_module

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
	"github.com/port-experimental/port-cli/internal/resources"
)

// Sidebar/page apply helpers live here (extracted from import.go):
// folder/page pipeline planning, create/update with narrow fallbacks,
// widget/agent merges, and sidebar parent / after-item error handling.

type SidebarPipelineOperation struct {
	ResourceType string
	Identifier   string
	Folder       api.Folder
	Page         api.Page
}

type SidebarPipelineStep struct {
	Operations []SidebarPipelineOperation
}

func (i *Importer) importSidebarPipeline(ctx context.Context, pipeline []SidebarPipelineStep, result *Result) {
	for _, step := range pipeline {
		pool := NewWorkerPool(DefaultConcurrency)
		for _, op := range step.Operations {
			op := op
			pool.Go(func() {
				switch op.ResourceType {
				case "folder":
					folderID := op.Identifier
					postedFolder := CleanFolderForCreate(op.Folder)
					if err := i.client.CreateFolder(ctx, postedFolder); err != nil && !isConflictError(err) {
						i.mu.Lock()
						i.errors.Add(err, "folder", folderID)
						i.mu.Unlock()
						return
					}
					i.logFolderCreateMismatch(ctx, folderID, postedFolder)
				case "page":
					i.importPage(ctx, op.Page, result)
				}
			})
		}
		pool.Wait()
	}
}

// isSidebarParentNotFound returns true when Port rejects a page because its parent
// sidebar item does not exist in the target organisation.
func isSidebarParentNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Sidebar item")
}

// IsSidebarParentNotFound is the exported form for use by the migrate package.
func IsSidebarParentNotFound(err error) bool {
	return isSidebarParentNotFound(err)
}

// IsAfterItemNotInParent returns true when Port rejects page creation because the
// `after` sibling item doesn't exist inside the specified parent folder.
func IsAfterItemNotInParent(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "after_item_not_in_parent") ||
		strings.Contains(err.Error(), "is not in the parent folder") ||
		strings.Contains(err.Error(), "Sidebar item with after")
}

// IsAgentIdentifierError returns true when the Port API rejects a request because
// a widget is missing the required agentIdentifier field.
func IsAgentIdentifierError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "agentIdentifier")
}

// isAdditionalPropertyError returns true when Port rejects a request because a field
// is not allowed for that page type (e.g. sidebar/requiredQueryParams on entity pages).
func isAdditionalPropertyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "must NOT have additional properties")
}

var additionalPropertyPattern = regexp.MustCompile(`additional property: (?:\\\")?([^"\\]+)(?:\\\")?`)

func extractAdditionalProperty(err error) string {
	if err == nil {
		return ""
	}
	matches := additionalPropertyPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

// IsAdditionalPropertyError is the exported form for use by the migrate package.
func IsAdditionalPropertyError(err error) bool {
	return isAdditionalPropertyError(err)
}

// CleanPageForCreateNoNav is like CleanPageForCreate but also strips navigation
// fields (after, sidebar, parent, section, requiredQueryParams). Used as a fallback
// when the target org is missing the sidebar parent.
func CleanPageForCreateNoNav(page api.Page) api.Page {
	strip := append(pageMetaFields, resources.PageNavFields...)
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// MergeWidgetAgentIdentifiers copies agentIdentifier values from existing
// widgets into new widgets so that Port's required-field validation passes.
func MergeWidgetAgentIdentifiers(newWidgets, existingWidgets []interface{}) []interface{} {
	return mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
}

// SortPagesByAfterDeps is the exported version of sortPagesByAfterDeps for use by migrate.
func SortPagesByAfterDeps(pages []api.Page) []api.Page {
	return sortPagesByAfterDeps(pages)
}

func SortFoldersByAfterLevels(folders []api.Folder) [][]api.Folder {
	pipeline := PlanSidebarPipeline(folders, nil)
	levels := make([][]api.Folder, 0, len(pipeline))
	for _, step := range pipeline {
		level := make([]api.Folder, 0, len(step.Operations))
		for _, op := range step.Operations {
			if op.ResourceType == "folder" {
				level = append(level, op.Folder)
			}
		}
		if len(level) > 0 {
			levels = append(levels, level)
		}
	}
	return levels
}

func PlanSidebarPipeline(folders []api.Folder, pages []api.Page) []SidebarPipelineStep {
	opsByID := make(map[string]SidebarPipelineOperation, len(folders)+len(pages))
	inDegree := make(map[string]int, len(folders)+len(pages))
	dependents := make(map[string][]string, len(folders)+len(pages))

	for _, folder := range folders {
		id, _ := folder["identifier"].(string)
		if id == "" {
			continue
		}
		opsByID[id] = SidebarPipelineOperation{
			ResourceType: "folder",
			Identifier:   id,
			Folder:       folder,
		}
		inDegree[id] = 0
	}
	for _, page := range pages {
		id, _ := page["identifier"].(string)
		if id == "" {
			continue
		}
		opsByID[id] = SidebarPipelineOperation{
			ResourceType: "page",
			Identifier:   id,
			Page:         page,
		}
		inDegree[id] = 0
	}

	addDependency := func(id, dep string) {
		if id == "" || dep == "" || id == dep {
			return
		}
		if _, exists := opsByID[dep]; !exists {
			return
		}
		inDegree[id]++
		dependents[dep] = append(dependents[dep], id)
	}

	for _, folder := range folders {
		id, _ := folder["identifier"].(string)
		if id == "" {
			continue
		}
		deps := make(map[string]bool)
		if parent, _ := folder["parent"].(string); parent != "" {
			deps[parent] = true
		}
		if after, _ := folder["after"].(string); after != "" {
			deps[after] = true
		}
		for dep := range deps {
			addDependency(id, dep)
		}
	}
	for _, page := range pages {
		id, _ := page["identifier"].(string)
		if id == "" {
			continue
		}
		deps := make(map[string]bool)
		if parent, _ := page["parent"].(string); parent != "" {
			deps[parent] = true
		}
		if after, _ := page["after"].(string); after != "" {
			deps[after] = true
		}
		for dep := range deps {
			addDependency(id, dep)
		}
	}

	remaining := make(map[string]bool, len(opsByID))
	for id := range opsByID {
		remaining[id] = true
	}

	var steps []SidebarPipelineStep
	for len(remaining) > 0 {
		readyIDs := make([]string, 0, len(remaining))
		for id := range remaining {
			if inDegree[id] == 0 {
				readyIDs = append(readyIDs, id)
			}
		}
		if len(readyIDs) == 0 {
			for id := range remaining {
				readyIDs = append(readyIDs, id)
			}
		}
		sort.Strings(readyIDs)

		step := SidebarPipelineStep{
			Operations: make([]SidebarPipelineOperation, 0, len(readyIDs)),
		}
		for _, id := range readyIDs {
			step.Operations = append(step.Operations, opsByID[id])
		}
		steps = append(steps, step)

		for _, id := range readyIDs {
			delete(remaining, id)
			for _, dep := range dependents[id] {
				inDegree[dep]--
			}
		}
	}

	return steps
}

func DescribeSidebarPipeline(steps []SidebarPipelineStep) []string {
	lines := make([]string, 0, len(steps))
	for idx, step := range steps {
		var folders []string
		var pages []string
		for _, op := range step.Operations {
			switch op.ResourceType {
			case "folder":
				folders = append(folders, op.Identifier)
			case "page":
				pages = append(pages, op.Identifier)
			}
		}
		sort.Strings(folders)
		sort.Strings(pages)

		parts := make([]string, 0, 2)
		if len(folders) > 0 {
			parts = append(parts, fmt.Sprintf("folders [%s]", strings.Join(folders, ", ")))
		}
		if len(pages) > 0 {
			parts = append(parts, fmt.Sprintf("pages [%s]", strings.Join(pages, ", ")))
		}
		lines = append(lines, fmt.Sprintf("Step %d: %s", idx+1, strings.Join(parts, " | ")))
	}
	return lines
}

// sortPagesByAfterDeps returns pages sorted so that if page B has after=A and A is
// also in the list, A comes before B.
func sortPagesByAfterDeps(pages []api.Page) []api.Page {
	pageSet := make(map[string]bool, len(pages))
	for _, p := range pages {
		if id, ok := p["identifier"].(string); ok {
			pageSet[id] = true
		}
	}

	result := make([]api.Page, 0, len(pages))
	placed := make(map[string]bool, len(pages))
	remaining := make([]api.Page, len(pages))
	copy(remaining, pages)

	for len(remaining) > 0 {
		added := 0
		var next []api.Page
		for _, p := range remaining {
			after, _ := p["after"].(string)
			if !pageSet[after] || placed[after] {
				result = append(result, p)
				if id, ok := p["identifier"].(string); ok {
					placed[id] = true
				}
				added++
			} else {
				next = append(next, p)
			}
		}
		remaining = next
		if added == 0 {
			result = append(result, remaining...)
			break
		}
	}
	return result
}

func CleanFolderForCreate(folder api.Folder) api.Folder {
	cleaned := make(api.Folder)
	for _, key := range []string{"identifier", "title", "after", "parent"} {
		if value, ok := folder[key]; ok && value != nil {
			cleaned[key] = value
		}
	}
	return cleaned
}

// sortPagesByAfterLevels groups pages into levels where all pages within a level
// have no after-dependencies on each other. Pages in the same level can be
// processed concurrently; levels must be processed sequentially.
func sortPagesByAfterLevels(pages []api.Page) [][]api.Page {
	pageSet := make(map[string]bool, len(pages))
	pageByID := make(map[string]api.Page, len(pages))
	for _, p := range pages {
		if id, ok := p["identifier"].(string); ok && id != "" {
			pageSet[id] = true
			pageByID[id] = p
		}
	}

	inDegree := make(map[string]int, len(pages))
	dependents := make(map[string][]string, len(pages))
	for _, p := range pages {
		id, _ := p["identifier"].(string)
		if id == "" {
			continue
		}
		inDegree[id] = 0
	}
	for _, p := range pages {
		id, _ := p["identifier"].(string)
		after, _ := p["after"].(string)
		if id == "" || after == "" || !pageSet[after] {
			continue
		}
		inDegree[id]++
		dependents[after] = append(dependents[after], id)
	}

	remaining := make(map[string]bool, len(pages))
	for id := range inDegree {
		remaining[id] = true
	}

	var levels [][]api.Page
	for len(remaining) > 0 {
		var level []api.Page
		for id := range remaining {
			if inDegree[id] == 0 {
				level = append(level, pageByID[id])
			}
		}
		if len(level) == 0 {
			// Cycle — flush all remaining to break deadlock.
			for id := range remaining {
				level = append(level, pageByID[id])
			}
		}
		for _, p := range level {
			id, _ := p["identifier"].(string)
			delete(remaining, id)
			for _, dep := range dependents[id] {
				inDegree[dep]--
			}
		}
		levels = append(levels, level)
	}
	return levels
}

// applyPageOrdering applies the `after` field for pages that have one, sequentially
// and in topological dependency order. This is called after the concurrent page
// content pass so that sidebar ordering is set without race conditions.
func (i *Importer) applyPageOrdering(ctx context.Context, pages []api.Page, result *Result) {
	// Collect pages that have a non-empty after value.
	var toOrder []api.Page
	for _, p := range pages {
		if after, ok := p["after"].(string); ok && after != "" {
			toOrder = append(toOrder, p)
		}
	}
	if len(toOrder) == 0 {
		return
	}

	sorted := sortPagesByAfterDeps(toOrder)
	for _, p := range sorted {
		pageID, ok := p["identifier"].(string)
		if !ok || pageID == "" {
			continue
		}
		after := p["after"].(string)
		_, err := i.client.UpdatePage(ctx, pageID, api.Page{"identifier": pageID, "after": after})
		if err != nil {
			// A missing sibling is benign — the page exists, just not in the exact spot.
			if !isSidebarParentNotFound(err) {
				i.errors.Add(err, "page", pageID)
			}
		}
	}
}

// importPages imports pages in topological `after` order.
// Pages are grouped into levels: all pages in a level are independent of each
// other (no after-dependency between them) and can run concurrently. Levels are
// processed sequentially so that `after` targets are always present before their
// dependents. This avoids race conditions without a separate second pass.
func (i *Importer) importPages(ctx context.Context, pages []api.Page, result *Result) {
	levels := sortPagesByAfterLevels(pages)
	for _, level := range levels {
		pool := NewWorkerPool(DefaultConcurrency)
		for _, page := range level {
			page := page
			pool.Go(func() {
				i.importPage(ctx, page, result)
			})
		}
		pool.Wait()
	}
}

// importPage imports a single page.
func (i *Importer) importPage(ctx context.Context, page api.Page, result *Result) {
	pageID, ok := page["identifier"].(string)
	if !ok || pageID == "" {
		return
	}

	// metaFields are always stripped (audit metadata, internal IDs).
	metaFields := []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected"}
	// navFields control sidebar placement.
	navFields := []string{"after", "section", "sidebar", "parent", "requiredQueryParams"}

	// buildPage strips the given extra fields and recursively cleans widget metadata.
	buildPage := func(extra []string) api.Page {
		strip := append(metaFields, extra...)
		cleaned := cleanSystemFields(page, strip)
		if widgets, ok := cleaned["widgets"].([]interface{}); ok {
			cleaned["widgets"] = cleanWidgetsRecursive(widgets)
		}
		return api.Page(cleaned)
	}

	// pageForCreate keeps `type` and sidebar placement fields, but strips
	// requiredQueryParams because Port rejects it for some page types on create.
	pageForCreate := buildPage([]string{"requiredQueryParams"})
	// pageForUpdate keeps navigation fields (including `after`) so Port places the
	// page in the correct sidebar position. `type` is stripped because the PATCH
	// endpoint rejects it. Null string nav fields are stripped to avoid clearing
	// existing values in the target.
	pageForUpdate := buildPage([]string{"type", "requiredQueryParams", "sidebar"})
	for _, field := range navFields {
		if v, exists := pageForUpdate[field]; exists && v == nil {
			delete(pageForUpdate, field)
		}
	}
	var (
		createPosted api.Page
		createdPage  api.Page
		err          error
	)

	createPosted = pageForCreate
	createdPage, err = i.client.CreatePage(ctx, createPosted)

	needsUpdate := false
	i.mu.Lock()
	if err == nil {
		result.PagesCreated++
		i.mu.Unlock()
		i.logPageCreateMismatch(ctx, pageID, pageForCreate, createPosted, createdPage)
		return
	} else if IsAfterItemNotInParent(err) || extractAdditionalProperty(err) != "" {
		createPosted, createdPage, retryErr := i.retryCreatePageWithNarrowFallbacks(ctx, pageForCreate, err)
		if retryErr == nil {
			result.PagesCreated++
			i.mu.Unlock()
			i.logPageCreateMismatch(ctx, pageID, pageForCreate, createPosted, createdPage)
			return
		} else if isConflictError(retryErr) {
			needsUpdate = true
		}
	} else if isConflictError(err) {
		needsUpdate = true
	} else if strings.Contains(err.Error(), "agentIdentifier") {
		// Create failed with agentIdentifier — check if the page already exists.
		existingPage, fetchErr := i.client.GetPage(ctx, pageID)
		if fetchErr == nil && existingPage != nil {
			pageWithoutWidgets := make(api.Page)
			for k, v := range pageForUpdate {
				if k != "widgets" {
					pageWithoutWidgets[k] = v
				}
			}
			_, updateErr := i.client.UpdatePage(ctx, pageID, pageWithoutWidgets)
			if updateErr != nil {
				i.errors.Add(updateErr, "page", pageID)
			} else {
				result.PagesUpdated++
			}
		} else {
			i.errors.Add(err, "page", pageID)
		}
	} else {
		i.errors.Add(err, "page", pageID)
	}

	if needsUpdate {
		// Fetch existing page to preserve fields like agentIdentifier.
		existingPage, fetchErr := i.client.GetPage(ctx, pageID)
		if fetchErr == nil && existingPage != nil {
			if existingWidgets, ok := existingPage["widgets"].([]interface{}); ok {
				if newWidgets, ok := pageForUpdate["widgets"].([]interface{}); ok {
					pageForUpdate["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
				}
			}
		}

		_, updateErr := i.client.UpdatePage(ctx, pageID, pageForUpdate)
		if updateErr != nil {
			if IsAfterItemNotInParent(updateErr) || extractAdditionalProperty(updateErr) != "" {
				_, retryErr := i.retryUpdatePageWithNarrowFallbacks(ctx, pageID, pageForUpdate, updateErr)
				if retryErr != nil {
					i.errors.Add(retryErr, "page", pageID)
				} else {
					result.PagesUpdated++
				}
			} else if strings.Contains(updateErr.Error(), "agentIdentifier") {
				// Fetch existing page to merge agentIdentifiers from its widgets, then retry.
				if existingPage, fetchErr := i.client.GetPage(ctx, pageID); fetchErr == nil && existingPage != nil {
					if existingWidgets, ok := existingPage["widgets"].([]interface{}); ok {
						if newWidgets, ok := pageForUpdate["widgets"].([]interface{}); ok {
							pageForUpdate["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
						}
					}
				}
				_, retryErr := i.client.UpdatePage(ctx, pageID, pageForUpdate)
				if retryErr != nil {
					// Last resort: update without widgets.
					pageWithoutWidgets := make(api.Page)
					for k, v := range pageForUpdate {
						if k != "widgets" {
							pageWithoutWidgets[k] = v
						}
					}
					_, lastErr := i.client.UpdatePage(ctx, pageID, pageWithoutWidgets)
					if lastErr != nil {
						i.errors.Add(lastErr, "page", pageID)
					} else {
						result.PagesUpdated++
					}
				} else {
					result.PagesUpdated++
				}
			} else {
				i.errors.Add(updateErr, "page", pageID)
			}
		} else {
			result.PagesUpdated++
		}
	}
	i.mu.Unlock()
}

func (i *Importer) logPageCreateMismatch(ctx context.Context, pageID string, intended api.Page, posted api.Page, created api.Page) {
	if !i.verbose {
		return
	}
	actualPage, err := i.client.GetPage(ctx, pageID)
	if err == nil && actualPage != nil {
		created = actualPage
	}

	normalizedIntended := normalizePageForLog(intended)
	normalizedPosted := normalizePageForLog(posted)
	normalizedCreated := normalizePageForLog(created)

	if subsetEqual(normalizedPosted, normalizedCreated) && subsetEqual(normalizedIntended, normalizedCreated) {
		return
	}

	lines := []string{
		fmt.Sprintf("Page create mismatch for %s", pageID),
	}
	if !subsetEqual(normalizedIntended, normalizedPosted) {
		lines = append(lines, fmt.Sprintf("  intended: %s", mustJSON(normalizedIntended)))
	}
	lines = append(lines,
		fmt.Sprintf("  posted: %s", mustJSON(normalizedPosted)),
		fmt.Sprintf("  created: %s", mustJSON(normalizedCreated)),
	)
	i.logLines(lines)
}

func (i *Importer) logFolderCreateMismatch(ctx context.Context, folderID string, posted api.Folder) {
	if !i.verbose {
		return
	}
	folders, err := i.client.GetFolders(ctx)
	if err != nil {
		i.logLines([]string{
			fmt.Sprintf("Folder create mismatch check failed for %s", folderID),
			fmt.Sprintf("  posted: %s", mustJSON(normalizeFolderForLog(posted))),
			fmt.Sprintf("  error: %v", err),
		})
		return
	}

	var actual api.Folder
	for _, folder := range folders {
		if identifier, _ := folder["identifier"].(string); identifier == folderID {
			actual = folder
			break
		}
	}
	if actual == nil {
		i.logLines([]string{
			fmt.Sprintf("Folder create mismatch for %s", folderID),
			fmt.Sprintf("  posted: %s", mustJSON(normalizeFolderForLog(posted))),
			"  created: null",
		})
		return
	}

	normalizedPosted := normalizeFolderForLog(posted)
	normalizedActual := normalizeFolderForLog(actual)
	if subsetEqual(normalizedPosted, normalizedActual) {
		return
	}

	i.logLines([]string{
		fmt.Sprintf("Folder create mismatch for %s", folderID),
		fmt.Sprintf("  posted: %s", mustJSON(normalizedPosted)),
		fmt.Sprintf("  created: %s", mustJSON(normalizedActual)),
	})
}

func (i *Importer) logLines(lines []string) {
	if i.log == nil {
		return
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	for _, line := range lines {
		i.log(line)
	}
}

func normalizePageForLog(page api.Page) api.Page {
	if page == nil {
		return nil
	}
	normalized := make(api.Page)
	for _, key := range []string{"identifier", "title", "type", "after", "parent", "section", "sidebar", "showInSidebar", "requiredQueryParams"} {
		if value, ok := page[key]; ok && value != nil {
			normalized[key] = value
		}
	}
	return normalized
}

func clonePage(page api.Page) api.Page {
	if page == nil {
		return nil
	}

	cloned := make(api.Page, len(page))
	for key, value := range page {
		cloned[key] = value
	}
	return cloned
}

func (i *Importer) retryCreatePageWithNarrowFallbacks(ctx context.Context, base api.Page, initialErr error) (api.Page, api.Page, error) {
	candidate := clonePage(base)
	currentErr := initialErr

	for {
		nextCandidate, changed := removeSingleFailingPageField(candidate, currentErr)
		if !changed {
			return candidate, nil, currentErr
		}

		createdPage, retryErr := i.client.CreatePage(ctx, nextCandidate)
		if retryErr == nil {
			return nextCandidate, createdPage, nil
		}
		if isConflictError(retryErr) {
			return nextCandidate, nil, retryErr
		}

		candidate = nextCandidate
		currentErr = retryErr
	}
}

func (i *Importer) retryUpdatePageWithNarrowFallbacks(ctx context.Context, pageID string, base api.Page, initialErr error) (api.Page, error) {
	candidate := clonePage(base)
	currentErr := initialErr

	for {
		nextCandidate, changed := removeSingleFailingPageField(candidate, currentErr)
		if !changed {
			return candidate, currentErr
		}

		updatedPage, retryErr := i.client.UpdatePage(ctx, pageID, nextCandidate)
		if retryErr == nil {
			return updatedPage, nil
		}

		candidate = nextCandidate
		currentErr = retryErr
	}
}

func removeSingleFailingPageField(page api.Page, err error) (api.Page, bool) {
	candidate := clonePage(page)

	if IsAfterItemNotInParent(err) {
		// Explicitly null out `after` so the PATCH clears any existing invalid
		// value in the target, rather than leaving it unchanged by omission.
		candidate["after"] = nil
		return candidate, true
	}

	if invalidProperty := extractAdditionalProperty(err); invalidProperty != "" {
		if _, exists := candidate[invalidProperty]; exists {
			delete(candidate, invalidProperty)
			return candidate, true
		}
	}

	return page, false
}

func normalizeFolderForLog(folder api.Folder) api.Folder {
	if folder == nil {
		return nil
	}

	normalized := make(api.Folder)
	for _, key := range []string{"identifier", "title", "after", "parent", "sidebar", "section", "showInSidebar"} {
		if value, ok := folder[key]; ok && value != nil {
			normalized[key] = value
		}
	}
	return normalized
}

func subsetEqual(expected, actual interface{}) bool {
	switch actualTyped := actual.(type) {
	case api.Page:
		return subsetEqual(expected, map[string]interface{}(actualTyped))
	case api.Folder:
		return subsetEqual(expected, map[string]interface{}(actualTyped))
	}

	switch expectedTyped := expected.(type) {
	case map[string]interface{}:
		actualTyped, ok := actual.(map[string]interface{})
		if !ok {
			return false
		}
		for key, expectedValue := range expectedTyped {
			actualValue, exists := actualTyped[key]
			if !exists || !subsetEqual(expectedValue, actualValue) {
				return false
			}
		}
		return true
	case api.Page:
		return subsetEqual(map[string]interface{}(expectedTyped), actual)
	case api.Folder:
		return subsetEqual(map[string]interface{}(expectedTyped), actual)
	case []interface{}:
		actualTyped, ok := actual.([]interface{})
		if !ok || len(expectedTyped) != len(actualTyped) {
			return false
		}
		for idx := range expectedTyped {
			if !subsetEqual(expectedTyped[idx], actualTyped[idx]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(expected, actual)
	}
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

// pageMetaFields are the audit/internal fields always stripped before sending a page to Port.
var pageMetaFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected"}

// CleanPageForCreate returns a copy of page with audit/internal fields removed.
// Sidebar placement fields are preserved, but requiredQueryParams is stripped
// because Port rejects it for some page types on create.
func CleanPageForCreate(page api.Page) api.Page {
	strip := append(pageMetaFields, "requiredQueryParams")
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// CleanPageForUpdate returns a copy of page with audit/internal fields and `type`
// removed. Navigation fields are kept so Port can move the page to the correct
// sidebar position, except requiredQueryParams and sidebar which are stripped by
// default because Port rejects them for some page types on update. Nav fields
// that are nil/null are also stripped — sending null would clear the page's
// existing navigation context in Port.
func CleanPageForUpdate(page api.Page) api.Page {
	strip := append(pageMetaFields, "type", "requiredQueryParams", "sidebar")
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	for _, field := range resources.PageNavFields {
		if v, exists := cleaned[field]; exists && v == nil {
			delete(cleaned, field)
		}
	}
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// CleanPageForUpdateNoNav is the fallback for CleanPageForUpdate when Port rejects
// the update because the parent page doesn't exist in the target org.
func CleanPageForUpdateNoNav(page api.Page) api.Page {
	strip := append(pageMetaFields, append(resources.PageNavFields, "type")...)
	cleaned := cleanSystemFields(map[string]interface{}(page), strip)
	if widgets, ok := cleaned["widgets"].([]interface{}); ok {
		cleaned["widgets"] = cleanWidgetsRecursive(widgets)
	}
	return api.Page(cleaned)
}

// cleanWidgetsRecursive removes system fields from widgets and their nested widgets.
// It also fixes widget configurations that would cause validation errors.
func cleanWidgetsRecursive(widgets []interface{}) []interface{} {
	systemFields := map[string]bool{
		"createdBy": true, "updatedBy": true, "createdAt": true, "updatedAt": true,
	}

	result := make([]interface{}, 0, len(widgets))
	for _, w := range widgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			result = append(result, w)
			continue
		}

		// Clean system fields from this widget
		cleaned := make(map[string]interface{})
		for k, v := range widget {
			if systemFields[k] {
				continue
			}
			// Recursively clean nested widgets
			if k == "widgets" {
				if nestedWidgets, ok := v.([]interface{}); ok {
					cleaned[k] = cleanWidgetsRecursive(nestedWidgets)
					continue
				}
			}
			// Recursively clean groups (which contain widgets)
			if k == "groups" {
				if groups, ok := v.([]interface{}); ok {
					cleanedGroups := make([]interface{}, 0, len(groups))
					for _, g := range groups {
						if group, ok := g.(map[string]interface{}); ok {
							cleanedGroup := make(map[string]interface{})
							for gk, gv := range group {
								if gk == "widgets" {
									if groupWidgets, ok := gv.([]interface{}); ok {
										cleanedGroup[gk] = cleanWidgetsRecursive(groupWidgets)
										continue
									}
								}
								cleanedGroup[gk] = gv
							}
							cleanedGroups = append(cleanedGroups, cleanedGroup)
						} else {
							cleanedGroups = append(cleanedGroups, g)
						}
					}
					cleaned[k] = cleanedGroups
					continue
				}
			}
			cleaned[k] = v
		}

		// Fix table-entities-explorer widgets that have dataset but no blueprint
		// The API requires either a blueprint property or a blueprint rule in the dataset
		widgetType, _ := cleaned["type"].(string)
		if widgetType == "table-entities-explorer" {
			_, hasBlueprint := cleaned["blueprint"]
			_, hasDataset := cleaned["dataset"]
			if hasDataset && !hasBlueprint {
				// Add empty blueprint to indicate cross-blueprint dataset query
				cleaned["blueprint"] = ""
			}
		}

		result = append(result, cleaned)
	}
	return result
}

// mergeWidgetAgentIdentifiers copies agentIdentifier from existing widgets to new widgets.
// This is needed because the API now requires agentIdentifier on certain widget types,
// but exported data may not have it.
func mergeWidgetAgentIdentifiers(newWidgets, existingWidgets []interface{}) []interface{} {
	// Build a map of existing widgets by ID for quick lookup
	existingByID := make(map[string]map[string]interface{})
	for _, w := range existingWidgets {
		if widget, ok := w.(map[string]interface{}); ok {
			if id, ok := widget["id"].(string); ok && id != "" {
				existingByID[id] = widget
			}
		}
	}

	result := make([]interface{}, 0, len(newWidgets))
	for idx, w := range newWidgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			result = append(result, w)
			continue
		}

		// Try to find matching existing widget by ID
		var existingWidget map[string]interface{}
		if id, ok := widget["id"].(string); ok && id != "" {
			existingWidget = existingByID[id]
		}
		// Fallback to index-based matching if no ID match
		if existingWidget == nil && idx < len(existingWidgets) {
			if ew, ok := existingWidgets[idx].(map[string]interface{}); ok {
				existingWidget = ew
			}
		}

		// Copy agentIdentifier from existing widget if present and not in new widget
		if existingWidget != nil {
			if agentID, ok := existingWidget["agentIdentifier"]; ok {
				if _, hasAgentID := widget["agentIdentifier"]; !hasAgentID {
					widget["agentIdentifier"] = agentID
				}
			}
		}

		// Recursively merge nested widgets
		if newNestedWidgets, ok := widget["widgets"].([]interface{}); ok {
			var existingNestedWidgets []interface{}
			if existingWidget != nil {
				existingNestedWidgets, _ = existingWidget["widgets"].([]interface{})
			}
			if existingNestedWidgets != nil {
				widget["widgets"] = mergeWidgetAgentIdentifiers(newNestedWidgets, existingNestedWidgets)
			}
		}

		// Recursively merge groups
		if newGroups, ok := widget["groups"].([]interface{}); ok {
			var existingGroups []interface{}
			if existingWidget != nil {
				existingGroups, _ = existingWidget["groups"].([]interface{})
			}
			if existingGroups != nil {
				widget["groups"] = mergeGroupAgentIdentifiers(newGroups, existingGroups)
			}
		}

		result = append(result, widget)
	}
	return result
}

// mergeGroupAgentIdentifiers merges agentIdentifier for widgets within groups.
func mergeGroupAgentIdentifiers(newGroups, existingGroups []interface{}) []interface{} {
	// Build a map of existing groups by title for matching
	existingByTitle := make(map[string]map[string]interface{})
	for _, g := range existingGroups {
		if group, ok := g.(map[string]interface{}); ok {
			if title, ok := group["title"].(string); ok && title != "" {
				existingByTitle[title] = group
			}
		}
	}

	result := make([]interface{}, 0, len(newGroups))
	for idx, g := range newGroups {
		group, ok := g.(map[string]interface{})
		if !ok {
			result = append(result, g)
			continue
		}

		// Try to find matching existing group by title
		var existingGroup map[string]interface{}
		if title, ok := group["title"].(string); ok && title != "" {
			existingGroup = existingByTitle[title]
		}
		// Fallback to index-based matching
		if existingGroup == nil && idx < len(existingGroups) {
			if eg, ok := existingGroups[idx].(map[string]interface{}); ok {
				existingGroup = eg
			}
		}

		// Recursively merge widgets within the group
		if newWidgets, ok := group["widgets"].([]interface{}); ok {
			var existingWidgets []interface{}
			if existingGroup != nil {
				existingWidgets, _ = existingGroup["widgets"].([]interface{})
			}
			if existingWidgets != nil {
				group["widgets"] = mergeWidgetAgentIdentifiers(newWidgets, existingWidgets)
			}
		}

		result = append(result, group)
	}
	return result
}
