package botmembershipprincipal

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// GlobalRemovalBinding is an authenticated Bot assertion of its saved owner
// decision and exact native obligation. It is not a human Space permission,
// historical legacy-source ownership proof, or a native-method capability.
type GlobalRemovalBinding struct {
	OperationID, DecisionID           uuid.UUID
	DecisionSHA256, FrozenScopeSHA256 [32]byte
	Cancellation                      InstallationBinding
}

type globalRemovalKey struct{}

type VerifiedGlobalRemoval struct {
	actor   principal.Principal
	binding GlobalRemovalBinding
	clock   func() time.Time
}

func globalRemovalBinding(r *spacev1.ApplyBotInstallationRemovalRequest) (GlobalRemovalBinding, error) {
	deny := status.Error(codes.InvalidArgument, "invalid native global Bot removal")
	if r == nil || r.ProtocolVersion != 1 || r.Purpose != "global_delete" || len(r.DecisionSha256) != 32 || len(r.FrozenScopeSha256) != 32 || len(r.ProtoReflect().GetUnknown()) != 0 {
		return GlobalRemovalBinding{}, deny
	}
	operation, ok := canonicalID(r.OperationId)
	if !ok {
		return GlobalRemovalBinding{}, deny
	}
	decision, ok := canonicalID(r.DecisionId)
	if !ok {
		return GlobalRemovalBinding{}, deny
	}
	cancel, method, err := requestBinding(r.NativeCancellation)
	if err != nil || method != spacev1.SpaceService_RemoveBotMember_FullMethodName || !cancel.Remove || r.BotId != cancel.BotID.String() || r.ActorProfileId != cancel.ProfileID.String() || r.SpaceId != cancel.SpaceID.String() {
		return GlobalRemovalBinding{}, deny
	}
	b := GlobalRemovalBinding{OperationID: operation, DecisionID: decision, Cancellation: cancel}
	copy(b.DecisionSHA256[:], r.DecisionSha256)
	copy(b.FrozenScopeSHA256[:], r.FrozenScopeSha256)
	return b, nil
}

func (v Verifier) verifyGlobalRemoval(ctx context.Context, value any, handler grpc.UnaryHandler) (any, error) {
	r, ok := value.(*spacev1.ApplyBotInstallationRemovalRequest)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid native global Bot removal")
	}
	binding, err := globalRemovalBinding(r)
	if err != nil {
		return nil, err
	}
	deny := status.Error(codes.Unauthenticated, "invalid global Bot removal principal")
	if v.Keys == nil || v.Replay == nil {
		return nil, deny
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || transport.RequestID != binding.OperationID.String() {
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
	actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: clock})
	if err != nil {
		return nil, deny
	}
	proof := &VerifiedGlobalRemoval{actor: actor, binding: binding, clock: clock}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	return handler(context.WithValue(principal.WithVerified(ctx, actor), globalRemovalKey{}, proof), value)
}

func (p *VerifiedGlobalRemoval) Check(r *spacev1.ApplyBotInstallationRemovalRequest) (GlobalRemovalBinding, error) {
	deny := status.Error(codes.Unauthenticated, "verified global Bot removal required")
	if p == nil || p.clock == nil {
		return GlobalRemovalBinding{}, deny
	}
	binding, err := globalRemovalBinding(r)
	if err != nil {
		return GlobalRemovalBinding{}, deny
	}
	hash, err := principal.RequestHash(r)
	a := p.actor
	if err != nil || binding != p.binding || a.Kind != "service" || a.Issuer != "bot" || a.Subject != "service:bot" || a.AccountID != "" || a.ProfileID != "" || a.SessionEpoch != 0 || a.AccountType != "" || a.ActorProfileID != "" || a.BotID != "" || a.SpaceID != "" || a.InstallationID != "" || a.BotScope != "" || a.Audience != "space" || a.RPC != spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName || a.RequestID != binding.OperationID.String() || a.RequestHash != hash || !p.clock().Before(a.ExpiresAt) {
		return GlobalRemovalBinding{}, deny
	}
	return binding, nil
}

func RequireGlobalRemoval(ctx context.Context, r *spacev1.ApplyBotInstallationRemovalRequest) (*VerifiedGlobalRemoval, error) {
	deny := status.Error(codes.Unauthenticated, "verified global Bot removal required")
	if ctx == nil || ctx.Err() != nil {
		return nil, deny
	}
	p, ok := ctx.Value(globalRemovalKey{}).(*VerifiedGlobalRemoval)
	a, authenticated := principal.FromContext(ctx)
	if !ok || p == nil || !authenticated || a != p.actor {
		return nil, deny
	}
	if _, err := p.Check(r); err != nil {
		return nil, err
	}
	return p, nil
}
