// Package resources provides shared resource identity and normalization for compare/import.
package resources

// ResourceKind identifies a Port resource type.
type ResourceKind string

const (
	KindBlueprints           ResourceKind = "blueprints"
	KindEntities             ResourceKind = "entities"
	KindScorecards           ResourceKind = "scorecards"
	KindActions              ResourceKind = "actions"
	KindPages                ResourceKind = "pages"
	KindIntegrations         ResourceKind = "integrations"
	KindTeams                ResourceKind = "teams"
	KindUsers                ResourceKind = "users"
	KindBlueprintPermissions ResourceKind = "blueprint-permissions"
	KindActionPermissions    ResourceKind = "action-permissions"
	KindPagePermissions      ResourceKind = "page-permissions"
)

// IdentityFunc extracts a stable identity key from a resource map.
type IdentityFunc func(map[string]interface{}) (string, bool)

// Descriptor describes how a resource kind is identified and normalized.
type Descriptor struct {
	Kind                ResourceKind
	Identity            IdentityFunc
	ServerManagedFields []string
}

// DefaultServerManagedFields are stripped during import diff equality checks.
var DefaultServerManagedFields = []string{"createdBy", "updatedBy", "createdAt", "updatedAt", "id"}

// BlueprintIdentity returns the blueprint identifier.
func BlueprintIdentity(m map[string]interface{}) (string, bool) {
	id, ok := m["identifier"].(string)
	return id, ok && id != ""
}

// EntityIdentity returns blueprint:entity composite key.
func EntityIdentity(m map[string]interface{}) (string, bool) {
	blueprintID, ok1 := m["blueprint"].(string)
	entityID, ok2 := m["identifier"].(string)
	if !ok1 || !ok2 || blueprintID == "" || entityID == "" {
		return "", false
	}
	return blueprintID + ":" + entityID, true
}

// ScorecardIdentity returns blueprintIdentifier:identifier composite key.
func ScorecardIdentity(m map[string]interface{}) (string, bool) {
	blueprintID, ok1 := m["blueprintIdentifier"].(string)
	scorecardID, ok2 := m["identifier"].(string)
	if !ok1 || !ok2 || blueprintID == "" || scorecardID == "" {
		return "", false
	}
	return blueprintID + ":" + scorecardID, true
}

// ActionIdentity returns the action identifier.
func ActionIdentity(m map[string]interface{}) (string, bool) {
	id, ok := m["identifier"].(string)
	return id, ok && id != ""
}

// PageIdentity returns the page identifier.
func PageIdentity(m map[string]interface{}) (string, bool) {
	id, ok := m["identifier"].(string)
	return id, ok && id != ""
}

// IntegrationIdentity returns installationId, falling back to identifier.
func IntegrationIdentity(m map[string]interface{}) (string, bool) {
	if id, ok := m["installationId"].(string); ok && id != "" {
		return id, true
	}
	if id, ok := m["identifier"].(string); ok && id != "" {
		return id, true
	}
	return "", false
}

// TeamIdentity returns the team name.
func TeamIdentity(m map[string]interface{}) (string, bool) {
	name, ok := m["name"].(string)
	return name, ok && name != ""
}

// UserIdentity returns the user email.
func UserIdentity(m map[string]interface{}) (string, bool) {
	email, ok := m["email"].(string)
	return email, ok && email != ""
}

// MapKeyIdentity returns the identifier field set when map-keyed resources are wrapped.
func MapKeyIdentity(m map[string]interface{}) (string, bool) {
	id, ok := m["identifier"].(string)
	return id, ok && id != ""
}

var registry = map[ResourceKind]Descriptor{
	KindBlueprints: {
		Kind:                KindBlueprints,
		Identity:            BlueprintIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindEntities: {
		Kind:                KindEntities,
		Identity:            EntityIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindScorecards: {
		Kind:                KindScorecards,
		Identity:            ScorecardIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindActions: {
		Kind:                KindActions,
		Identity:            ActionIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindPages: {
		Kind:                KindPages,
		Identity:            PageIdentity,
		ServerManagedFields: append(DefaultServerManagedFields, "protected"),
	},
	KindIntegrations: {
		Kind:                KindIntegrations,
		Identity:            IntegrationIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindTeams: {
		Kind:                KindTeams,
		Identity:            TeamIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindUsers: {
		Kind:                KindUsers,
		Identity:            UserIdentity,
		ServerManagedFields: DefaultServerManagedFields,
	},
	KindBlueprintPermissions: {
		Kind:                KindBlueprintPermissions,
		Identity:            MapKeyIdentity,
		ServerManagedFields: nil,
	},
	KindActionPermissions: {
		Kind:                KindActionPermissions,
		Identity:            MapKeyIdentity,
		ServerManagedFields: nil,
	},
	KindPagePermissions: {
		Kind:                KindPagePermissions,
		Identity:            MapKeyIdentity,
		ServerManagedFields: nil,
	},
}

// Get returns the descriptor for a resource kind.
func Get(kind ResourceKind) (Descriptor, bool) {
	desc, ok := registry[kind]
	return desc, ok
}

// MustGet returns the descriptor for a resource kind or panics if unknown.
func MustGet(kind ResourceKind) Descriptor {
	desc, ok := registry[kind]
	if !ok {
		panic("unknown resource kind: " + string(kind))
	}
	return desc
}
