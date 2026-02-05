# Remaining Import Errors Analysis

## Error Category Summary

| Category | Count | Description |
|----------|-------|-------------|
| UNKNOWN | 706 | Unclassified errors (need better categorization) |
| AUTH | 492 | Permission/protected resource errors |
| DEPENDENCY | 289 | Missing blueprints/entities/properties |
| VALIDATION | 183 | Data validation failures |

## Sample Errors by Category

### AUTH Errors (492)

Most are "protected_resource" errors for `_rule_result` entities:
```
Cannot create entity from blueprint "_rule_result" as it is marked protected
```

**Root cause:** System blueprints like `_rule_result` are protected and can't have entities created via API.

**Potential fix:** Skip entities belonging to protected system blueprints during import.

---

### DEPENDENCY Errors (289)

**Type 1: Missing blueprint targets**
```
blueprint jiraIssue: Blueprint with identifier "githubPullRequest" was not found
blueprint service: cannot add dependent fields - missing blueprints: [containerVulnerability snykProject baseImageVulnerability]
blueprint group: cannot add dependent fields - missing blueprints: [dora_deployment dora_incident]
```

**Root cause:** Blueprints reference other blueprints that either:
- Don't exist in the export
- Failed to create due to their own dependencies
- Are part of a cycle

**Potential fix:**
- Warn about missing external dependencies before import
- Better cycle detection and handling

---

**Type 2: Missing property references**
```
blueprint question: Property with identifier "numeric_value" was not found
blueprint action: Property with identifier "savings_cycle_time_h" was not found
```

**Root cause:** Mirror/calculation properties reference properties that don't exist on the target blueprint.

**Potential fix:** Validate property references before attempting update.

---

**Type 3: Missing entity references**
```
entity kubernetes-repo: Entity "ericf+@getport.io" does not exist in blueprint "_user"
entity demo-service-2-repo: Entity "edd1efc4-4bf8-4af6-b234-79aae7040d1c" does not exist in blueprint "snykTarget"
```

**Root cause:** Entities reference other entities that weren't imported (either failed or not in export).

**Potential fix:** Two-phase entity import (strip relations first, add back after all entities created).

---

### VALIDATION Errors (183)

**Type 1: Invalid mirror property paths**
```
blueprint action_run: The path "ran_by_actual_user._owning_teams.group.$title" do not exists
```

**Root cause:** Mirror property references a relation chain that doesn't exist.

---

**Type 2: Missing properties on target blueprints**
```
blueprint response: The "question" Blueprint does not have property with name "avg_score"
```

**Root cause:** Blueprint references a property on another blueprint that doesn't exist.

---

### UNKNOWN Errors (706)

Need to review and improve categorization. Many might be:
- Page creation failures
- Scorecard issues
- Integration config errors

---

## Recommended Improvements

1. **Skip protected resources** - Don't attempt to create entities for `_rule_result` and other protected blueprints

2. **Pre-flight validation** - Before import, check:
   - All blueprint dependencies exist (or will be created)
   - No unresolvable cycles
   - External dependencies are documented

3. **Two-phase entity import** - Strip entity relations in phase 1, add back in phase 2

4. **Better error categorization** - Improve UNKNOWN classification

5. **Dry-run improvements** - Show which resources will fail before attempting import
