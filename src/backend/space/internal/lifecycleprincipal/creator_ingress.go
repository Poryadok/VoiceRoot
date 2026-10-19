package lifecycleprincipal

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
)

// Creator consumes only this dedicated authority chain. Existing public
// lifecycle handlers retain their own verifier and context contract.
func (v Verifier) verifyCreatorCall(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if v.Keys == nil || v.CreatorReplay == nil || v.CreatorEpoch == nil {
		return nil, status.Error(codes.Unavailable, "creator authority unavailable")
	}
	deny := status.Error(codes.Unauthenticated, "invalid creator principal")
	req, ok := request.(*spacev1.CreateSpaceRequest)
	if !ok || req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid creator request")
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || !canonicalID(transport.RequestID) {
		return nil, deny
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, deny
	}
	verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.CreatorReplay, SessionEpochChecker: v.CreatorEpoch, Clock: v.Clock})
	if errors.Is(err, ErrDependencyUnavailable) || errors.Is(err, creatorauthority.ErrUnavailable) {
		return nil, status.Error(codes.Unavailable, "creator authentication unavailable")
	}
	if err != nil || !canonicalID(verified.AccountID) || !canonicalID(verified.ProfileID) || verified.AccountType != "regular" {
		return nil, deny
	}
	clock := v.Clock
	if clock == nil {
		clock = time.Now
	}
	ctx = principal.WithVerified(ctx, verified)
	ctx = context.WithValue(ctx, creatorProofKey{}, &creatorProof{actor: verified, epoch: v.CreatorEpoch, clock: clock})
	return handler(ctx, req)
}
