package botmembershipprincipal

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type PurgeHandoffBinding struct {
	RequestID, SpaceID, DeletionOperationID, ManifestID uuid.UUID
	Generation, ItemCount                               uint64
	ManifestSHA256                                      [32]byte
}
type purgeHandoffKey struct{}

// VerifiedPurgeHandoff authorizes only this finite read relay. It is neither
// an installation/global-removal capability nor a Space deletion decision.
type VerifiedPurgeHandoff struct {
	actor   principal.Principal
	binding PurgeHandoffBinding
	clock   func() time.Time
}

func purgeHandoffBinding(r *spacev1.GetBotPurgeHandoffRequest) (PurgeHandoffBinding, error) {
	deny := status.Error(codes.InvalidArgument, "invalid Bot purge handoff binding")
	if r == nil || r.ProtocolVersion != 1 || len(r.ProtoReflect().GetUnknown()) != 0 || r.Generation == 0 || r.Generation > math.MaxInt64 || r.Manifest == nil || len(r.Manifest.ProtoReflect().GetUnknown()) != 0 || len(r.Manifest.ManifestSha256) != 32 || r.Manifest.ItemCount > math.MaxInt64 {
		return PurgeHandoffBinding{}, deny
	}
	ids := []string{r.RequestId, r.SpaceId, r.DeletionOperationId, r.Manifest.ManifestId}
	var parsed [4]uuid.UUID
	for i, raw := range ids {
		id, ok := canonicalID(raw)
		if !ok {
			return PurgeHandoffBinding{}, deny
		}
		parsed[i] = id
	}
	b := PurgeHandoffBinding{RequestID: parsed[0], SpaceID: parsed[1], DeletionOperationID: parsed[2], ManifestID: parsed[3], Generation: r.Generation, ItemCount: r.Manifest.ItemCount}
	copy(b.ManifestSHA256[:], r.Manifest.ManifestSha256)
	return b, nil
}

func (v Verifier) verifyPurgeHandoff(ctx context.Context, value any, handler grpc.UnaryHandler) (any, error) {
	r, ok := value.(*spacev1.GetBotPurgeHandoffRequest)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot purge handoff request")
	}
	binding, err := purgeHandoffBinding(r)
	if err != nil {
		return nil, err
	}
	deny := status.Error(codes.Unauthenticated, "invalid Bot purge handoff principal")
	if v.Keys == nil || v.Replay == nil {
		return nil, deny
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || transport.RequestID != binding.RequestID.String() {
		return nil, deny
	}
	hash, err := principal.RequestHash(r)
	if err != nil {
		return nil, deny
	}
	clock := v.Clock
	if clock == nil {
		clock = time.Now
	}
	actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: clock})
	if err != nil {
		return nil, deny
	}
	proof := &VerifiedPurgeHandoff{actor: actor, binding: binding, clock: clock}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	return handler(context.WithValue(principal.WithVerified(ctx, actor), purgeHandoffKey{}, proof), value)
}

func (p *VerifiedPurgeHandoff) Check(r *spacev1.GetBotPurgeHandoffRequest) (PurgeHandoffBinding, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot purge handoff required")
	if p == nil || p.clock == nil {
		return PurgeHandoffBinding{}, deny
	}
	b, err := purgeHandoffBinding(r)
	if err != nil {
		return PurgeHandoffBinding{}, deny
	}
	hash, err := principal.RequestHash(r)
	a := p.actor
	if err != nil || b != p.binding || a.Kind != "service" || a.Issuer != "bot" || a.Subject != "service:bot" || a.AccountID != "" || a.ProfileID != "" || a.SessionEpoch != 0 || a.AccountType != "" || a.ActorProfileID != "" || a.BotID != "" || a.SpaceID != "" || a.InstallationID != "" || a.BotScope != "" || a.Audience != "space" || a.RPC != spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName || a.RequestID != b.RequestID.String() || a.RequestHash != hash || !p.clock().Before(a.ExpiresAt) {
		return PurgeHandoffBinding{}, deny
	}
	return b, nil
}

func RequirePurgeHandoff(ctx context.Context, r *spacev1.GetBotPurgeHandoffRequest) (*VerifiedPurgeHandoff, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot purge handoff required")
	if ctx == nil || ctx.Err() != nil {
		return nil, deny
	}
	p, ok := ctx.Value(purgeHandoffKey{}).(*VerifiedPurgeHandoff)
	a, authenticated := principal.FromContext(ctx)
	if !ok || p == nil || !authenticated || a != p.actor {
		return nil, deny
	}
	if _, err := p.Check(r); err != nil {
		return nil, err
	}
	return p, nil
}
