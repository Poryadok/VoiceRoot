package manifest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Subcommand is a grouped slash subcommand (e.g. queue join).
type Subcommand struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// Command describes a slash command from manifest.
type Command struct {
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description" yaml:"description"`
	Options     []Option     `json:"options" yaml:"options"`
	Subcommands []Subcommand `json:"subcommands" yaml:"subcommands"`
}

// FlatCommand is a stored slash command after subcommand expansion.
type FlatCommand struct {
	Name        string
	GroupName   string
	Description string
	Options     []Option
}

// Option is a slash command parameter.
type Option struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	Required     bool   `json:"required" yaml:"required"`
	Autocomplete bool   `json:"autocomplete" yaml:"autocomplete"`
}

// Document is the parsed bot manifest.
type Document struct {
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description" yaml:"description"`
	IconURL     string    `json:"icon_url" yaml:"icon_url"`
	WebhookURL  string    `json:"webhook_url" yaml:"webhook_url"`
	Scopes      []string  `json:"scopes" yaml:"scopes"`
	Commands    []Command `json:"commands" yaml:"commands"`
}

var allowedScopes = map[string]struct{}{
	"TEXT_CHAT_SEND_MESSAGES":   {},
	"DM_SEND":                   {},
	"SPACE_VIEW_MEMBER_LIST":    {},
	"MEMBER_ASSIGN_ROLES":       {},
	"TEXT_CHAT_CREATE_IN_SPACE": {},
	"TEXT_CHAT_READ_HISTORY":    {},
	"SPACE_MANAGE_ROLES":        {},
}

// ParseYAML validates and normalizes manifest YAML.
func ParseYAML(raw string) (Document, []string, error) {
	var doc Document
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return Document{}, nil, err
	}
	errs := Validate(doc)
	if len(errs) > 0 {
		return Document{}, errs, fmt.Errorf("manifest invalid")
	}
	doc.Name = strings.TrimSpace(doc.Name)
	doc.Description = strings.TrimSpace(doc.Description)
	doc.WebhookURL = strings.TrimSpace(doc.WebhookURL)
	doc.IconURL = strings.TrimSpace(doc.IconURL)
	return doc, nil, nil
}

// Validate returns human-readable validation errors.
func Validate(doc Document) []string {
	var errs []string
	if strings.TrimSpace(doc.Name) == "" {
		errs = append(errs, "name is required")
	}
	errs = append(errs, ValidateScopes(doc.Scopes)...)
	errs = append(errs, validateCommands(doc.Commands)...)
	return errs
}

// ValidateScopes returns validation errors for a canonical set of bot scopes.
func ValidateScopes(scopes []string) []string {
	var errs []string
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if strings.TrimSpace(scope) != scope || scope == "" {
			errs = append(errs, "scope identifiers must be non-empty and canonical")
			continue
		}
		if _, ok := allowedScopes[scope]; !ok {
			errs = append(errs, "unknown scope: "+scope)
			continue
		}
		if _, duplicate := seen[scope]; duplicate {
			errs = append(errs, "duplicate scope: "+scope)
			continue
		}
		seen[scope] = struct{}{}
	}
	return errs
}

// ParseScopeSetJSON decodes a scope JSON array into a canonical set. Stored
// scopes are parsed without the allowed-scope check so callers can distinguish
// malformed persistence from a request for a scope the bot does not have.
func ParseScopeSetJSON(raw string, validateAllowed bool) (map[string]struct{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return nil, fmt.Errorf("scopes_json must be a JSON array")
	}

	var scopes []string
	if err := json.Unmarshal([]byte(trimmed), &scopes); err != nil || scopes == nil {
		return nil, fmt.Errorf("scopes_json must be a JSON array of strings")
	}
	if validateAllowed {
		if errs := ValidateScopes(scopes); len(errs) > 0 {
			return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
		}
	}

	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if strings.TrimSpace(scope) != scope || scope == "" {
			return nil, fmt.Errorf("scope identifiers must be non-empty and canonical")
		}
		if _, duplicate := seen[scope]; duplicate {
			return nil, fmt.Errorf("duplicate scope: %s", scope)
		}
		seen[scope] = struct{}{}
	}
	return seen, nil
}

// ValidateCommands validates slash command definitions without manifest metadata.
func ValidateCommands(commands []Command) []string {
	return validateCommands(commands)
}

