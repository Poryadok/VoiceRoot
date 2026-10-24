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

const MemberAdmissionAuthorityMethod = "/voice.space.v1.SpaceService/GetMemberAdmissionAuthority"

func allowsMethod(target, method string) bool {
	return Method(target) != "" && (method == Method(target) || (target == "space" && method == MemberAdmissionAuthorityMethod))
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
	return requireMethod(ctx, target, Method(target), req)
}

// RequireMemberAdmissionAuthority grants a Role observation or the exact
// installation-bound Messaging send observation selected by the request.
// It is not a user membership capability or permission decision.
func RequireMemberAdmissionAuthority(ctx context.Context, req proto.Message) error {
	return requireMethod(ctx, "space", MemberAdmissionAuthorityMethod, req)
}

func requireMethod(ctx context.Context, target, method string, req proto.Message) error {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Role principal required")
	}
	issuer := "role"
	if target == "space" && method == MemberAdmissionAuthorityMethod && hasBotMessageBinding(req) {
		issuer = "messaging"
	}
	if p.Kind != "service" || p.Issuer != issuer || p.Subject != "service:"+issuer {
		return status.Error(codes.PermissionDenied, "Role principal required")
	}
	if req == nil || !req.ProtoReflect().IsValid() || len(req.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "invalid target request")
	}
	hash, err := principal.RequestHash(req)
	_, idErr := CanonicalID(p.RequestID)
	if err != nil || idErr != nil || !allowsMethod(target, method) || p.Audience != target || p.RPC != method || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return status.Error(codes.Unauthenticated, "invalid Role principal binding")
	}
	return nil
}

// Reflection keeps the transport package independent of the Space wire module.
// The Space domain validates the complete selector; no other request shape can
// select Messaging as an issuer on this listener.
func hasBotMessageBinding(req proto.Message) bool {
	if req == nil || !req.ProtoReflect().IsValid() {
		return false
	}
	m := req.ProtoReflect()
	if m.Descriptor().FullName() != "voice.space.v1.GetMemberAdmissionAuthorityRequest" {
		return false
	}
	f := m.Descriptor().Fields().ByName("bot_message_send_binding")
	return f != nil && m.Has(f)
}
