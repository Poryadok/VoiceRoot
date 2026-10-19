package botpinprincipal

import (
	"bytes"
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

// VerifiedPurgeCoverage authorizes one exact read, never pin removal or purge.
type VerifiedPurgeCoverage struct {
	body  []byte
	proof principal.Principal
}
type coverageKey struct{}

func canonicalCoverageID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}
func validCoverageRequest(q *messagingv1.GetBotPurgeChatCoverageRequest) bool {
	if q == nil || q.ProtocolVersion != 1 || !canonicalCoverageID(q.RequestId) || !canonicalCoverageID(q.SpaceId) || !canonicalCoverageID(q.DeletionOperationId) || q.Generation == 0 || q.Generation > math.MaxInt64 || len(q.ProtoReflect().GetUnknown()) != 0 || q.Manifest == nil || len(q.Manifest.ProtoReflect().GetUnknown()) != 0 || !canonicalCoverageID(q.Manifest.ManifestId) || len(q.Manifest.ManifestSha256) != 32 || q.Manifest.ItemCount > math.MaxInt64 || len(q.ChatIds) > 1000 {
		return false
	}
	for i, id := range q.ChatIds {
		if !canonicalCoverageID(id) || i > 0 && q.ChatIds[i-1] >= id {
			return false
		}
	}
	return true
}

func (v VerifiedPurgeCoverage) Validate(q *messagingv1.GetBotPurgeChatCoverageRequest) error {
	if len(v.body) == 0 || !validCoverageRequest(q) {
		return ErrInvalidProof
	}
	p := v.proof
	if p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "messaging" || p.RPC != messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName || p.RequestID != q.RequestId || p.JWTID == "" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.ActorProfileID != "" || p.BotID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return ErrInvalidProof
	}
	now := time.Now().UTC()
	if p.IssuedAt.IsZero() || p.IssuedAt.After(now.Add(5*time.Second)) || !p.ExpiresAt.After(now) || !p.ExpiresAt.After(p.IssuedAt) || p.ExpiresAt.Sub(p.IssuedAt) > 30*time.Second {
		return ErrInvalidProof
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil || !bytes.Equal(raw, v.body) {
		return ErrInvalidProof
	}
	hash, err := principal.RequestHash(q)
	if err != nil || hash != p.RequestHash {
		return ErrInvalidProof
	}
	return nil
}

func (v VerifiedPurgeCoverage) Request() (*messagingv1.GetBotPurgeChatCoverageRequest, error) {
	q := new(messagingv1.GetBotPurgeChatCoverageRequest)
	if proto.Unmarshal(v.body, q) != nil {
		return nil, ErrInvalidProof
	}
	if err := v.Validate(q); err != nil {
		return nil, err
	}
	return q, nil
}

func RequireCoverage(ctx context.Context, q *messagingv1.GetBotPurgeChatCoverageRequest) (VerifiedPurgeCoverage, error) {
	v, ok := ctx.Value(coverageKey{}).(VerifiedPurgeCoverage)
	if !ok {
		return VerifiedPurgeCoverage{}, ErrInvalidProof
	}
	return v, v.Validate(q)
}

func coverageUnary(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler, verifier ServiceVerifier) (any, error) {
	q, ok := request.(*messagingv1.GetBotPurgeChatCoverageRequest)
	if !ok || !validCoverageRequest(q) {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot purge coverage request")
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || verifier == nil || transport.RequestID != q.RequestId {
		return nil, status.Error(codes.Unauthenticated, "invalid Bot purge coverage transport")
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot purge coverage body")
	}
	owned := new(messagingv1.GetBotPurgeChatCoverageRequest)
	if proto.Unmarshal(raw, owned) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot purge coverage body")
	}
	hash, err := principal.RequestHash(owned)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Bot purge coverage hash")
	}
	proof, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid Bot purge coverage principal")
	}
	v := VerifiedPurgeCoverage{body: raw, proof: proof}
	if v.Validate(owned) != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid Bot purge coverage binding")
	}
	return handler(context.WithValue(ctx, coverageKey{}, v), owned)
}
