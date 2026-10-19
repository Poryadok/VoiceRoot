package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func IsDelegatedRoleMethod(method string) bool {
	switch method {
	case rolev1.RoleService_CreateRole_FullMethodName, rolev1.RoleService_UpdateRole_FullMethodName, rolev1.RoleService_DeleteRole_FullMethodName,
		rolev1.RoleService_ListRoles_FullMethodName, rolev1.RoleService_ReorderRoles_FullMethodName,
		rolev1.RoleService_AssignRole_FullMethodName, rolev1.RoleService_RevokeRole_FullMethodName, rolev1.RoleService_GetMemberRoles_FullMethodName,
		rolev1.RoleService_SetChatOverride_FullMethodName, rolev1.RoleService_RemoveChatOverride_FullMethodName, rolev1.RoleService_GetChatOverrides_FullMethodName,
		rolev1.RoleService_SetVoiceRoomOverride_FullMethodName, rolev1.RoleService_RemoveVoiceRoomOverride_FullMethodName, rolev1.RoleService_GetVoiceRoomOverrides_FullMethodName,
		rolev1.RoleService_SetDefaultJoinRole_FullMethodName, rolev1.RoleService_GetDefaultJoinRole_FullMethodName, rolev1.RoleService_GetEffectivePermissions_FullMethodName:
		return true
	}
	return false
}

func IsSpaceRoleRead(method string) bool {
	return method == rolev1.RoleService_ListRoles_FullMethodName || method == rolev1.RoleService_GetMemberRoles_FullMethodName || method == rolev1.RoleService_GetDefaultJoinRole_FullMethodName
}

// The handler still owns target ACLs and membership proof. Request profile_id
// never replaces the principal's actor, and metadata cannot supply authority.
func RequirePublicRole(ctx context.Context, request proto.Message, method string) error {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Role principal required")
	}
	if !IsDelegatedRoleMethod(method) || request == nil || !request.ProtoReflect().IsValid() || unknownRoleRequest(request.ProtoReflect(), 0) {
		return status.Error(codes.InvalidArgument, "invalid Role request")
	}
	hash, err := principal.RequestHash(request)
	if err != nil || p.Audience != "role" || p.RPC != method || p.RequestHash != hash || !publicRoleUUID(p.RequestID) {
		return status.Error(codes.PermissionDenied, "exact Role request binding required")
	}
	if IsSpaceRoleRead(method) && p.Kind == "service" && p.Issuer == "space" && p.Subject == "service:space" && p.AccountID == "" && p.ProfileID == "" && p.SessionEpoch == 0 && p.ActorProfileID == "" && p.BotID == "" && p.SpaceID == "" && p.InstallationID == "" && p.AccountType == "" && p.BotScope == "" {
		return nil
	}
	if p.Kind != "delegated_user" || p.Issuer != "gateway" || p.Subject != p.AccountID || !publicRoleUUID(p.AccountID) || !publicRoleUUID(p.ProfileID) || p.SessionEpoch <= 0 || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return status.Error(codes.PermissionDenied, "Gateway delegated Role actor required")
	}
	return nil
}

// A valid transport signature binds unknown bytes too. It does not grant the
// receiver authority to interpret an unknown nested request schema.
func unknownRoleRequest(message protoreflect.Message, depth int) bool {
	if !message.IsValid() || depth > 64 || len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.IsMap() && field.MapValue().Message() != nil:
			value.Map().Range(func(_ protoreflect.MapKey, entry protoreflect.Value) bool {
				unknown = unknownRoleRequest(entry.Message(), depth+1)
				return !unknown
			})
		case field.IsList() && field.Message() != nil:
			list := value.List()
			for i := 0; i < list.Len() && !unknown; i++ {
				unknown = unknownRoleRequest(list.Get(i).Message(), depth+1)
			}
		case !field.IsMap() && !field.IsList() && field.Message() != nil:
			unknown = unknownRoleRequest(value.Message(), depth+1)
		}
		return !unknown
	})
	return unknown
}
func publicRoleUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
