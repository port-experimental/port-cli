package resources

// PageNavFields control sidebar placement; Port rejects them when the referenced parent
// doesn't exist yet. These fields are treated specially during import/compare equality.
var PageNavFields = []string{"after", "section", "sidebar", "parent", "requiredQueryParams"}

// pageCompareExcludedFields are always excluded from page equality checks.
var pageCompareExcludedFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "protected"}

// PagesEqual compares two pages for import/compare equality.
//
// Nav fields that are nil/null in the source page are excluded from comparison —
// we don't send null nav fields to Port (sending null clears existing values),
// so a null source nav field should not trigger an update.
//
// requiredQueryParams: null and [] are both treated as "empty" and excluded
// when the source value is empty, since we strip it before sending.
func PagesEqual(sourcePage, currentPage map[string]interface{}) bool {
	exclude := append([]string{}, pageCompareExcludedFields...)

	for _, field := range PageNavFields {
		if field == "requiredQueryParams" {
			sourceVal := sourcePage[field]
			sourceEmpty := sourceVal == nil
			if arr, ok := sourceVal.([]interface{}); ok && len(arr) == 0 {
				sourceEmpty = true
			}
			if sourceEmpty {
				exclude = append(exclude, field)
			}
		} else if v, exists := sourcePage[field]; exists && v == nil {
			exclude = append(exclude, field)
		}
	}

	return ResourcesEqual(sourcePage, currentPage, exclude)
}
