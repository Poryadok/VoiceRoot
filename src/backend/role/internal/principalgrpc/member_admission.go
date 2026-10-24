package principalgrpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
)

// RequireMemberAdmission validates the already verified exact request again at
// the handler boundary. A Space service credential never carries human or bot
// assignment authority and cannot select a role.
func RequireMemberAdmission(ctx context.Context, req proto.Message, method string) error {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Space member admission required")
	}
	if p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "role" || p.RPC != method || p.RequestID == "" || p.RequestHash == "" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return status.Error(codes.PermissionDenied, "exact Space member admission binding required")
	}
	if req == nil || !req.ProtoReflect().IsValid() || len(req.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "invalid member admission request")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != p.RequestHash {
		return status.Error(codes.PermissionDenied, "member admission request hash mismatch")
	}
	switch method {
	case rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName:
		v, ok := req.(*rolev1.EnsureDefaultMemberRoleRequest)
		if !ok || v == nil {
			return status.Error(codes.InvalidArgument, "invalid Ensure request")
		}
		err = memberadmission.ValidateEnsure(memberadmission.EnsureBinding{ProtocolVersion: v.ProtocolVersion, SpaceID: v.SpaceId, ProfileID: v.ProfileId, MembershipGeneration: v.MembershipGeneration, AdmissionOperationID: v.AdmissionOperationId, AdmissionBindingSHA256: v.AdmissionBindingSha256})
	case rolev1.RoleService_RemoveMemberRoles_FullMethodName:
		v, ok := req.(*rolev1.RemoveMemberRolesRequest)
		if !ok || v == nil {
			return status.Error(codes.InvalidArgument, "invalid Remove request")
		}
		err = memberadmission.ValidateRemove(memberadmission.RemoveBinding{ProtocolVersion: v.ProtocolVersion, SpaceID: v.SpaceId, ProfileID: v.ProfileId, RemovalGenerationFloor: v.RemovalGenerationFloor, RemovalOperationID: v.RemovalOperationId, RemovalBindingSHA256: v.RemovalBindingSha256})
	default:
		return status.Error(codes.PermissionDenied, "member admission method denied")
	}
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid member admission binding")
	}
	return nil
}
