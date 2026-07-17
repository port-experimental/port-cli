package import_module

import (
	"context"
	"fmt"
	"strings"

	"github.com/port-experimental/port-cli/internal/api"
)

// Non-blueprint/page resource apply helpers (extracted from import.go):
// scorecards, actions, teams, users, and integrations.

// importScorecards imports scorecards grouped by blueprint.
func (i *Importer) importScorecards(ctx context.Context, scorecards []api.Scorecard, result *Result, pool *WorkerPool) {
	byBlueprint := make(map[string][]api.Scorecard)
	for _, sc := range scorecards {
		bpID, ok1 := sc["blueprintIdentifier"].(string)
		scID, ok2 := sc["identifier"].(string)
		if !ok1 || !ok2 || bpID == "" || scID == "" {
			i.errors.Add(fmt.Errorf("scorecard is missing identifier or blueprintIdentifier field, skipping"), "scorecard", "<unknown>")
			continue
		}
		cleaned := cleanSystemFields(sc, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "blueprint", "blueprintIdentifier"})
		byBlueprint[bpID] = append(byBlueprint[bpID], api.Scorecard(cleaned))
	}

	for bpID, scs := range byBlueprint {
		bpID := bpID
		scs := scs
		pool.Go(func() {
			var toMerge []api.Scorecard
			for _, sc := range scs {
				scID := sc["identifier"].(string)
				_, err := i.client.CreateScorecard(ctx, bpID, sc)

				i.mu.Lock()
				if err == nil {
					result.ScorecardsCreated++
				} else if isConflictError(err) {
					toMerge = append(toMerge, sc)
				} else {
					i.errors.Add(err, "scorecard", scID)
				}
				i.mu.Unlock()
			}

			// Port has no PATCH endpoint for individual scorecards, so we
			// fetch the full set, merge in our updates, and bulk PUT.
			if len(toMerge) > 0 {
				existing, fetchErr := i.client.GetScorecards(ctx, bpID)
				if fetchErr != nil {
					i.mu.Lock()
					i.errors.Add(fetchErr, "scorecard", fmt.Sprintf("fetch:%s", bpID))
					i.mu.Unlock()
					return
				}

				mergeSet := make(map[string]api.Scorecard, len(toMerge))
				for _, sc := range toMerge {
					mergeSet[sc["identifier"].(string)] = sc
				}

				merged := make([]api.Scorecard, 0, len(existing))
				for _, ex := range existing {
					exID, _ := ex["identifier"].(string)
					cleaned := cleanSystemFields(ex, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id", "blueprint", "blueprintIdentifier"})
					if replacement, ok := mergeSet[exID]; ok {
						merged = append(merged, replacement)
						delete(mergeSet, exID)
					} else {
						merged = append(merged, api.Scorecard(cleaned))
					}
				}
				for _, sc := range mergeSet {
					merged = append(merged, sc)
				}

				_, putErr := i.client.UpdateScorecards(ctx, bpID, merged)
				i.mu.Lock()
				if putErr != nil {
					i.errors.Add(putErr, "scorecard", fmt.Sprintf("bulk-put:%s", bpID))
				} else {
					result.ScorecardsUpdated += len(toMerge)
				}
				i.mu.Unlock()
			}
		})
	}
}

