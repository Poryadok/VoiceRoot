package principalgrpc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

type ownershipV2CapabilitiesContextKey struct{}

// WithOwnershipV2CapabilitiesActive records a server-owned activation decision.
// Incoming metadata cannot construct this context value.
func WithOwnershipV2CapabilitiesActive(ctx context.Context) context.Context {
	return context.WithValue(ctx, ownershipV2CapabilitiesContextKey{}, true)
}

func OwnershipV2CapabilitiesActive(ctx context.Context) bool {
	active, _ := ctx.Value(ownershipV2CapabilitiesContextKey{}).(bool)
	return active
}

// StrictUnaryInterceptor authenticates a protected RPC before its handler.
func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		if info.FullMethod == rolev1.RoleService_BootstrapSpaceRoles_FullMethodName {
			bootstrap, ok := message.(*rolev1.BootstrapSpaceRolesRequest)
			if !ok || len(bootstrap.ProtoReflect().GetUnknown()) != 0 || !bootstrapUUID(bootstrap.GetSpaceId()) || !bootstrapUUID(bootstrap.GetOwnerProfileId()) {
				return nil, status.Error(codes.InvalidArgument, "invalid bootstrap binding")
			}
		}
		requestHash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash)
		if err != nil {
			return nil, verificationStatus(err)
		}
		if IsDelegatedRoleMethod(info.FullMethod) {
			if err := RequirePublicRole(principal.WithVerified(ctx, verified), message, info.FullMethod); err != nil {
				return nil, err
			}
		}
		if info.FullMethod == rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName {
			removal, ok := message.(*rolev1.ApplyBotInstallationRemovalRequest)
			if !ok {
				return nil, status.Error(codes.InvalidArgument, "invalid Bot removal request")
			}
			if err := RequireBotRemoval(principal.WithVerified(ctx, verified), removal); err != nil {
				return nil, err
			}
		}
		if info.FullMethod == rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName || info.FullMethod == rolev1.RoleService_RemoveMemberRoles_FullMethodName {
			if err := RequireMemberAdmission(principal.WithVerified(ctx, verified), message, info.FullMethod); err != nil {
				return nil, err
			}
		}
		if info.FullMethod == rolev1.RoleService_CheckPermission_FullMethodName {
			var decisionErr error
			if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceManageBots {
				decisionErr = RequireBotManagementPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceApproveJoinRequests {
				decisionErr = RequireJoinApprovalPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceManageSettings {
				decisionErr = RequireSettingsPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.MemberManageNicknames {
				decisionErr = RequireNicknamePermission(principal.WithVerified(ctx, verified), req)
			} else {
				decisionErr = validateCreationDecision(verified, message, transport.RequestID, requestHash)
			}
			if decisionErr != nil {
				return nil, decisionErr
			}
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}
}

type unavailableError struct{ err error }

func (e unavailableError) Error() string { return e.err.Error() }
func (e unavailableError) Unwrap() error { return e.err }
func Unavailable(err error) error {
	if err == nil {
		err = errors.New("unavailable")
	}
	return unavailableError{err}
}
func IsUnavailable(err error) bool {
	var dependency unavailableError
	return errors.As(err, &dependency)
}
func verificationStatus(err error) error {
	var unavailable unavailableError
	if errors.As(err, &unavailable) {
		return status.Error(codes.Unavailable, "principal verifier unavailable")
	}
	return status.Error(codes.Unauthenticated, "invalid principal")
}

// OwnershipUnaryInterceptor drains every protected protocol from the ordinary
// listener without consuming protected verifier dependencies.
func OwnershipUnaryInterceptor(_ Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsDelegatedRoleMethod(info.FullMethod) {
			return nil, status.Error(codes.Unauthenticated, "protected verified Role actor listener required")
		}

		if info.FullMethod == rolev1.RoleService_CheckPermission_FullMethodName {
			if req, ok := request.(*rolev1.CheckPermissionRequest); ok && (req.GetPermissionName() == permissions.SpaceApproveJoinRequests || req.GetPermissionName() == permissions.SpaceManageSettings || req.GetPermissionName() == permissions.SpaceManageBots) {
				return nil, status.Error(codes.PermissionDenied, "protected approval decision listener required")
			}
			if req, ok := request.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.MemberManageNicknames {
				return nil, status.Error(codes.PermissionDenied, "protected nickname decision listener required")
			}
		}
		switch info.FullMethod {
		case rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName:
			return nil, status.Error(codes.Unavailable, "protected Bot cleanup listener required")
		case rolev1.RoleService_DeleteRolesCreatedByProfile_FullMethodName:
			return nil, status.Error(codes.Unavailable, "protected Bot cleanup adapter required")
		case rolev1.RoleService_BootstrapSpaceRoles_FullMethodName:
			return nil, status.Error(codes.Unavailable, "protected bootstrap listener required")
		case rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName,
			rolev1.RoleService_RemoveMemberRoles_FullMethodName:
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		case "/voice.role.v1.RoleService/ApplySpaceLifecycleFence":
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		case "/voice.role.v1.RoleService/GetOwnershipTransferCapabilities",
			"/voice.role.v1.RoleService/PrepareOwnershipTransfer",
			"/voice.role.v1.RoleService/FinalizeOwnershipTransfer",
			"/voice.role.v1.RoleService/AbortOwnershipTransfer",
			"/voice.role.v1.RoleService/ApplyOwnershipTransfer",
			"/voice.role.v1.RoleService/CompensateOwnershipTransfer",
			"/voice.role.v1.RoleService/ResolveVoiceRoomGrants",
			"/voice.role.v1.RoleService/ApplyGameSessionGrants",
			"/voice.role.v1.RoleService/RevokeGameSessionGrants",
			"/voice.role.v1.RoleService/CheckGameSessionGrant",
			"/voice.role.v1.RoleService/RetireSpace":
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		default:
			return handler(ctx, request)
		}
	}
}

func bootstrapUUID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}
