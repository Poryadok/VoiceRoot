// Package lifecycleprincipal authenticates Gateway's public lifecycle transport.
package lifecycleprincipal

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
)

type Verifier struct {
	Keys          principal.KeyResolver
	Replay        principal.ReplayGuard
	Epoch         principal.SessionEpochChecker
	Clock         func() time.Time
	CreatorEpoch  principal.SessionEpochChecker
	CreatorReplay principal.ReplayGuard
}

func protectedMutation(method string) bool {
	if method == spacev1.SpaceService_CreateInvite_FullMethodName {
		return true
	}
	if moderationMethod(method) || nonInteractiveJoinMethod(method) {
		return true
	}
	if method == spacev1.SpaceService_CreateSpace_FullMethodName {
		return true
	}
	return method == spacev1.SpaceService_DeleteSpace_FullMethodName || method == spacev1.SpaceService_RestoreSpace_FullMethodName || method == spacev1.SpaceService_GetSpaceDeletionCoordinatorStatus_FullMethodName || method == spacev1.SpaceService_UpdateMemberNickname_FullMethodName || method == spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName
}

func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if protectedMutation(info.FullMethod) || info.FullMethod == spacev1.SpaceService_GetAuditLog_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "protected lifecycle listener required")
		}
		return handler(ctx, req)
	}
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == spacev1.SpaceService_CreateInvite_FullMethodName {
			return v.verifyModerationCall(ctx, req, info, handler)
		}
		if moderationMethod(info.FullMethod) || nonInteractiveJoinMethod(info.FullMethod) {
			return v.verifyModerationCall(ctx, req, info, handler)
		}
		if info.FullMethod == spacev1.SpaceService_CreateSpace_FullMethodName {
			return v.verifyCreatorCall(ctx, req, info, handler)
		}
		if !protectedMutation(info.FullMethod) && info.FullMethod != spacev1.SpaceService_GetSpace_FullMethodName && info.FullMethod != spacev1.SpaceService_GetAuditLog_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method is not available on lifecycle listener")
		}
		deny := status.Error(codes.Unauthenticated, "invalid lifecycle principal")
		if v.Keys == nil || v.Replay == nil || v.Epoch == nil {
			return nil, deny
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || !canonicalID(transport.RequestID) {
			return nil, deny
		}
		message, ok := req.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid lifecycle request")
		}
		if operation, ok := req.(interface{ GetOperationId() string }); ok && operation.GetOperationId() != transport.RequestID {
			return nil, deny
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, deny
		}
		verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, SessionEpochChecker: v.Epoch, Clock: v.Clock})
		if errors.Is(err, ErrDependencyUnavailable) {
			return nil, status.Error(codes.Unavailable, "lifecycle authentication unavailable")
		}
		if err != nil || !canonicalID(verified.AccountID) || !canonicalID(verified.ProfileID) {
			return nil, deny
		}
		ctx = principal.WithVerified(ctx, verified)
		if info.FullMethod == spacev1.SpaceService_GetAuditLog_FullMethodName {
			// This migrated chain consumes typed verified identity exclusively.
			// Preserve transport/tracing metadata; never project raw actor headers.
			return handler(ctx, req)
		}
		accountType := "regular" // Existing lifecycle transport compatibility.
		if info.FullMethod == spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName {
			if verified.AccountType != "regular" && verified.AccountType != "guest" {
				return nil, deny
			}
			accountType = verified.AccountType
		}
		// Legacy handlers consume this projection only after cryptographic verification.
		// Caller-supplied identity metadata was rejected above; the projection is server-owned.
		identity := metadata.Pairs(authctx.HeaderUserID, verified.AccountID, authctx.HeaderProfileID, verified.ProfileID, authctx.HeaderSessionEpoch, strconv.FormatInt(verified.SessionEpoch, 10), "x-voice-account-type", accountType)
		ctx = metadata.NewIncomingContext(ctx, identity)
		return handler(ctx, req)
	}
}