// importActions imports actions/automations.
func (i *Importer) importActions(ctx context.Context, actions []api.Action, result *Result, pool *WorkerPool) {
	for _, action := range actions {
		action := action
		pool.Go(func() {
			actionID, ok := action["identifier"].(string)
			if !ok || actionID == "" {
				return
			}

			cleaned := cleanSystemFields(action, []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"})
			apiAction := api.Automation(cleaned)

			_, err := i.client.CreateAutomation(ctx, apiAction)

			i.mu.Lock()
			if err == nil {
				result.ActionsCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdateAutomation(ctx, actionID, apiAction)
				if updateErr != nil {
					i.errors.Add(updateErr, "action", actionID)
				} else {
					result.ActionsUpdated++
				}
			} else {
				i.errors.Add(err, "action", actionID)
			}
			i.mu.Unlock()
		})
	}
}

// sanitizeTeamFields removes nil-valued fields from a team map before sending
// to the API. Some fields (e.g. description) exported as null from the source
// org cause invalid_request errors on upsert even though the API stores null
// internally. Omitting the field avoids the validation error.
func sanitizeTeamFields(team api.Team) api.Team {
	result := make(api.Team, len(team))
	for k, v := range team {
		if v != nil {
			result[k] = v
		}
	}
	return result
}

// importTeams imports teams.
func (i *Importer) importTeams(ctx context.Context, teams []api.Team, result *Result, pool *WorkerPool) {
	for _, team := range teams {
		team := team
		pool.Go(func() {
			teamName, ok := team["name"].(string)
			if !ok || teamName == "" {
				return
			}

			sanitized := sanitizeTeamFields(team)
			_, err := i.client.CreateTeam(ctx, sanitized)

			i.mu.Lock()
			if err == nil {
				result.TeamsCreated++
			} else if isConflictError(err) {
				_, updateErr := i.client.UpdateTeam(ctx, teamName, sanitized)
				if updateErr != nil {
					i.errors.Add(updateErr, "team", teamName)
				} else {
					result.TeamsUpdated++
				}
			} else {
				i.errors.Add(err, "team", teamName)
			}
			i.mu.Unlock()
		})
	}
}

// UserBatchSize is the maximum number of _user entities per bulk API call.
const UserBatchSize = 20

// UserStatusForCreate returns the status to set when creating a new user entity.
func UserStatusForCreate(user api.User, usersAsDisabled bool) string {
	if usersAsDisabled {
		userType, _ := user["type"].(string)
		if userType != "ADMIN" {
			return "DISABLED"
		}
	}
	return "STAGED"
}

// UserToEntity converts a User API response to a _user blueprint entity payload.
// Pass statusOverride="" to keep the source status (used for updates).
func UserToEntity(user api.User, statusOverride string) api.Entity {
	email, _ := user["email"].(string)
	firstName, _ := user["firstName"].(string)
	lastName, _ := user["lastName"].(string)

	systemFields := map[string]bool{
		"id": true, "createdAt": true, "updatedAt": true,
		"createdBy": true, "updatedBy": true,
	}
	props := make(map[string]interface{})
	for k, v := range user {
		if !systemFields[k] {
			props[k] = v
		}
	}
	if statusOverride != "" {
		props["status"] = statusOverride
	}

	title := strings.TrimSpace(firstName + " " + lastName)
	if title == "" {
		title = email
	}
	return api.Entity{
		"identifier": email,
		"title":      title,
		"properties": props,
	}
}

// importUsers imports users as _user blueprint entities.
// New users are created with STAGED status (or DISABLED for non-admins when usersAsDisabled is true).
// Existing users are updated with source data as-is.
func userUpdateEmailsFromDiff(diff *DiffResult) map[string]bool {
	if diff == nil || len(diff.UsersToUpdate) == 0 {
		return nil
	}
	emails := make(map[string]bool, len(diff.UsersToUpdate))
	for _, u := range diff.UsersToUpdate {
		if email, ok := u["email"].(string); ok && email != "" {
			emails[email] = true
		}
	}
	return emails
}

func (i *Importer) importUsers(ctx context.Context, users []api.User, result *Result, usersAsDisabled bool, userUpdateEmails map[string]bool) {
	var toUpdate []api.User
	if len(userUpdateEmails) > 0 {
		var toCreate []api.User
		for _, u := range users {
			email, ok := u["email"].(string)
			if !ok || email == "" {
				continue
			}
			if userUpdateEmails[email] {
				toUpdate = append(toUpdate, u)
			} else {
				toCreate = append(toCreate, u)
			}
		}
		users = toCreate
	}

	// Index by email for conflict resolution
	byEmail := make(map[string]api.User, len(users))
	for _, u := range users {
		if email, ok := u["email"].(string); ok && email != "" {
			byEmail[email] = u
		}
	}

	for start := 0; start < len(users); start += UserBatchSize {
		end := start + UserBatchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[start:end]

		entities := make([]api.Entity, 0, len(batch))
		for _, u := range batch {
			if email, ok := u["email"].(string); !ok || email == "" {
				continue
			}
			status := UserStatusForCreate(u, usersAsDisabled)
			entities = append(entities, UserToEntity(u, status))
		}
		if len(entities) == 0 {
			continue
		}

		errs, err := i.client.CreateUserEntitiesBulk(ctx, entities, false)
		if err != nil {
			i.mu.Lock()
			for _, e := range entities {
				if email, ok := e["identifier"].(string); ok {
					i.errors.Add(err, "user", email)
				}
			}
			i.mu.Unlock()
			continue
		}

		i.mu.Lock()
		result.UsersCreated += len(entities) - len(errs)
		i.mu.Unlock()

		// Collect conflicting users and re-POST with upsert=true, source data as-is
		var conflictEntities []api.Entity
		var nonConflictErrs []api.BulkEntityError
		for _, be := range errs {
			if int(be.StatusCode) == 409 {
				if orig, ok := byEmail[be.Identifier]; ok {
					conflictEntities = append(conflictEntities, UserToEntity(orig, ""))
				}
			} else {
				nonConflictErrs = append(nonConflictErrs, be)
			}
		}

		for _, be := range nonConflictErrs {
			i.mu.Lock()
			i.errors.Add(fmt.Errorf("%s: %s", be.Error, be.Message), "user", be.Identifier)
			i.mu.Unlock()
		}

		if len(conflictEntities) > 0 {
			updateErrs, updateErr := i.client.CreateUserEntitiesBulk(ctx, conflictEntities, true)
			if updateErr != nil {
				i.mu.Lock()
				for _, e := range conflictEntities {
					if email, ok := e["identifier"].(string); ok {
						i.errors.Add(updateErr, "user", email)
					}
				}
				i.mu.Unlock()
			} else {
				i.mu.Lock()
				result.UsersUpdated += len(conflictEntities) - len(updateErrs)
				i.mu.Unlock()
				for _, be := range updateErrs {
					i.mu.Lock()
					i.errors.Add(fmt.Errorf("%s: %s", be.Error, be.Message), "user", be.Identifier)
					i.mu.Unlock()
				}
			}
		}
	}
	i.importUserUpdates(ctx, toUpdate, result)
}

func (i *Importer) importUserUpdates(ctx context.Context, users []api.User, result *Result) {
	for start := 0; start < len(users); start += UserBatchSize {
		end := start + UserBatchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[start:end]

		entities := make([]api.Entity, 0, len(batch))
		for _, u := range batch {
			if email, ok := u["email"].(string); !ok || email == "" {
				continue
			}
			entities = append(entities, UserToEntity(u, ""))
		}
		if len(entities) == 0 {
			continue
		}

		updateErrs, err := i.client.CreateUserEntitiesBulk(ctx, entities, true)
		i.mu.Lock()
		if err != nil {
			for _, e := range entities {
				if email, ok := e["identifier"].(string); ok {
					i.errors.Add(err, "user", email)
				}
			}
		} else {
			result.UsersUpdated += len(entities) - len(updateErrs)
			for _, be := range updateErrs {
				i.errors.Add(fmt.Errorf("%s: %s", be.Error, be.Message), "user", be.Identifier)
			}
		}
		i.mu.Unlock()
	}
}

// actionAuditFields are the audit/internal fields that must be stripped before
// sending an action or automation to the Port API.
var actionAuditFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}

