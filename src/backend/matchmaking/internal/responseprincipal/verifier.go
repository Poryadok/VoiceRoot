// Package responseprincipal binds an authenticated Gateway actor to one exact
// public match decision. Generic authctx/metadata values cannot create it.
package responseprincipal

import (
	"context"
	"encoding/hex"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type Verifier struct {
	KeyResolver         principal.KeyResolver
	ReplayGuard         principal.ReplayGuard
	SessionEpochChecker principal.SessionEpochChecker
	Clock               func() time.Time
}
type Subject struct {
	AccountID, ProfileID, OperationID uuid.UUID
	SessionEpoch                      int64
	RequestHash                       [32]byte
}

type verifiedResponse struct {
	subject     Subject
	requestHash string
}
type responseKey struct{}

func (v Verifier) Verify(ctx context.Context, request *matchmakingv1.RespondToMatchRequest) (context.Context, error) {
	deny := func() (context.Context, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid match response principal")
	}
	if ctx == nil || request == nil || len(request.ProtoReflect().GetUnknown()) != 0 || v.KeyResolver == nil || v.ReplayGuard == nil || v.SessionEpochChecker == nil {
		return deny()
	}
	operation, err := canonicalID(request.OperationId)
	if err != nil {
		return deny()
	}
	if _, err = canonicalID(request.MatchId); err != nil {
		return deny()
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || transport.RequestID != operation.String() {
		return deny()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return deny()
	}
	verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "matchmaking", ExpectedRPC: matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName,
		ExpectedRequestID: operation.String(), ExpectedRequestHash: hash, KeyResolver: v.KeyResolver, ReplayGuard: v.ReplayGuard, SessionEpochChecker: v.SessionEpochChecker, Clock: v.Clock,
	})
	if err != nil || verified.RequestID != operation.String() || verified.SessionEpoch <= 0 {
		return deny()
	}
	account, err := canonicalID(verified.AccountID)
	if err != nil {
		return deny()
	}
	profile, err := canonicalID(verified.ProfileID)
	if err != nil {
		return deny()
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(verified.RequestHash, "sha256:"))
	if err != nil || len(raw) != 32 {
		return deny()
	}
	var digest [32]byte
	copy(digest[:], raw)
	return context.WithValue(ctx, responseKey{}, verifiedResponse{subject: Subject{AccountID: account, ProfileID: profile, OperationID: operation, SessionEpoch: verified.SessionEpoch, RequestHash: digest}, requestHash: hash}), nil
}

func canonicalID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, status.Error(codes.Unauthenticated, "invalid match response principal")
	}
	return id, nil
}

func SubjectFromContext(ctx context.Context, request *matchmakingv1.RespondToMatchRequest) (Subject, bool) {
	if ctx == nil || request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return Subject{}, false
	}
	verified, ok := ctx.Value(responseKey{}).(verifiedResponse)
	if !ok {
		return Subject{}, false
	}
	hash, err := principal.RequestHash(request)
	if err != nil || hash != verified.requestHash {
		return Subject{}, false
	}
	return verified.subject, true
}
