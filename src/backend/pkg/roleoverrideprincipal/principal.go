// Package roleoverrideprincipal owns the narrow Role-to-resource existence
// boundary. It grants no user ACL or lifecycle mutation authority.
package roleoverrideprincipal

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

func Method(target string) string {
	switch target {
	case "chat":
		return "/voice.chat.v1.ChatService/ValidateRoleOverrideTarget"
	case "space":
		return "/voice.space.v1.SpaceService/ValidateRoleOverrideTarget"
	default:
		return ""
	}
}

func CanonicalID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid target identifier")
	}
	return id, nil
}

// Require repeats the verified workload and complete body binding at the
// domain boundary, including direct calls that do not traverse an interceptor.
func Require(ctx context.Context, target string, req proto.Message) error {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Role principal required")
	}
	if p.Kind != "service" || p.Issuer != "role" || p.Subject != "service:role" {
		return status.Error(codes.PermissionDenied, "Role principal required")
	}
	if req == nil || !req.ProtoReflect().IsValid() || len(req.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "invalid target request")
	}
	hash, err := principal.RequestHash(req)
	_, idErr := CanonicalID(p.RequestID)
	if err != nil || idErr != nil || Method(target) == "" || p.Audience != target || p.RPC != Method(target) || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "invalid Role principal binding")
	}
	return nil
}
