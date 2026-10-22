package lifecycleprincipal

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func moderationMethod(method string) bool {
	return method == spacev1.SpaceService_BanMember_FullMethodName || method == spacev1.SpaceService_UnbanMember_FullMethodName
}

// Public operation identity is part of the signed body. Transport request ID
// and JTI remain fresh for each dispatch, including an identical intent retry.
func (v Verifier) verifyModerationCall(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	deny := status.Error(codes.Unauthenticated, "invalid moderation principal")
	if v.Keys == nil || v.Replay == nil || v.Epoch == nil {
		return nil, deny
	}
	clock := v.Clock
	if clock == nil {
		clock = time.Now
	}
	started := clock()
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || !canonicalID(transport.RequestID) {
		return nil, deny
	}
	message, ok := req.(proto.Message)
	if !ok || !validAdmissionMessage(message, 0) {
		return nil, status.Error(codes.InvalidArgument, "invalid moderation request")
	}
	hash, err := principal.RequestHash(message)
	if err != nil {
		return nil, deny
	}
	// Bound the dependency phase even if a configured checker omits its own
	// timeout. The original observation clock survives all later lock waits.
	epoch := func(callCtx context.Context, account string, revision int64) error {
		bounded, cancel := context.WithTimeout(callCtx, 2*time.Second)
		defer cancel()
		err := v.Epoch(bounded, account, revision)
		if bounded.Err() != nil {
			return ErrDependencyUnavailable
		}
		return err
	}
	actor, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: info.FullMethod,
		ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
		KeyResolver: v.Keys, ReplayGuard: v.Replay, SessionEpochChecker: epoch, Clock: clock,
	})
	if errors.Is(err, ErrDependencyUnavailable) {
		return nil, status.Error(codes.Unavailable, "moderation authentication unavailable")
	}
	if err != nil {
		return nil, deny
	}
	proof := &admissionCallProof{actor: actor, method: info.FullMethod,
		message: message.ProtoReflect().Descriptor().FullName(), clock: clock, ownerStarted: started}
	verified := &VerifiedAdmissionCall{proof: *proof}
	if _, err = verified.ActorFor(message); err != nil {
		return nil, err
	}
	ctx = principal.WithVerified(ctx, actor)
	ctx = context.WithValue(ctx, admissionCallKey{}, proof)
	return handler(ctx, req)
}
