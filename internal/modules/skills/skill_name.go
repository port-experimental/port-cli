package skills

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var agentSkillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// normalizeToAgentSkillName converts an arbitrary string to a lowercase,
// hyphen-separated identifier containing only letters and digits.
func normalizeToAgentSkillName(s string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		default:
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// agentSkillNameFromIdentifier derives an Agent Skills name from a skill
// identifier, using only the base component (last path segment).
func agentSkillNameFromIdentifier(identifier string) (string, error) {
	name := normalizeToAgentSkillName(skillIdentifierBase(identifier))
	if err := validateAgentSkillName(name); err != nil {
		return "", fmt.Errorf("cannot derive Agent Skills name from %q: %w", identifier, err)
	}
	return name, nil
}

// agentSkillNameFromTitle derives an Agent Skills name from a skill title,
// normalizing the full title string (not just a base component).
func agentSkillNameFromTitle(title string) (string, error) {
	name := normalizeToAgentSkillName(title)
	if err := validateAgentSkillName(name); err != nil {
		return "", fmt.Errorf("cannot derive Agent Skills name from title %q: %w", title, err)
	}
	return name, nil
}

func validateAgentSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("must not be empty")
	}
	if !agentSkillNamePattern.MatchString(name) {
		return fmt.Errorf("must contain only lowercase letters, numbers, and single hyphens")
	}
	return nil
}
