package botmembershipprincipal

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type GlobalTerminalBinding struct{ BotID, DecisionID uuid.UUID }
type globalTerminalKey struct{}

// VerifiedGlobalTerminal binds only Bot's accepted opaque global decision.
// It grants an all-Add fence, never source cleanup or Space/P3 authority.
type VerifiedGlobalTerminal struct {
	actor   principal.Principal
	binding GlobalTerminalBinding
	clock   func() time.Time
}

func globalTerminalBinding(r *spacev1.RegisterBotGlobalTerminalRequest) (GlobalTerminalBinding, error) {
	deny := status.Error(codes.InvalidArgument, "invalid minimal Bot terminal binding")
	if r == nil || r.ProtocolVersion != 1 || len(r.ProtoReflect().GetUnknown()) != 0 {
		return GlobalTerminalBinding{}, deny
	}
	bot, ok := canonicalID(r.BotId)
	if !ok {
		return GlobalTerminalBinding{}, deny
	}
	decision, ok := canonicalID(r.DecisionId)
	if !ok {
		return GlobalTerminalBinding{}, deny
	}
	return GlobalTerminalBinding{BotID: bot, DecisionID: decision}, nil
}

func (v Verifier) verifyGlobalTerminal(ctx context.Context, value any, handler grpc.UnaryHandler) (any, error) {
	r, ok := value.(*spacev1.RegisterBotGlobalTerminalRequest)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot global terminal registration request")
	}
	binding, err := globalTerminalBinding(r)
	if err != nil {
		return nil, err
	}
	deny := status.Error(codes.Unauthenticated, "invalid Bot global terminal registration principal")
	if v.Keys == nil || v.Replay == nil {
		return nil, status.Error(codes.Unavailable, "Bot global terminal registration verification unavailable")
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || transport.RequestID != binding.DecisionID.String() {
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
	actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, Clock: clock})
	if err != nil {
		var unavailable *dependencyUnavailable
		if errors.As(err, &unavailable) {
			return nil, status.Error(codes.Unavailable, "Bot global terminal registration verification unavailable")
		}
		return nil, deny
	}
	proof := &VerifiedGlobalTerminal{actor: actor, binding: binding, clock: clock}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	return handler(context.WithValue(principal.WithVerified(ctx, actor), globalTerminalKey{}, proof), value)
}

func (p *VerifiedGlobalTerminal) Check(r *spacev1.RegisterBotGlobalTerminalRequest) (GlobalTerminalBinding, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot global terminal registration required")
	if p == nil || p.clock == nil {
		return GlobalTerminalBinding{}, deny
	}
	b, err := globalTerminalBinding(r)
	if err != nil {
		return GlobalTerminalBinding{}, deny
	}
	hash, err := principal.RequestHash(r)
	a := p.actor
	if err != nil || b != p.binding || a.Kind != "service" || a.Issuer != "bot" || a.Subject != "service:bot" || a.AccountID != "" || a.ProfileID != "" || a.SessionEpoch != 0 || a.AccountType != "" || a.ActorProfileID != "" || a.BotID != "" || a.SpaceID != "" || a.InstallationID != "" || a.BotScope != "" || a.Audience != "space" || a.RPC != spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName || a.RequestID != b.DecisionID.String() || a.RequestHash != hash || !p.clock().Before(a.ExpiresAt) {
		return GlobalTerminalBinding{}, deny
	}
	return b, nil
}

func RequireGlobalTerminal(ctx context.Context, r *spacev1.RegisterBotGlobalTerminalRequest) (*VerifiedGlobalTerminal, error) {
	deny := status.Error(codes.Unauthenticated, "verified Bot global terminal registration required")
	if ctx == nil || ctx.Err() != nil {
		return nil, deny
	}
	p, ok := ctx.Value(globalTerminalKey{}).(*VerifiedGlobalTerminal)
	a, authenticated := principal.FromContext(ctx)
	if !ok || p == nil || !authenticated || a != p.actor {
		return nil, deny
	}
	if _, err := p.Check(r); err != nil {
		return nil, err
	}
	return p, nil
}
