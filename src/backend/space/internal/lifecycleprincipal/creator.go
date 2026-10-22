package lifecycleprincipal

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
)

type creatorProofKey struct{}
type creatorProof struct {
	actor principal.Principal
	epoch principal.SessionEpochChecker
	clock func() time.Time
}

// VerifiedCreator has no public constructor or writable identity fields. A raw
// principal/context projection cannot manufacture native membership provenance.
type VerifiedCreator struct{ proof creatorProof }

func (v *VerifiedCreator) ActorFor(request *spacev1.CreateSpaceRequest) (principal.Principal, error) {
	deny := status.Error(codes.Unauthenticated, "verified creator required")
	if v == nil || v.proof.clock == nil || request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return principal.Principal{}, deny
	}
	p := v.proof.actor
	hash, err := principal.RequestHash(request)
	if err != nil || p.Issuer != "gateway" || p.Audience != "space" || p.RPC != spacev1.SpaceService_CreateSpace_FullMethodName || p.RequestHash != hash || p.Subject != p.AccountID || !canonicalID(p.AccountID) || !canonicalID(p.ProfileID) || !canonicalID(p.RequestID) || p.SessionEpoch <= 0 || p.AccountType != "regular" || !v.proof.clock().Before(p.ExpiresAt) {
		return principal.Principal{}, deny
	}
	return p, nil
}

// RequireCreator performs the final fresh Auth observation outside DB locks.
// ActorFor repeats the body/expiry check locally after any later lock wait.
func RequireCreator(ctx context.Context, request *spacev1.CreateSpaceRequest) (*VerifiedCreator, error) {
	proof, ok := ctx.Value(creatorProofKey{}).(*creatorProof)
	if !ok || proof == nil || proof.epoch == nil || ctx.Err() != nil {
		return nil, status.Error(codes.Unauthenticated, "verified creator required")
	}
	current, ok := principal.FromContext(ctx)
	if !ok || current != proof.actor {
		return nil, status.Error(codes.Unauthenticated, "verified creator required")
	}
	verified := &VerifiedCreator{proof: *proof}
	p, err := verified.ActorFor(request)
	if err != nil {
		return nil, err
	}
	if err = proof.epoch(ctx, p.AccountID, p.SessionEpoch); err != nil {
		if errors.Is(err, ErrDependencyUnavailable) || errors.Is(err, creatorauthority.ErrUnavailable) {
			return nil, status.Error(codes.Unavailable, "creator authentication unavailable")
		}
		return nil, status.Error(codes.Unauthenticated, "creator session rejected")
	}
	if ctx.Err() != nil {
		return nil, status.Error(codes.Unavailable, "creator authentication unavailable")
	}
	if _, err = verified.ActorFor(request); err != nil {
		return nil, err
	}
	return verified, nil
}
