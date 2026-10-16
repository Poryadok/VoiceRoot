package registry

import (
	"fmt"
	"strings"
)

// PermanentOwnerRejection is reserved for an allow-listed, request-bound
// rejection returned by a durable session owner. Transport, principal,
// malformed-request, setup, and persistence errors must not use this type.
type PermanentOwnerRejection struct {
	Category string
}

func (r *PermanentOwnerRejection) Error() string {
	if r == nil {
		return "permanent session owner rejection"
	}
	return fmt.Sprintf("permanent session owner rejection: %s", r.Category)
}

func permanentOwnerFailureCode(stage, category string) string {
	allowed := false
	switch stage {
	case "chat_create":
		allowed = category == "operation_conflict" || category == "resource_conflict"
	case "chat_roster":
		allowed = category == "operation_conflict" || category == "resource_conflict" || category == "resource_missing"
	case "voice_provision", "voice_roster":
		allowed = category == "operation_conflict"
	case "role_apply":
		allowed = category == "operation_conflict" || category == "terminal_revoked"
	}
	if !allowed {
		return ""
	}
	return fmt.Sprintf("OWNER_%s_%s", strings.ToUpper(stage), strings.ToUpper(category))
}