// CleanActionForCreate returns a copy of the action with audit fields removed.
func CleanActionForCreate(action api.Action) api.Action {
	return api.Action(cleanSystemFields(map[string]interface{}(action), actionAuditFields))
}

// importIntegrations imports integrations (update config only).
func (i *Importer) importIntegrations(ctx context.Context, integrations []api.Integration, result *Result, pool *WorkerPool) {
	for _, integration := range integrations {
		integration := integration
		pool.Go(func() {
			integrationID, ok := integration["identifier"].(string)
			if !ok || integrationID == "" {
				i.errors.Add(fmt.Errorf("integration is missing identifier field, skipping"), "integration", "<unknown>")
				return
			}

			// The integration config endpoint expects {"config": {...}} wrapper
			config, ok := integration["config"].(map[string]interface{})
			if !ok || config == nil {
				// No config to update — report so the user knows this integration was skipped
				i.errors.Add(fmt.Errorf("integration has no config field to update, skipping"), "integration", integrationID)
				return
			}

			// Wrap the config in the expected format
			payload := map[string]interface{}{
				"config": config,
			}

			_, err := i.client.UpdateIntegrationConfig(ctx, integrationID, payload)

			i.mu.Lock()
			if err != nil {
				i.errors.Add(err, "integration", integrationID)
			} else {
				result.IntegrationsUpdated++
			}
			i.mu.Unlock()
		})
	}
}

// importPermissions applies blueprint and action permission changes from a DiffResult.
// Permissions are applied after all other resources have been imported so that the
// underlying blueprints, actions, and pages are guaranteed to exist.
// When the API rejects a payload due to orphaned relations or properties (422
// invalid_permissions), the offending keys are stripped and the request is retried.
