// Package intakeprincipal admits only Gateway's exact delegated CreateReport.
package intakeprincipal

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/pkg/principal"
)

var ErrDependencyUnavailable = errors.New("report authentication dependency unavailable")

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Epoch  principal.SessionEpochChecker
	Clock  func() time.Time
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func OrdinaryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == moderationv1.ModerationService_CreateReport_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "protected report intake listener required")
		}
		return handler(ctx, req)
	}
}

func (v Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != moderationv1.ModerationService_CreateReport_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on report intake listener")
		}
		deny := status.Error(codes.Unauthenticated, "invalid report principal")
		if v.Keys == nil || v.Replay == nil || v.Epoch == nil {
			return nil, status.Error(codes.Unavailable, "report authentication unavailable")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || !canonicalID(transport.RequestID) {
			return nil, deny
		}
		request, ok := req.(*moderationv1.CreateReportRequest)
		if !ok || request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid report request")
		}
		hash, err := principal.RequestHash(proto.Message(request))
		if err != nil {
			return nil, deny
		}
		verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "moderation", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, SessionEpochChecker: v.Epoch, Clock: v.Clock})
		if errors.Is(err, ErrDependencyUnavailable) {
			return nil, status.Error(codes.Unavailable, "report authentication unavailable")
		}
		if err != nil || !canonicalID(verified.AccountID) || !canonicalID(verified.ProfileID) || (verified.AccountType != "regular" && verified.AccountType != "guest" && verified.AccountType != "sdk-account") {
			return nil, deny
		}
		return handler(principal.WithVerified(ctx, verified), req)
	}
}
