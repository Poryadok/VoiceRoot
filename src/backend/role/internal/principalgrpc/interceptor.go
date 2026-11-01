package principalgrpc

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
		requestHash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		var verified principal.Principal
		chatACL := false
		workloadIssuer := ""
		if req, ok := message.(*rolev1.CheckPermissionRequest); ok && info.FullMethod == rolev1.RoleService_CheckPermission_FullMethodName {
			workloadIssuer = WorkloadDecisionIssuer(req.GetPermissionName())
		}
		if req, ok := message.(*rolev1.CheckPermissionRequest); ok && info.FullMethod == rolev1.RoleService_CheckPermission_FullMethodName && req.GetPermissionName() == "TEXT_CHAT_VIEW" {
			chatACL = true
		}
		if workloadIssuer != "" {
			workloadVerifier, ok := verifier.(interface {
				VerifyWorkloadDecision(context.Context, string, string, string, string, string) (principal.Principal, error)
			})
			if !ok {
				return nil, status.Error(codes.Unavailable, "workload decision verifier unavailable")
			}
			verified, err = workloadVerifier.VerifyWorkloadDecision(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash, workloadIssuer)
		} else if chatACL {
			chatVerifier, ok := verifier.(interface {
				VerifyChatCurrentACL(context.Context, string, string, string, string) (principal.Principal, error)
			})
			if !ok {
				return nil, status.Error(codes.Unavailable, "Chat ACL verifier unavailable")
			}
			verified, err = chatVerifier.VerifyChatCurrentACL(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash)
		} else {
			verified, err = verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, requestHash)
		}
		if err != nil {
			return nil, verificationStatus(err)
		}
		if info.FullMethod == rolev1.RoleService_BootstrapSpaceRoles_FullMethodName {
			req, ok := message.(*rolev1.BootstrapSpaceRolesRequest)
			if !ok {
				return nil, status.Error(codes.InvalidArgument, "invalid bootstrap input")
			}
			if err := RequireOwnerBootstrap(principal.WithVerified(ctx, verified), req); err != nil {
				return nil, err
			}
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
			if req, ok := message.(*rolev1.CheckPermissionRequest); ok && workloadIssuer != "" {
				decisionErr = RequireWorkloadDecision(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && chatACL {
				decisionErr = RequireChatCurrentACL(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceManageBots {
				decisionErr = RequireBotManagementPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceApproveJoinRequests {
				decisionErr = RequireJoinApprovalPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceManageSettings {
				decisionErr = RequireSettingsPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceManageMatchmaking {
				decisionErr = RequireMatchmakingPermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.SpaceManageInvites {
				decisionErr = RequireInvitePermission(principal.WithVerified(ctx, verified), req)
			} else if req, ok := message.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == permissions.MemberBan {
				decisionErr = RequireBanPermission(principal.WithVerified(ctx, verified), req)
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
			if req, ok := request.(*rolev1.CheckPermissionRequest); ok && WorkloadDecisionIssuer(req.GetPermissionName()) != "" {
				return nil, status.Error(codes.Unavailable, "protected workload decision listener required")
			}
			if req, ok := request.(*rolev1.CheckPermissionRequest); ok && (req.GetPermissionName() == permissions.SpaceManageInvites || req.GetPermissionName() == permissions.SpaceManageMatchmaking || req.GetPermissionName() == permissions.MemberBan) {
				return nil, status.Error(codes.PermissionDenied, "protected Space decision listener required")
			}
			if req, ok := request.(*rolev1.CheckPermissionRequest); ok && (req.GetPermissionName() == permissions.SpaceApproveJoinRequests || req.GetPermissionName() == permissions.SpaceManageSettings || req.GetPermissionName() == permissions.SpaceManageBots) {
				return nil, status.Error(codes.PermissionDenied, "protected approval decision listener required")
			}
			if req, ok := request.(*rolev1.CheckPermissionRequest); ok && req.GetPermissionName() == "TEXT_CHAT_VIEW" {
				if md, _ := metadata.FromIncomingContext(ctx); len(md.Get("authorization")) != 0 {
					return nil, status.Error(codes.Unavailable, "protected Chat ACL listener required")
				}
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
