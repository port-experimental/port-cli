package resources

import (
	"encoding/json"
	"reflect"
	"sort"
)

// CompareExcludedFields contains fields excluded from org-to-org compare diffs.
// These are typically auto-generated or organization-specific.
var CompareExcludedFields = map[string]bool{
	"_id":       true,
	"id":        true,
	"orgId":     true,
	"createdAt": true,
	"createdBy": true,
	"updatedAt": true,
	"updatedBy": true,
	"icon":      true,
	"color":     true,
}

// IsCompareExcluded reports whether a field name should be skipped during compare.
func IsCompareExcluded(field string) bool {
	return CompareExcludedFields[field]
}

// NormalizeResource removes server-managed fields and normalizes nested values.
func NormalizeResource(resource map[string]interface{}, systemFields []string) map[string]interface{} {
	normalized := make(map[string]interface{})
	removeSet := make(map[string]bool)
	for _, f := range systemFields {
		removeSet[f] = true
	}

	for k, v := range resource {
		if !removeSet[k] {
			normalized[k] = NormalizeValue(v)
		}
	}

	return normalized
}

// NormalizeValue recursively normalizes a value for comparison.
func NormalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{})
		for k, v := range val {
			normalized[k] = NormalizeValue(v)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(val))
		for i, item := range val {
			normalized[i] = NormalizeValue(item)
		}
		if len(normalized) > 0 {
			allStrings := true
			for _, item := range normalized {
				if _, ok := item.(string); !ok {
					allStrings = false
					break
				}
			}
			if allStrings {
				sort.Slice(normalized, func(i, j int) bool {
					return normalized[i].(string) < normalized[j].(string)
				})
			}
		}
		return normalized
	default:
		return v
	}
}

// ResourcesEqual checks if two resources are equal after normalization.
func ResourcesEqual(a, b map[string]interface{}, systemFields []string) bool {
	normA := NormalizeResource(a, systemFields)
	normB := NormalizeResource(b, systemFields)

	jsonA, errA := json.Marshal(normA)
	jsonB, errB := json.Marshal(normB)

	if errA != nil || errB != nil {
		return reflect.DeepEqual(normA, normB)
	}

	return string(jsonA) == string(jsonB)
}
