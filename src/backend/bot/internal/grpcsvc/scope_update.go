package grpcsvc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/manifest"
)

func validateUpdateScopes(currentRaw, requestedRaw string) (string, error) {
	requested, err := manifest.ParseScopeSetJSON(requestedRaw, true)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid scopes_json: %v", err)
	}
	current, err := manifest.ParseScopeSetJSON(currentRaw, false)
	if err != nil {
		return "", status.Error(codes.Internal, "stored bot scopes are invalid")
	}
	for scope := range requested {
		if _, ok := current[scope]; !ok {
			return "", status.Error(codes.PermissionDenied, "scope escalation requires renewed consent")
		}
	}

	return manifest.CanonicalScopeSetJSON(requested), nil
}
