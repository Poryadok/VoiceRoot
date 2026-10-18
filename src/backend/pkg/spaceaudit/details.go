// Package spaceaudit owns the frozen Role/Chat audit wire codecs shared by
// transactional producers and Space ingestion.
package spaceaudit

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrInvalidDetails = errors.New("invalid audit details")

type detailsKind uint8

const (
	detailsEmpty detailsKind = iota
	detailsRoleUpdate
	detailsChatUpdate
	detailsAssignment
	detailsOverride
	detailsReorder
)

// ActionSpec is the closed external registry. Producer is the authenticated
// service issuer; TargetsSpace additionally requires target_id == space_id.
type ActionSpec struct {
	Producer     string
	TargetType   string
	TargetsSpace bool
	kind         detailsKind
}

var actions = map[string]ActionSpec{
	"role_created":                {Producer: "role", TargetType: "role", kind: detailsEmpty},
	"role_updated":                {Producer: "role", TargetType: "role", kind: detailsRoleUpdate},
	"role_deleted":                {Producer: "role", TargetType: "role", kind: detailsEmpty},
	"default_join_role_set":       {Producer: "role", TargetType: "role", kind: detailsEmpty},
	"roles_reordered":             {Producer: "role", TargetType: "space", TargetsSpace: true, kind: detailsReorder},
	"role_assigned":               {Producer: "role", TargetType: "profile", kind: detailsAssignment},
	"role_revoked":                {Producer: "role", TargetType: "profile", kind: detailsAssignment},
	"chat_override_set":           {Producer: "role", TargetType: "chat", kind: detailsOverride},
	"chat_override_removed":       {Producer: "role", TargetType: "chat", kind: detailsOverride},
	"voice_room_override_set":     {Producer: "role", TargetType: "voice_room", kind: detailsOverride},
	"voice_room_override_removed": {Producer: "role", TargetType: "voice_room", kind: detailsOverride},
	"chat_created":                {Producer: "chat", TargetType: "chat", kind: detailsEmpty},
	"chat_updated":                {Producer: "chat", TargetType: "chat", kind: detailsChatUpdate},
	"chat_deleted":                {Producer: "chat", TargetType: "chat", kind: detailsEmpty},
}

func LookupAction(action string) (ActionSpec, bool) {
	spec, ok := actions[action]
	return spec, ok
}

func EmptyDetails(action string) (string, error) {
	if spec, ok := LookupAction(action); !ok || spec.kind != detailsEmpty {
		return "", ErrInvalidDetails
	}
	return "{}", nil
}

// UpdateDetails sorts a copy; the caller must supply actual normalized changes,
// not request presence. An unchanged mutation must not produce an audit fact.
func UpdateDetails(action string, fields []string) (string, error) {
	spec, ok := LookupAction(action)
	if !ok || (spec.kind != detailsRoleUpdate && spec.kind != detailsChatUpdate) || len(fields) == 0 {
		return "", ErrInvalidDetails
	}
	ordered := slices.Clone(fields)
	slices.Sort(ordered)
	for i, field := range ordered {
		if (i > 0 && field == ordered[i-1]) || !allowedUpdateField(spec.kind, field) {
			return "", ErrInvalidDetails
		}
	}
	return `{"changed_fields":["` + strings.Join(ordered, `","`) + `"]}`, nil
}

func allowedUpdateField(kind detailsKind, field string) bool {
	if kind == detailsRoleUpdate {
		return field == "name" || field == "permissions" || field == "position"
	}
	switch field {
	case "allow_guests", "allow_user_main_feed", "avatar_url", "name", "slow_mode_seconds", "threads_enabled", "topic":
		return true
	default:
		return false
	}
}

func canonicalRoleID(roleID string) bool {
	id, err := uuid.Parse(roleID)
	return err == nil && id.String() == roleID
}

func RoleAssignmentDetails(action, roleID string) (string, error) {
	if spec, ok := LookupAction(action); !ok || spec.kind != detailsAssignment || !canonicalRoleID(roleID) {
		return "", ErrInvalidDetails
	}
	return `{"role_id":"` + roleID + `"}`, nil
}

// OverrideDetails records masks without imposing permission policy or bit quotas.
// Removed overrides must use the prior committed row's masks, not fabricated zeroes.
func OverrideDetails(action, roleID string, allow, deny uint64) (string, error) {
	if spec, ok := LookupAction(action); !ok || spec.kind != detailsOverride || !canonicalRoleID(roleID) {
		return "", ErrInvalidDetails
	}
	return `{"allow_mask":` + strconv.FormatUint(allow, 10) + `,"deny_mask":` + strconv.FormatUint(deny, 10) + `,"role_id":"` + roleID + `"}`, nil
}

func ReorderDetails(count uint64) (string, error) {
	if count == 0 {
		return "", ErrInvalidDetails
	}
	return `{"count":` + strconv.FormatUint(count, 10) + `}`, nil
}

// ValidateDetails compares the original bytes to the frozen typed codec. This
// rejects duplicate/escaped keys, alternate number forms and whitespace without
// accepting JSONB's lossy equality or decoding integers through float64.
func ValidateDetails(action, original string) error {
	spec, ok := LookupAction(action)
	if !ok || !utf8.ValidString(original) {
		return ErrInvalidDetails
	}
	if spec.kind == detailsEmpty {
		if original != "{}" {
			return ErrInvalidDetails
		}
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(original), &object); err != nil || object == nil {
		return ErrInvalidDetails
	}
	var canonical string
	var err error
	switch spec.kind {
	case detailsRoleUpdate, detailsChatUpdate:
		var fields []string
		if len(object) != 1 || json.Unmarshal(object["changed_fields"], &fields) != nil {
			return ErrInvalidDetails
		}
		canonical, err = UpdateDetails(action, fields)
	case detailsAssignment:
		var roleID string
		if len(object) != 1 || json.Unmarshal(object["role_id"], &roleID) != nil {
			return ErrInvalidDetails
		}
		canonical, err = RoleAssignmentDetails(action, roleID)
	case detailsOverride:
		var roleID string
		var allow, deny uint64
		if len(object) != 3 || json.Unmarshal(object["role_id"], &roleID) != nil ||
			json.Unmarshal(object["allow_mask"], &allow) != nil || json.Unmarshal(object["deny_mask"], &deny) != nil {
			return ErrInvalidDetails
		}
		canonical, err = OverrideDetails(action, roleID, allow, deny)
	case detailsReorder:
		var count uint64
		if len(object) != 1 || json.Unmarshal(object["count"], &count) != nil {
			return ErrInvalidDetails
		}
		canonical, err = ReorderDetails(count)
	default:
		return ErrInvalidDetails
	}
	if err != nil || canonical != original {
		return ErrInvalidDetails
	}
	return nil
}
