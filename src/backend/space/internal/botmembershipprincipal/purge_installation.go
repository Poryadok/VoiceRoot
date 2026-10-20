package botmembershipprincipal

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type PurgeInstallationBinding struct {
	OperationID, SpaceID, DeletionOperationID, ManifestID uuid.UUID
	Generation, ItemCount                                 uint64
	ManifestSHA256                                        [32]byte
	Cancellation                                          InstallationBinding
}

type purgeInstallationKey struct{}

// VerifiedPurgeInstallation authenticates only the exact Bot request. Space's
// saved irreversible decision and source checks independently authorize effects.
// This is not a read, ordinary-installation, global-root or human capability.
type VerifiedPurgeInstallation struct {
	actor   principal.Principal
	binding PurgeInstallationBinding
	clock   func() time.Time
}

func purgeInstallationBinding(r *spacev1.ReconcileBotPurgeInstallationRequest) (PurgeInstallationBinding, error) {
	deny := status.Error(codes.InvalidArgument, "invalid native Bot P3 child binding")
	if r == nil || r.ProtocolVersion != 1 || len(r.ProtoReflect().GetUnknown()) != 0 || r.Generation == 0 || r.Generation > math.MaxInt64 || r.Manifest == nil || len(r.Manifest.ProtoReflect().GetUnknown()) != 0 || len(r.Manifest.ManifestSha256) != 32 || r.Manifest.ItemCount > math.MaxInt64 {
		return PurgeInstallationBinding{}, deny
	}
	ids := []string{r.OperationId, r.SpaceId, r.DeletionOperationId, r.Manifest.ManifestId}
	var parsed [4]uuid.UUID
	for i, raw := range ids {
		id, ok := canonicalID(raw)
		if !ok {
			return PurgeInstallationBinding{}, deny
		}
		parsed[i] = id
	}
	cancel, method, err := requestBinding(r.NativeCancellation)
	if err != nil || method != spacev1.SpaceService_RemoveBotMember_FullMethodName || !cancel.Remove || r.SpaceId != cancel.SpaceID.String() || r.BotId != cancel.BotID.String() || r.InstallationId != cancel.InstallationID.String() || parsed[0] == parsed[2] || parsed[0] == cancel.OperationID || parsed[0] == cancel.InstallOperationID {
		return PurgeInstallationBinding{}, deny
	}
	b := PurgeInstallationBinding{OperationID: parsed[0], SpaceID: parsed[1], DeletionOperationID: parsed[2], ManifestID: parsed[3], Generation: r.Generation, ItemCount: r.Manifest.ItemCount, Cancellation: cancel}
	copy(b.ManifestSHA256[:], r.Manifest.ManifestSha256)
	return b, nil
}

func (v Verifier) verifyPurgeInstallation(ctx context.Context, value any, handler grpc.UnaryHandler) (any, error) {
	r, ok := value.(*spacev1.ReconcileBotPurgeInstallationRequest)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot P3 child request")
	}
	b, err := purgeInstallationBinding(r)
	if err != nil {
		return nil, err
	}
	deny := status.Error(codes.Unauthenticated, "invalid Bot P3 child principal")
	if v.Keys == nil || v.Replay == nil {
		return nil, status.Error(codes.Unavailable, "Bot P3 child verification unavailable")
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || transport.RequestID != b.OperationID.String() {
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
	actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: clock})
	if err != nil {
		var unavailable *dependencyUnavailable
		if errors.As(err, &unavailable) {
			return nil, status.Error(codes.Unavailable, "Bot P3 child verification unavailable")
		}
		return nil, deny
	}
	proof := &VerifiedPurgeInstallation{actor: actor, binding: b, clock: clock}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	return handler(context.WithValue(principal.WithVerified(ctx, actor), purgeInstallationKey{}, proof), value)
}

func (p *VerifiedPurgeInstallation) Check(r *spacev1.ReconcileBotPurgeInstallationRequest) (PurgeInstallationBinding, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot P3 child required")
	if p == nil || p.clock == nil {
		return PurgeInstallationBinding{}, deny
	}
	b, err := purgeInstallationBinding(r)
	if err != nil {
		return PurgeInstallationBinding{}, deny
	}
	hash, err := principal.RequestHash(r)
	a := p.actor
	if err != nil || b != p.binding || a.Kind != "service" || a.Issuer != "bot" || a.Subject != "service:bot" || a.AccountID != "" || a.ProfileID != "" || a.SessionEpoch != 0 || a.AccountType != "" || a.ActorProfileID != "" || a.BotID != "" || a.SpaceID != "" || a.InstallationID != "" || a.BotScope != "" || a.Audience != "space" || a.RPC != spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName || a.RequestID != b.OperationID.String() || a.RequestHash != hash || !p.clock().Before(a.ExpiresAt) {
		return PurgeInstallationBinding{}, deny
	}
	return b, nil
}

func RequirePurgeInstallation(ctx context.Context, r *spacev1.ReconcileBotPurgeInstallationRequest) (*VerifiedPurgeInstallation, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot P3 child required")
	if ctx == nil || ctx.Err() != nil {
		return nil, deny
	}
	p, ok := ctx.Value(purgeInstallationKey{}).(*VerifiedPurgeInstallation)
	a, authenticated := principal.FromContext(ctx)
	if !ok || p == nil || !authenticated || a != p.actor {
		return nil, deny
	}
	if _, err := p.Check(r); err != nil {
		return nil, err
	}
	return p, nil
}