func validateCommands(commands []Command) []string {
	var errs []string
	seen := map[string]struct{}{}
	for _, cmd := range commands {
		name := strings.TrimSpace(cmd.Name)
		if name == "" {
			errs = append(errs, "command name is required")
			continue
		}
		if len(cmd.Subcommands) > 0 && len(cmd.Options) > 0 {
			errs = append(errs, "command "+name+": cannot have both options and subcommands")
		}
		if len(cmd.Subcommands) == 0 {
			if _, dup := seen[name]; dup {
				errs = append(errs, "duplicate command: "+name)
			}
			seen[name] = struct{}{}
			continue
		}
		for _, sub := range cmd.Subcommands {
			subName := strings.TrimSpace(sub.Name)
			if subName == "" {
				errs = append(errs, "subcommand name is required for "+name)
				continue
			}
			full := name + " " + subName
			if _, dup := seen[full]; dup {
				errs = append(errs, "duplicate command: "+full)
			}
			seen[full] = struct{}{}
		}
	}
	return errs
}

// FlattenCommands expands subcommands into storable slash commands.
func FlattenCommands(commands []Command) []FlatCommand {
	var out []FlatCommand
	for _, cmd := range commands {
		name := strings.TrimSpace(cmd.Name)
		if len(cmd.Subcommands) == 0 {
			out = append(out, FlatCommand{
				Name:        name,
				Description: strings.TrimSpace(cmd.Description),
				Options:     cmd.Options,
			})
			continue
		}
		for _, sub := range cmd.Subcommands {
			subName := strings.TrimSpace(sub.Name)
			if subName == "" {
				continue
			}
			desc := strings.TrimSpace(sub.Description)
			if desc == "" {
				desc = strings.TrimSpace(cmd.Description)
			}
			out = append(out, FlatCommand{
				Name:        subName,
				GroupName:   name,
				Description: desc,
			})
		}
	}
	return out
}

// ToYAML serializes a manifest document as YAML for export.
func ToYAML(doc Document) (string, error) {
	b, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CommandsFromStoredRows rebuilds manifest commands from flattened DB rows.
func CommandsFromStoredRows(rows []StoredCommandRow) []Command {
	groups := map[string][]Subcommand{}
	var singles []Command
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			continue
		}
		parts := strings.SplitN(name, " ", 2)
		if len(parts) == 2 {
			group, sub := parts[0], parts[1]
			groups[group] = append(groups[group], Subcommand{
				Name:        sub,
				Description: strings.TrimSpace(row.Description),
			})
			continue
		}
		var opts []Option
		if strings.TrimSpace(row.Parameters) != "" && row.Parameters != "null" {
			_ = json.Unmarshal([]byte(row.Parameters), &opts)
		}
		singles = append(singles, Command{
			Name:        name,
			Description: strings.TrimSpace(row.Description),
			Options:     opts,
		})
	}
	for group, subs := range groups {
		singles = append(singles, Command{Name: group, Subcommands: subs})
	}
	return singles
}

// StoredCommandRow is a slash command row from persistence (name may be grouped).
type StoredCommandRow struct {
	Name        string
	Description string
	Parameters  string
}

// ToJSON returns normalized manifest JSON for storage.
func ToJSON(doc Document) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CanonicalScopesJSON validates scopes and encodes them in deterministic order.
func CanonicalScopesJSON(scopes []string) (string, error) {
	if errs := ValidateScopes(scopes); len(errs) > 0 {
		return "", fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	ordered := append([]string(nil), scopes...)
	sort.Strings(ordered)
	b, _ := json.Marshal(ordered)
	return string(b), nil
}

// CanonicalScopeSetJSON encodes a scope set in deterministic order.
func CanonicalScopeSetJSON(scopes map[string]struct{}) string {
	ordered := make([]string, 0, len(scopes))
	for scope := range scopes {
		ordered = append(ordered, scope)
	}
	sort.Strings(ordered)
	b, _ := json.Marshal(ordered)
	return string(b)
}

// ScopesJSON encodes a validated scope slice. Callers accepting external input
// should use CanonicalScopesJSON first.
func ScopesJSON(scopes []string) string {
	b, _ := json.Marshal(scopes)
	return string(b)
}

// CommandsJSON encodes commands slice.
func CommandsJSON(commands []Command) string {
	b, _ := json.Marshal(commands)
	return string(b)
}
