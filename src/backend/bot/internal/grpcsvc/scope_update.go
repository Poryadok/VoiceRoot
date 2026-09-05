package grpcsvc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/manifest"
)

func validateUpdateScopes(currentRaw, requestedRaw string) (string, error) {
	requested, err := parseScopeSet(requestedRaw, true)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid scopes_json: %v", err)
	}
	current, err := parseScopeSet(currentRaw, false)
	if err != nil {
		return "", status.Error(codes.Internal, "stored bot scopes are invalid")
	}
	for scope := range requested {
		if _, ok := current[scope]; !ok {
			return "", status.Error(codes.PermissionDenied, "scope escalation requires renewed consent")
		}
	}

	return canonicalScopesJSON(requested), nil
}

func parseScopeSet(raw string, validateAllowed bool) (map[string]struct{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return nil, fmt.Errorf("scopes_json must be a JSON array")
	}

	var scopes []string
	if err := json.Unmarshal([]byte(trimmed), &scopes); err != nil || scopes == nil {
		return nil, fmt.Errorf("scopes_json must be a JSON array of strings")
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
	if validateAllowed {
		errs := manifest.Validate(manifest.Document{Name: "scope-update", Scopes: scopes})
		if len(errs) > 0 {
			return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
		}
	}
	return seen, nil
}

func canonicalScopesJSON(scopes map[string]struct{}) string {
	ordered := make([]string, 0, len(scopes))
	for scope := range scopes {
		ordered = append(ordered, scope)
	}
	sort.Strings(ordered)
	encoded, _ := json.Marshal(ordered)
	return string(encoded)
}
